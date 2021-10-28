package tcp

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/i/logger"
	"github.com/stretchr/testify/assert"
)

type serverStatusOb struct {
}

func (ob *serverStatusOb) OnConnect(addr string) {
	fmt.Println("SERVER OnConnect:", addr)
}
func (ob *serverStatusOb) OnClose(addr string) {
	fmt.Println("SERVER OnClose:", addr)
}
func (ob *serverStatusOb) OnException(addr string, err error) {
	fmt.Println("SERVER OnException:", addr, err)
}

type clientStatusOb struct {
	id int
}

func (ob *clientStatusOb) OnConnect() {
	fmt.Println("CLIENT OnConnect:", ob.id)
}
func (ob *clientStatusOb) OnClose() {
	fmt.Println("CLIENT OnClose:", ob.id)
}
func (ob *clientStatusOb) OnException(err error) {
	fmt.Println("CLIENT OnException:", ob.id, err)
}

func TestTCP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log := logger.NewWrapper(logger.NewCommLogger(&logger.FmtRecorder{}))

	s, err := NewServer(ctx, "127.0.0.1:11111", &serverStatusOb{}, log)
	assert.Nil(t, err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		loop := true
		for loop {
			select {
			case <-ctx.Done():
				loop = false

				continue
			case di := <-s.ReadCh():
				fmt.Println("server receive:", di.Addr, string(di.Data))
				v, _ := strconv.Atoi(string(di.Data))
				v++
				s.WriteCh() <- &inter.ServerData{
					Addr: di.Addr,
					Data: []byte(strconv.Itoa(v)),
				}
			}
		}
	}()

	fnCli := func(start int) {
		defer wg.Done()

		c, err := NewClient(ctx, "127.0.0.1:11111", &clientStatusOb{id: start}, log)
		assert.Nil(t, err)

		loop := true
		for loop {
			select {
			case <-ctx.Done():
				loop = false

				continue
			case d := <-c.ReadCh():
				fmt.Println("client receive:", string(d))
				start, _ = strconv.Atoi(string(d))
			case <-time.After(time.Second):
				c.WriteCh() <- []byte(strconv.Itoa(start))
			}
		}
	}

	wg.Add(1)
	go fnCli(10)

	wg.Add(1)
	go fnCli(100)

	wg.Wait()

	s.CloseAndWait()
}
