package wrapper

import (
	"context"
	"github.com/sgostarter/i/logger"
	"sync"

	"github.com/jiuzhou-zhao/data-channel/inter"
)

func NewServer(server inter.Server, log logger.Wrapper, processors ...inter.ServerDataProcessor) inter.Server {
	ob := server.GetOb()

	if log == nil {
		log = logger.NewWrapper(&logger.NopLogger{}).WithFields(logger.FieldString("role", "udp_server"))
	}

	impl := &serverImpl{
		server:     server,
		ob:         ob,
		log:        log,
		processors: processors,
		readCh:     make(chan *inter.ServerData, 10),
		writeCh:    make(chan *inter.ServerData, 10),
		closeCh:    make(chan string, 10),
	}

	server.SetOb(impl)

	impl.wg.Add(1)
	go impl.procRoutine()

	return impl
}

type serverImpl struct {
	wg sync.WaitGroup

	server     inter.Server
	ob         inter.ServerStatusOb
	processors []inter.ServerDataProcessor
	log        logger.Wrapper

	readCh  chan *inter.ServerData
	writeCh chan *inter.ServerData
	closeCh chan string
}

func (impl *serverImpl) Context() context.Context {
	return impl.server.Context()
}

func (impl *serverImpl) SetOb(ob inter.ServerStatusOb) {
	if ob == nil {
		ob = &inter.UnimplementedServerStatusOb{}
	}
	impl.ob = ob
}

func (impl *serverImpl) GetOb() inter.ServerStatusOb {
	return impl.ob
}

func (impl *serverImpl) ReadCh() chan *inter.ServerData {
	return impl.readCh
}

func (impl *serverImpl) WriteCh() chan *inter.ServerData {
	return impl.writeCh
}

func (impl *serverImpl) Wait() {
	impl.wg.Wait()
	impl.server.CloseAndWait()
}

func (impl *serverImpl) CloseAndWait() {
	impl.server.CloseAndWait()
	impl.wg.Wait()
}

func (impl *serverImpl) OnConnect(addr string) {
	impl.ob.OnConnect(addr)
}

func (impl *serverImpl) OnClose(addr string) {
	impl.closeCh <- addr
	impl.ob.OnClose(addr)
}

func (impl *serverImpl) OnException(addr string, err error) {
	impl.ob.OnException(addr, err)
}

func (impl *serverImpl) processReadData(dIn *inter.ServerData) (dOut *inter.ServerData, err error) {
	impl.log.Infof("read %d from %s", len(dIn.Data), dIn.Addr)

	dOut = dIn
	for idx := len(impl.processors) - 1; idx >= 0; idx-- {
		dOut, err = impl.processors[idx].OnRead(dOut)
		if err != nil {
			break
		}
		if dOut == nil {
			break
		}
	}

	return
}

func (impl *serverImpl) processWriteData(dIn *inter.ServerData) (dOut *inter.ServerData, err error) {
	impl.log.Infof("write %d from %s", len(dIn.Data), dIn.Addr)

	dOut = dIn
	for _, processor := range impl.processors {
		dOut, err = processor.OnWrite(dOut)
		if err != nil {
			break
		}
		if dOut == nil {
			break
		}
	}

	return
}

func (impl *serverImpl) procRoutine() {
	defer impl.wg.Done()

	loop := true
	for loop {
		select {
		case <-impl.server.Context().Done():
			loop = false

			continue
		case addr := <-impl.closeCh:
			for _, processor := range impl.processors {
				processor.OnTerminate(addr)
			}
		case d := <-impl.server.ReadCh():
			addr := d.Addr
			d, err := impl.processReadData(d)
			if err != nil {
				impl.server.GetOb().OnException(addr, err)
				continue
			}
			if d != nil {
				impl.readCh <- d
			}
		case d := <-impl.writeCh:
			addr := d.Addr
			d, err := impl.processWriteData(d)
			if err != nil {
				impl.server.GetOb().OnException(addr, err)
				continue
			}
			if d != nil {
				impl.server.WriteCh() <- d
			}
		}
	}
}
