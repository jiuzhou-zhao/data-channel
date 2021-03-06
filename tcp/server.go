package tcp

import (
	"context"
	"net"
	"sync"

	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/l"
)

const (
	serverBufferCount = 1000
)

func NewServer(ctx context.Context, address string, statusOb inter.ServerStatusOb, log l.Wrapper) (svr inter.Server, err error) {
	return NewServerEx(ctx, address, statusOb, log, serverBufferCount)
}

func NewServerEx(ctx context.Context, address string, statusOb inter.ServerStatusOb, log l.Wrapper, channelBufferSize int) (svr inter.Server, err error) {
	if statusOb == nil {
		statusOb = &inter.UnimplementedServerStatusOb{}
	}

	if log == nil {
		log = l.NewNopLoggerWrapper()
	}

	if channelBufferSize <= 0 {
		channelBufferSize = serverBufferCount
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return
	}

	conn, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(ctx)

	impl := &serverImpl{
		ctx:       ctx,
		ctxCancel: cancel,
		statusOb:  statusOb,
		log:       log.WithFields(l.StringField(l.ClsKey, "tcp-server")),
		listener:  conn,
		readCh:    make(chan *inter.ServerData, channelBufferSize),
		writeCh:   make(chan *inter.ServerData, channelBufferSize),
		acceptCh:  make(chan net.Conn, channelBufferSize),
		closeCh:   make(chan net.Conn, channelBufferSize),
		cliMap:    make(map[string]net.Conn),
	}

	impl.wg.Add(1)

	go impl.procRoutine()

	svr = impl

	return
}

type serverImpl struct {
	wg sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	statusOb inter.ServerStatusOb
	log      l.Wrapper

	listener *net.TCPListener

	readCh  chan *inter.ServerData
	writeCh chan *inter.ServerData

	acceptCh chan net.Conn
	closeCh  chan net.Conn
	cliMap   map[string]net.Conn
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
	impl.ctxCancel()
	_ = impl.listener.Close()
	impl.wg.Wait()
}

func (impl *serverImpl) procRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(l.StringField(l.ClsModuleKey, "main_routine"))
	log.Info("enter")

	defer log.Info("leave")

	impl.wg.Add(1)

	go impl.acceptRoutine()

	defer func() {
		for _, conn := range impl.cliMap {
			_ = conn.Close()
		}
	}()

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			log.Info("try exit")

			continue
		case cli := <-impl.acceptCh:
			log.Infof("accept client: %s", cli.RemoteAddr().String())
			impl.cliMap[cli.RemoteAddr().String()] = cli

			impl.wg.Add(1)

			go impl.readRoutine(cli)
		case cli := <-impl.closeCh:
			log.Infof("close client: %s", cli.RemoteAddr().String())
			delete(impl.cliMap, cli.RemoteAddr().String())
		case d := <-impl.writeCh:
			log.Debugf("write %d to %s", len(d.Data), d.Addr)

			if cli, ok := impl.cliMap[d.Addr]; ok {
				n, err := cli.Write(d.Data)
				if err != nil {
					impl.statusOb.OnException(d.Addr, err)
					log.Errorf("cli[%s] write failed: %v", d.Addr, err)

					_ = cli.Close()
				}

				if n != len(d.Data) {
					log.Errorf("cli[%s] write: %n/%d", d.Addr, n, len(d.Data))
				}
			} else {
				log.Errorf("try write, no client: %s", d.Addr)
			}
		}
	}
}

func (impl *serverImpl) acceptRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(l.StringField(l.ClsModuleKey, "accept_routine"))
	log.Info("enter")

	defer log.Info("leave")

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			log.Info("try exit")

			continue
		default:
		}

		cli, err := impl.listener.Accept()
		if err != nil {
			impl.statusOb.OnException("", err)

			continue
		}

		_ = cli.(*net.TCPConn).SetNoDelay(false)

		impl.acceptCh <- cli
	}
}

func (impl *serverImpl) readRoutine(conn net.Conn) {
	defer impl.wg.Done()

	log := impl.log.WithFields(l.StringField(l.ClsModuleKey, "read_routine"),
		l.FieldString("addr", conn.RemoteAddr().String()))
	log.Info("enter")

	defer log.Info("leave")

	addr := conn.RemoteAddr().String()

	impl.statusOb.OnConnect(addr)
	defer impl.statusOb.OnClose(addr)

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			log.Info("try exit")

			continue
		default:
		}

		buf := make([]byte, frameSize)
		n, err := conn.Read(buf)

		if err != nil {
			impl.statusOb.OnException(addr, err)

			break
		}
		impl.readCh <- &inter.ServerData{
			Addr: addr,
			Data: buf[:n],
		}
	}

	impl.closeCh <- conn
}
