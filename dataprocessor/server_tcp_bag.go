package dataprocessor

import "github.com/jiuzhou-zhao/data-channel/inter"

func NewServerTCPBag() inter.ServerDataProcessor {
	return &serverTCPBagImpl{
		cliMap: make(map[string]inter.ClientDataProcessor),
	}
}

type serverTCPBagImpl struct {
	cliMap map[string]inter.ClientDataProcessor
}

func (impl *serverTCPBagImpl) OnTerminate(addr string) {
	delete(impl.cliMap, addr)
}

func (impl *serverTCPBagImpl) mustCli(addr string) inter.ClientDataProcessor {
	if dp, ok := impl.cliMap[addr]; ok {
		return dp
	}

	impl.cliMap[addr] = NewClientTCPBag()

	return impl.cliMap[addr]
}

func (impl *serverTCPBagImpl) OnRead(dIn *inter.ServerData) (dOut *inter.ServerData, err error) {
	dp := impl.mustCli(dIn.Addr)
	d, err := dp.OnRead(dIn.Data)

	if err != nil {
		return
	}

	if d == nil {
		return
	}

	dOut = &inter.ServerData{
		Addr: dIn.Addr,
		Data: d,
	}

	return
}

func (impl *serverTCPBagImpl) OnWrite(dIn *inter.ServerData) (dOut *inter.ServerData, err error) {
	dp := impl.mustCli(dIn.Addr)
	d, err := dp.OnWrite(dIn.Data)

	if err != nil {
		return
	}

	if d == nil {
		return
	}

	dOut = &inter.ServerData{
		Addr: dIn.Addr,
		Data: d,
	}

	return
}
