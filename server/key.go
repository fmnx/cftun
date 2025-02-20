package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"golang.org/x/crypto/curve25519"
)

type Key [32]byte

func (k *Key) String() string {
	return base64.StdEncoding.EncodeToString(k[:])
}

func (k *Key) IsZero() bool {
	var zeros Key
	return subtle.ConstantTimeCompare(zeros[:], k[:]) == 1
}

func (k *Key) Public() *Key {
	var p [32]byte
	curve25519.ScalarBaseMult(&p, (*[32]byte)(k))
	return (*Key)(&p)
}

func NewPresharedKey() (*Key, error) {
	var k [32]byte
	_, err := rand.Read(k[:])
	if err != nil {
		return nil, err
	}
	return (*Key)(&k), nil
}

func NewPrivateKey() *Key {
	k, err := NewPresharedKey()
	if err != nil {
		return nil
	}
	k[0] &= 248
	k[31] = (k[31] & 127) | 64
	return k
}
