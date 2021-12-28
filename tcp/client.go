package tcp

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/l"
)

var ErrWriteBroken = errors.New("writeBroken")

const (
	clientBufferCount = 10
	frameSize         = 65507
)

func NewClient(ctx context.Context, address string, statusOb inter.ClientStatusOb, log l.Wrapper) (cli inter.Client, err error) {
	if statusOb == nil {
		statusOb = &inter.UnimplementedClientStatusOb{}
	}

	if log == nil {
		log = l.NewNopLoggerWrapper()
	}

	ctx, cancel := context.WithCancel(ctx)

	impl := &clientImpl{
		ctx:         ctx,
		ctxCancel:   cancel,
		address:     address,
		statusOb:    statusOb,
		log:         log.WithFields(l.StringField(l.ClsKey, "tcp-client")),
		connOpened:  false,
		conn:        nil,
		readCh:      make(chan []byte, clientBufferCount),
		writeCh:     make(chan []byte, clientBufferCount),
		connectedCh: make(chan interface{}, clientBufferCount),
		readErrCh:   make(chan error, clientBufferCount),
	}

	impl.wg.Add(1)

	go impl.procRoutine()

	cli = impl

	return
}

type clientImpl struct {
	wg sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	address string

	statusOb inter.ClientStatusOb
	log      l.Wrapper

	readCh      chan []byte
	writeCh     chan []byte
	connectedCh chan interface{}
	readErrCh   chan error

	connOpened bool
	conn       *net.TCPConn
}

func (impl *clientImpl) Context() context.Context {
	return impl.ctx
}

func (impl *clientImpl) SetOb(ob inter.ClientStatusOb) {
	if ob == nil {
		ob = &inter.UnimplementedClientStatusOb{}
	}

	impl.statusOb = ob
}

func (impl *clientImpl) GetOb() inter.ClientStatusOb {
	return impl.statusOb
}

func (impl *clientImpl) ReadCh() chan []byte {
	return impl.readCh
}

func (impl *clientImpl) WriteCh() chan []byte {
	return impl.writeCh
}

func (impl *clientImpl) Wait() {
	impl.wg.Wait()
}

func (impl *clientImpl) CloseAndWait() {
	impl.ctxCancel()
	impl.readErrCh <- nil
	impl.wg.Wait()
}

func (impl *clientImpl) dial() (conn *net.TCPConn, err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", impl.address)
	if err != nil {
		return
	}

	return net.DialTCP("tcp", nil, tcpAddr)
}

func (impl *clientImpl) procRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(l.StringField(l.ClsModuleKey, "main_routine"))
	log.Info("enter")

	defer log.Info("leave")

	impl.wg.Add(1)

	go impl.readRoutine()

	loop := true

	fnCloseConn := func(err error) {
		if err != nil {
			impl.statusOb.OnException(err)
		}

		if impl.conn != nil {
			_ = impl.conn.Close()
			impl.conn = nil
			impl.connOpened = false
			impl.statusOb.OnClose()
		}
	}

	dialInterval := time.Millisecond

	for loop {
		if !impl.connOpened {
			select {
			case <-impl.ctx.Done():
				loop = false

				log.Info("try exit")

				continue
			case <-time.After(dialInterval):
			}

			conn, err := impl.dial()
			if err != nil {
				impl.statusOb.OnException(err)

				dialInterval *= 2

				if dialInterval >= time.Minute {
					dialInterval = time.Minute
				}

				log.Infof("dial error: %v", err)

				continue
			}

			log.Info("dial success")

			impl.conn = conn
			impl.connOpened = true
			dialInterval = time.Millisecond
			impl.connectedCh <- true
			impl.statusOb.OnConnect()
		}

		select {
		case <-impl.ctx.Done():
			loop = false

			log.Info("try exit")

			continue
		case e := <-impl.readErrCh:
			log.Info("check read error, close listener")
			fnCloseConn(e)

			continue
		case d := <-impl.writeCh:
			n, e := impl.conn.Write(d)
			if e != nil {
				fnCloseConn(e)
				log.Info("write error, close listener")

				continue
			}

			if n != len(d) {
				log.Errorf("write failed: %v, %v-%v", e, n, len(d))
				impl.statusOb.OnException(ErrWriteBroken)

				continue
			}
		}
	}
}

func (impl *clientImpl) readRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(l.StringField(l.ClsModuleKey, "read_routine"))
	log.Info("enter")

	defer log.Info("leave")

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			log.Info("try exit")

			continue
		case <-impl.connectedCh:
			log.Info("check connect")
		}

		for loop {
			select {
			case <-impl.ctx.Done():
				loop = false

				log.Info("try exit")

				continue
			default:
			}

			buf := make([]byte, frameSize)
			n, e := impl.conn.Read(buf)

			if e != nil {
				log.Errorf("read failed: %v", e)
				impl.readErrCh <- e
				impl.statusOb.OnException(e)

				break
			}

			impl.readCh <- buf[:n]
		}
	}
}
