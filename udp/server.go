package udp

import (
	"context"
	"net"
	"sync"

	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/l"
)

const (
	serverBufferCount = 10
)

func NewServer(ctx context.Context, address string, statusOb inter.ServerStatusOb, log l.Wrapper) (inter.Server, error) {
	if statusOb == nil {
		statusOb = &inter.UnimplementedServerStatusOb{}
	}

	if log == nil {
		log = l.NewNopLoggerWrapper()
	}

	lAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", lAddr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	impl := &serverImpl{
		ctx:            ctx,
		ctxCancel:      cancel,
		statusOb:       statusOb,
		log:            log.WithFields(l.StringField(l.ClsKey, "udp-server")),
		conn:           conn,
		readCh:         make(chan *inter.ServerData, serverBufferCount),
		writeCh:        make(chan *inter.ServerData, serverBufferCount),
		readChInternal: make(chan *dataInternal, serverBufferCount),
		cliMap:         make(map[string]net.UDPAddr),
	}

	impl.wg.Add(1)

	go impl.procRoutine()

	return impl, nil
}

type dataInternal struct {
	d []byte
	a *net.UDPAddr
}

type serverImpl struct {
	wg sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	statusOb inter.ServerStatusOb
	log      l.Wrapper

	conn *net.UDPConn

	readCh  chan *inter.ServerData
	writeCh chan *inter.ServerData

	readChInternal chan *dataInternal

	cliMap map[string]net.UDPAddr
}

func (impl *serverImpl) Wait() {
	impl.wg.Wait()
}

func (impl *serverImpl) Context() context.Context {
	return impl.ctx
}

func (impl *serverImpl) GetOb() inter.ServerStatusOb {
	return impl.statusOb
}

func (impl *serverImpl) SetOb(ob inter.ServerStatusOb) {
	if ob == nil {
		ob = &inter.UnimplementedServerStatusOb{}
	}

	impl.statusOb = ob
}

func (impl *serverImpl) ReadCh() chan *inter.ServerData {
	return impl.readCh
}

func (impl *serverImpl) WriteCh() chan *inter.ServerData {
	return impl.writeCh
}

func (impl *serverImpl) CloseAndWait() {

}

func (impl *serverImpl) procRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(l.StringField(l.ClsModuleKey, "main_routine"))

	impl.wg.Add(1)

	go impl.readRoutine()

	loop := true

	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			continue
		case d := <-impl.writeCh:
			if cliAddr, ok := impl.cliMap[d.Addr]; ok {
				_, err := impl.conn.WriteToUDP(d.Data, &cliAddr)
				if err != nil {
					log.Errorf("udp server write failed: %v", err)
				}
			} else {
				log.Errorf("udp server no address: %s", d.Addr)
			}

		case di := <-impl.readChInternal:
			if _, ok := impl.cliMap[di.a.String()]; !ok {
				impl.cliMap[di.a.String()] = *di.a
				log.Infof("add client %s", di.a.String())
				impl.statusOb.OnConnect(di.a.String())
			}

			impl.readCh <- &inter.ServerData{
				Addr: di.a.String(),
				Data: di.d,
			}
		}
	}
}

func (impl *serverImpl) readRoutine() {
	defer impl.wg.Done()

	log := impl.log.WithFields(l.StringField(l.ClsModuleKey, "read_routine"))

	loop := true
	for loop {
		select {
		case <-impl.ctx.Done():
			loop = false

			continue
		default:
		}

		buf := make([]byte, frameSize)
		n, addr, e := impl.conn.ReadFromUDP(buf)

		if e != nil {
			log.Errorf("read failed: %v", e)

			continue
		}

		impl.readChInternal <- &dataInternal{
			d: buf[:n],
			a: addr,
		}
	}
}
