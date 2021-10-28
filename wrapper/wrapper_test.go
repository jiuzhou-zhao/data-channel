package wrapper

import (
	"context"
	"fmt"
	"github.com/jiuzhou-zhao/data-channel/dataprocessor"
	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/jiuzhou-zhao/data-channel/tcp"
	"github.com/jiuzhou-zhao/data-channel/udp"
	"github.com/sgostarter/i/logger"
	"github.com/stretchr/testify/assert"
	"strconv"
	"sync"
	"testing"
	"time"
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

func TestUDPWrapper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log := logger.NewWrapper(logger.NewCommLogger(&logger.FmtRecorder{}))

	rs, err := udp.NewServer(ctx, "127.0.0.1:11111", &serverStatusOb{}, log)
	assert.Nil(t, err)

	s := NewServer(rs, dataprocessor.NewServerEncryptDataProcess([]byte("1")))

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

		rc, err := udp.NewClient(ctx, "127.0.0.1:11111", &clientStatusOb{id: start}, log)
		assert.Nil(t, err)

		c := NewClient(rc, dataprocessor.NewClientEncryptDataProcess([]byte("1")))

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
}

func TestTCPWrapper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log := logger.NewWrapper(logger.NewCommLogger(&logger.FmtRecorder{}))

	rs, err := tcp.NewServer(ctx, "127.0.0.1:11111", &serverStatusOb{}, log)
	assert.Nil(t, err)

	s := NewServer(rs, dataprocessor.NewServerTCPBag(), dataprocessor.NewServerEncryptDataProcess([]byte("1")))

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

		rc, err := tcp.NewClient(ctx, "127.0.0.1:11111", &clientStatusOb{id: start}, log)
		assert.Nil(t, err)

		c := NewClient(rc, dataprocessor.NewClientTCPBag(), dataprocessor.NewClientEncryptDataProcess([]byte("1")))

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
}
