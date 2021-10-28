package dataprocessor

import (
	"github.com/jiuzhou-zhao/data-channel/inter"
)

func NewServerEncryptDataProcess(key []byte) inter.ServerDataProcessor {
	return &serverAesDataProcessorImpl{
		clientDataProcessor: NewClientEncryptDataProcess(key),
	}
}

type serverAesDataProcessorImpl struct {
	clientDataProcessor inter.ClientDataProcessor
}

func (impl *serverAesDataProcessorImpl) OnTerminate(addr string) {

}

func (impl *serverAesDataProcessorImpl) OnRead(dIn *inter.ServerData) (dOut *inter.ServerData, err error) {
	d, err := impl.clientDataProcessor.OnRead(dIn.Data)
	if err != nil || d == nil {
		return
	}
	dOut = &inter.ServerData{
		Addr: dIn.Addr,
		Data: d,
	}
	return
}

func (impl *serverAesDataProcessorImpl) OnWrite(dIn *inter.ServerData) (dOut *inter.ServerData, err error) {
	d, err := impl.clientDataProcessor.OnWrite(dIn.Data)
	if err != nil || d == nil {
		return
	}
	dOut = &inter.ServerData{
		Addr: dIn.Addr,
		Data: d,
	}
	return
}
