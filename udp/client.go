package udp

import (
	"context"
	"net"
	"sync"

	"errors"
	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/logger"
)

const (
	clientBufferCount = 10
	frameSize         = 65507
)

var (
	ErrWriteBroken = errors.New("writeBroken")
)

func NewClient(ctx context.Context, address string, statusOb inter.ClientStatusOb, log logger.Wrapper) (inter.Client, error) {
	if statusOb == nil {
		statusOb = &inter.UnimplementedClientStatusOb{}
	}

	if log == nil {
		log = logger.NewWrapper(&logger.NopLogger{}).WithFields(logger.FieldString("role", "udp_client"))
	}

	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	impl := &clientImpl{
		ctx:       ctx,
		ctxCancel: cancel,
		statusOb:  statusOb,
		log:       log,
		conn:      conn,
		readCh:    make(chan []byte, clientBufferCount),
		writeCh:   make(chan []byte, clientBufferCount),
	}

	impl.wg.Add(1)
	go impl.procRoutine()

	return impl, nil
}

type clientImpl struct {
	wg sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	statusOb inter.ClientStatusOb
	log      logger.Wrapper

	conn net.Conn

	readCh  chan []byte
	writeCh chan []byte
}

func (impl *clientImpl) Wait() {
	impl.wg.Wait()
}

func (impl *clientImpl) Context() context.Context {
	return impl.ctx
}

func (impl *clientImpl) GetOb() inter.ClientStatusOb {
	return impl.statusOb
}

func (impl *clientImpl) SetOb(ob inter.ClientStatusOb) {
	if ob == nil {
		ob = &inter.UnimplementedClientStatusOb{}
	}
	impl.statusOb = ob
}

func (impl *clientImpl) ReadCh() chan []byte {
	return impl.readCh
}

func (impl *clientImpl) WriteCh() chan []byte {
	return impl.writeCh
}

func (impl *clientImpl) CloseAndWait() {
	impl.ctxCancel()
	_ = impl.conn.Close()
	impl.wg.Wait()
}

func (impl *clientImpl) procRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(logger.FieldString("role", "udp_client_proc"))

	impl.statusOb.OnConnect()

	impl.wg.Add(1)
	go impl.readRoutine()

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			continue
		case d := <-impl.writeCh:
			n, e := impl.conn.Write(d)
			if e != nil {
				impl.statusOb.OnException(e)
				continue
			}

			if n != len(d) {
				log.Errorf("write failed: %v, %v-%v", e, n, len(d))
				impl.statusOb.OnException(ErrWriteBroken)

				continue
			}
		}
	}

	impl.statusOb.OnClose()
}

func (impl *clientImpl) readRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(logger.FieldString("role", "udp_client_proc_read"))

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			continue
		default:
		}

		buf := make([]byte, frameSize)
		n, e := impl.conn.Read(buf)

		if e != nil {
			log.Errorf("read failed: %v", e)
			impl.statusOb.OnException(e)

			continue
		}

		impl.readCh <- buf[:n]
	}
}
