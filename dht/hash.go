package main

import (
	"crypto/md5"
	"encoding/binary"
	"io"
)

type hashFace interface {
	hash(key string) uint64
}

type defaultHash struct{}

func (d *defaultHash) hash(key string) uint64 {
	h := md5.New()
	io.WriteString(h, key)
	return binary.LittleEndian.Uint64(h.Sum(nil)[:8])
}

func newDefaultHash() hashFace {
	return &defaultHash{}
}
