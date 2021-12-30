package grpc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/jiuzhou-zhao/data-channel/grpc/channelpb"
	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/l"
	"google.golang.org/grpc"
)

const (
	serverBufferCount = 1000
)

func NewServer(ctx context.Context, address string, statusOb inter.ServerStatusOb, logger l.Wrapper) (inter.Server, error) {
	return NewServerEx(ctx, address, statusOb, logger, serverBufferCount)
}

func NewServerEx(ctx context.Context, address string, statusOb inter.ServerStatusOb, logger l.Wrapper, channelBufferSize int) (inter.Server, error) {
	if statusOb == nil {
		statusOb = &inter.UnimplementedServerStatusOb{}
	}

	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	if channelBufferSize <= 0 {
		channelBufferSize = serverBufferCount
	}

	listen, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	server := &serverImpl{
		ctx:             ctx,
		statusOb:        statusOb,
		logger:          logger.WithFields(l.StringField(l.ClsKey, "grpc-server")),
		readCh:          make(chan *inter.ServerData, channelBufferSize),
		writeCh:         make(chan *inter.ServerData, channelBufferSize),
		cliConnectCh:    make(chan *cliConn, channelBufferSize),
		cliDisconnectCh: make(chan *cliConn, channelBufferSize),
	}

	server.start(listen)

	return server, nil
}

type cliConn struct {
	addr   string
	stream channelpb.Channel_DataServer
	cancel context.CancelFunc
}

type serverImpl struct {
	wg       sync.WaitGroup
	ctx      context.Context
	server   *grpc.Server
	statusOb inter.ServerStatusOb
	logger   l.Wrapper

	readCh  chan *inter.ServerData
	writeCh chan *inter.ServerData

	cliConnectCh    chan *cliConn
	cliDisconnectCh chan *cliConn
}

func (impl *serverImpl) start(listener net.Listener) {
	impl.server = grpc.NewServer()

	impl.wg.Add(1)

	go func() {
		defer impl.wg.Done()

		channelpb.RegisterChannelServer(impl.server, impl)
		err := impl.server.Serve(listener)

		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("grpcServe")
		}
	}()

	impl.wg.Add(1)

	go impl.writeRoutine()
}

func (impl *serverImpl) writeRoutine() {
	defer impl.wg.Done()

	logger := impl.logger.WithFields(l.StringField(l.ClsModuleKey, "write-routine"))
	logger.Info("enter")

	defer logger.Info("leave")

	writers := make(map[string]*cliConn)
	loop := true

	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			continue
		case cliC := <-impl.cliConnectCh:
			impl.statusOb.OnConnect(cliC.addr)
			writers[cliC.addr] = cliC
		case cliC := <-impl.cliDisconnectCh:
			impl.statusOb.OnClose(cliC.addr)
			delete(writers, cliC.addr)
		case d := <-impl.writeCh:
			if cliC, ok := writers[d.Addr]; ok {
				err := cliC.stream.Send(&channelpb.D{
					Datas: d.Data,
				})
				if err != nil {
					cliC.cancel()
				}
			}
		}
	}
}

func (impl *serverImpl) Data(stream channelpb.Channel_DataServer) error {
	addr := uuid.New().String()

	ctx, cancel := context.WithCancel(impl.ctx)
	defer cancel()

	logger := impl.logger.WithFields(l.StringField(l.ClsModuleKey, "data-"+addr))
	logger.Info("enter")

	defer logger.Info("leave")

	impl.cliConnectCh <- &cliConn{
		addr:   addr,
		stream: stream,
		cancel: cancel,
	}

	defer func() {
		impl.cliDisconnectCh <- &cliConn{
			addr:   addr,
			stream: stream,
		}
	}()

	go func() {
		defer cancel()

		for {
			in, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				logger.Info("sendClose")

				break
			}

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Info("sendFailed")

				break
			}

			impl.readCh <- &inter.ServerData{
				Addr: addr,
				Data: in.Datas,
			}
		}
	}()

	<-ctx.Done()

	return nil
}

func (impl *serverImpl) Context() context.Context {
	return impl.ctx
}

func (impl *serverImpl) SetOb(ob inter.ServerStatusOb) {
	if ob == nil {
		ob = &inter.UnimplementedServerStatusOb{}
	}

	impl.statusOb = ob
}

func (impl *serverImpl) GetOb() inter.ServerStatusOb {
	return impl.statusOb
}

func (impl *serverImpl) ReadCh() chan *inter.ServerData {
	return impl.readCh
}

func (impl *serverImpl) WriteCh() chan *inter.ServerData {
	return impl.writeCh
}

func (impl *serverImpl) Wait() {
	impl.wg.Wait()
}

func (impl *serverImpl) CloseAndWait() {
	impl.server.Stop()
	impl.wg.Wait()
}
