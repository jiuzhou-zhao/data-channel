package dataprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	buf := []byte{0x01, 0x02, 0x03}
	buf1 := buf[1:2]
	buf2 := buf[2:]
	buf[0] = 0x10
	buf[1] = 0x11
	buf[2] = 0x12
	t.Log(buf1, buf2)
}

func TestClientTCPBag(t *testing.T) {
	dp := NewClientTCPBag()
	bag1, err := dp.OnWrite([]byte{0x01, 0x02, 0x03})
	assert.Nil(t, err)

	bag, err := dp.OnRead(bag1)
	assert.Nil(t, err)
	assert.Equal(t, bag, []byte{0x01, 0x02, 0x03})
}

func TestClientTCPBag2(t *testing.T) {
	dp := NewClientTCPBag()
	bag1, err := dp.OnWrite([]byte{0x01, 0x02, 0x03})
	assert.Nil(t, err)

	bag, err := dp.OnRead(bag1[:1])
	assert.Nil(t, err)
	assert.Nil(t, bag)

	bag, err = dp.OnRead(bag1[1:2])
	assert.Nil(t, err)
	assert.Nil(t, bag)

	bag, err = dp.OnRead(bag1[2:])
	assert.Nil(t, err)
	assert.Equal(t, bag, []byte{0x01, 0x02, 0x03})
}

func TestClientTCPBag3(t *testing.T) {
	dp := NewClientTCPBag()
	bag1, err := dp.OnWrite([]byte{0x01, 0x02, 0x03})
	assert.Nil(t, err)
	bag2, err := dp.OnWrite([]byte{0x04, 0x05, 0x06, 0x07})
	assert.Nil(t, err)

	bag, err := dp.OnRead(bag1[:1])
	assert.Nil(t, err)
	assert.Nil(t, bag)

	bag, err = dp.OnRead(bag1[1:2])
	assert.Nil(t, err)
	assert.Nil(t, bag)

	bag, err = dp.OnRead(append(bag1[2:], bag2[:6]...))
	assert.Nil(t, err)
	assert.Equal(t, bag, []byte{0x01, 0x02, 0x03})

	bag, err = dp.OnRead(bag2[6:])
	assert.Nil(t, err)
	assert.Equal(t, bag, []byte{0x04, 0x05, 0x06, 0x07})
}
