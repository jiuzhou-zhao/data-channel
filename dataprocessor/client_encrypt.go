package dataprocessor

import (
	"crypto/sha256"

	"github.com/jiuzhou-zhao/data-channel/inter"
	"github.com/sgostarter/libencrypt/aes"
)

func NewClientEncryptDataProcess(key []byte) inter.ClientDataProcessor {
	rKey := sha256.Sum256(key)
	impl := &clientAesDataProcessorImpl{
		key: rKey[:16],
	}

	return impl
}

type clientAesDataProcessorImpl struct {
	key []byte
}

func (impl *clientAesDataProcessorImpl) OnRead(d []byte) ([]byte, error) {
	return aes.ECBDecrypt(d, impl.key)
}

func (impl *clientAesDataProcessorImpl) OnWrite(d []byte) ([]byte, error) {
	return aes.ECBEncrypt(d, impl.key)
}
