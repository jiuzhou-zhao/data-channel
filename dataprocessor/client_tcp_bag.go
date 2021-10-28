package dataprocessor

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/jiuzhou-zhao/data-channel/inter"
)

var (
	ErrInvalidMagic = errors.New("invalidMagic")
)

var Magic = uint32(0x2F9b)

func NewClientTCPBag() inter.ClientDataProcessor {
	return &clientTCPBagImpl{}
}

type clientTCPBagImpl struct {
	readBuffer []byte
}

func (impl *clientTCPBagImpl) OnRead(dIn []byte) (dOut []byte, err error) {
	impl.readBuffer = append(impl.readBuffer, dIn...)
	if len(impl.readBuffer) < 8 {
		return
	}

	var magic, l uint32
	r := bytes.NewReader(impl.readBuffer)
	err = binary.Read(r, binary.BigEndian, &magic)
	if err != nil {
		return
	}
	if magic != Magic {
		err = ErrInvalidMagic

		return
	}
	err = binary.Read(r, binary.BigEndian, &l)
	if err != nil {
		return
	}

	if len(impl.readBuffer) < 8+int(l) {
		return
	}

	dOut = append(dOut, impl.readBuffer[8:8+l]...)
	impl.readBuffer = impl.readBuffer[8+l:]

	return
}

func (impl *clientTCPBagImpl) OnWrite(dIn []byte) (dOut []byte, err error) {
	buf := &bytes.Buffer{}
	err = binary.Write(buf, binary.BigEndian, Magic)
	if err != nil {
		return
	}
	l := uint32(len(dIn))
	err = binary.Write(buf, binary.BigEndian, l)
	if err != nil {
		return
	}

	err = binary.Write(buf, binary.BigEndian, dIn)
	if err != nil {
		return
	}

	dOut = buf.Bytes()

	return
}
