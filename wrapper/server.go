package wrapper

import (
	"context"
	"sync"

	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/l"
)

const DefaultChannelBufferSize = 1000

func NewServer(server inter.Server, log l.Wrapper, processors ...inter.ServerDataProcessor) inter.Server {
	return NewServerEx(server, DefaultChannelBufferSize, log, processors...)
}

func NewServerEx(server inter.Server, channelBufferSize int, log l.Wrapper, processors ...inter.ServerDataProcessor) inter.Server {
	ob := server.GetOb()

	if channelBufferSize <= 0 {
		channelBufferSize = DefaultChannelBufferSize
	}

	if log == nil {
		log = l.NewNopLoggerWrapper()
	}

	impl := &serverImpl{
		server:     server,
		ob:         ob,
		log:        log.WithFields(l.StringField(l.ClsModuleKey, "server-wrapper")),
		processors: processors,
		readCh:     make(chan *inter.ServerData, channelBufferSize),
		writeCh:    make(chan *inter.ServerData, channelBufferSize),
		closeCh:    make(chan string, channelBufferSize),
	}

	server.SetOb(impl)

	impl.wg.Add(1)

	go impl.writeCloseRoutine()

	impl.wg.Add(1)

	go impl.readRoutine()

	return impl
}

type serverImpl struct {
	wg sync.WaitGroup

	server     inter.Server
	ob         inter.ServerStatusOb
	processors []inter.ServerDataProcessor
	log        l.Wrapper

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

func (impl *serverImpl) readRoutine() {
	defer impl.wg.Done()

	loop := true
	for loop {
		select {
		case <-impl.server.Context().Done():
			loop = false

			continue
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
		}
	}
}

func (impl *serverImpl) writeCloseRoutine() {
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
