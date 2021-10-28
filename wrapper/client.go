package wrapper

import (
	"context"
	"sync"

	"github.com/jiuzhou-zhao/data-channel/inter"
)

func NewClient(client inter.Client, processors ...inter.ClientDataProcessor) inter.Client {
	impl := &clientImpl{
		client:     client,
		processors: processors,
		readCh:     make(chan []byte, 10),
		writeCh:    make(chan []byte, 10),
	}

	impl.wg.Add(1)
	go impl.procRoutine()

	return impl
}

type clientImpl struct {
	wg sync.WaitGroup

	client     inter.Client
	processors []inter.ClientDataProcessor

	readCh  chan []byte
	writeCh chan []byte
}

func (impl *clientImpl) Context() context.Context {
	return impl.client.Context()
}

func (impl *clientImpl) GetOb() inter.ClientStatusOb {
	return impl.client.GetOb()
}

func (impl *clientImpl) SetOb(ob inter.ClientStatusOb) {
	impl.client.SetOb(ob)
}

func (impl *clientImpl) ReadCh() chan []byte {
	return impl.readCh
}

func (impl *clientImpl) WriteCh() chan []byte {
	return impl.writeCh
}

func (impl *clientImpl) CloseAndWait() {
	impl.client.CloseAndWait()
	impl.wg.Wait()
}

func (impl *clientImpl) processReadData(dIn []byte) (dOut []byte, err error) {
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

func (impl *clientImpl) processWriteData(dIn []byte) (dOut []byte, err error) {
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

func (impl *clientImpl) procRoutine() {
	defer impl.wg.Done()

	loop := true
	for loop {
		select {
		case <-impl.client.Context().Done():
			loop = false

			continue
		case d := <-impl.client.ReadCh():
			d, err := impl.processReadData(d)
			if err != nil {
				impl.client.GetOb().OnException(err)
				continue
			}
			if d != nil {
				impl.readCh <- d
			}
		case d := <-impl.writeCh:
			d, err := impl.processWriteData(d)
			if err != nil {
				impl.client.GetOb().OnException(err)
				continue
			}

			if d != nil {
				impl.client.WriteCh() <- d
			}
		}
	}
}
