package grpc

import (
	"context"
	"sync"

	"github.com/jiuzhou-zhao/data-channel/grpc/channelpb"
	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/l"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	clientBufferCount = 1000
)

func NewClient(ctx context.Context, address string, statusOb inter.ClientStatusOb, logger l.Wrapper) (inter.Client, error) {
	if statusOb == nil {
		statusOb = &inter.UnimplementedClientStatusOb{}
	}

	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	ctx, cancel := context.WithCancel(ctx)

	cli := &cliImpl{
		ctx:              ctx,
		ctxCancel:        cancel,
		statusOb:         statusOb,
		logger:           logger.WithFields(l.StringField(l.ClsKey, "grpc-client")),
		readCh:           make(chan []byte, clientBufferCount),
		writeCh:          make(chan []byte, clientBufferCount),
		streamConnect:    make(chan channelpb.Channel_DataClient, clientBufferCount),
		streamDisconnect: make(chan channelpb.Channel_DataClient, clientBufferCount),
	}

	if err := cli.init(address); err != nil {
		return nil, err
	}

	return cli, nil
}

type cliImpl struct {
	wg sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
	statusOb  inter.ClientStatusOb
	logger    l.Wrapper

	readCh           chan []byte
	writeCh          chan []byte
	streamConnect    chan channelpb.Channel_DataClient
	streamDisconnect chan channelpb.Channel_DataClient

	conn *grpc.ClientConn
}

func (impl *cliImpl) init(address string) (err error) {
	impl.conn, err = grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}

	impl.wg.Add(1)

	go impl.mainRoutine()

	impl.wg.Add(1)

	go impl.connectRoutine()

	return
}

func (impl *cliImpl) mainRoutine() {
	defer func() {
		impl.wg.Done()
		_ = impl.conn.Close()
	}()

	loop := true

	var stream channelpb.Channel_DataClient

	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			continue
		case s := <-impl.streamConnect:
			stream = s
		case s := <-impl.streamDisconnect:
			if stream == s {
				stream = nil
			}
		case d := <-impl.writeCh:
			if stream != nil {
				err := stream.Send(&channelpb.D{
					Datas: d,
				})
				if err != nil {
					_ = stream.CloseSend()
				}
			}
		}
	}
}

func (impl *cliImpl) connectRoutine() {
	defer impl.wg.Done()

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			continue
		default:
		}

		grpcCli := channelpb.NewChannelClient(impl.conn)

		stream, err := grpcCli.Data(impl.ctx)
		if err != nil {
			continue
		}

		impl.streamConnect <- stream

		for {
			d, errI := stream.Recv()
			if errI != nil {
				break
			}
			impl.readCh <- d.Datas
		}

		impl.streamDisconnect <- stream
	}
}

func (impl *cliImpl) Context() context.Context {
	return impl.ctx
}

func (impl *cliImpl) SetOb(ob inter.ClientStatusOb) {
	if ob == nil {
		ob = &inter.UnimplementedClientStatusOb{}
	}

	impl.statusOb = ob
}

func (impl *cliImpl) GetOb() inter.ClientStatusOb {
	return impl.statusOb
}

func (impl *cliImpl) ReadCh() chan []byte {
	return impl.readCh
}

func (impl *cliImpl) WriteCh() chan []byte {
	return impl.writeCh
}

func (impl *cliImpl) Wait() {
	impl.wg.Wait()
}

func (impl *cliImpl) CloseAndWait() {
	impl.ctxCancel()

	if conn := impl.conn; conn != nil {
		_ = conn.Close()
	}

	impl.wg.Wait()
}
