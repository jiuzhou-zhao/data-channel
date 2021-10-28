package inter

type UnimplementedClientStatusOb struct {
}

func (ob *UnimplementedClientStatusOb) OnConnect() {

}

func (ob *UnimplementedClientStatusOb) OnClose() {

}

func (ob *UnimplementedClientStatusOb) OnException(error) {

}

type UnimplementedServerStatusOb struct {
}

func (ob *UnimplementedServerStatusOb) OnConnect(_ string) {

}

func (ob *UnimplementedServerStatusOb) OnClose(_ string) {

}

func (ob *UnimplementedServerStatusOb) OnException(_ string, _ error) {

}
