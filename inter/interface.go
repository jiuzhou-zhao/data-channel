package inter

import "context"

type ClientStatusOb interface {
	OnConnect()
	OnClose()
	OnException(err error)
}

type Client interface {
	Context() context.Context

	SetOb(ob ClientStatusOb)
	GetOb() ClientStatusOb

	ReadCh() chan []byte
	WriteCh() chan []byte

	Wait()
	CloseAndWait()
}

type ClientDataProcessor interface {
	OnRead(d []byte) ([]byte, error)
	OnWrite(d []byte) ([]byte, error)
}

//
//
//

type ServerStatusOb interface {
	OnConnect(addr string)
	OnClose(addr string)
	OnException(addr string, err error)
}
type ServerData struct {
	Addr string
	Data []byte
}

type Server interface {
	Context() context.Context

	SetOb(ob ServerStatusOb)
	GetOb() ServerStatusOb

	ReadCh() chan *ServerData
	WriteCh() chan *ServerData

	Wait()
	CloseAndWait()
}

type ServerDataProcessor interface {
	OnTerminate(addr string)
	OnRead(d *ServerData) (*ServerData, error)
	OnWrite(d *ServerData) (*ServerData, error)
}
