// Copyright (c) 2025 Visvasity LLC

package slabs

import (
	"bytes"
	"encoding/binary"
)

type Bytes []byte

var zeros [4096]byte

func (v Bytes) IsZero() bool {
	size := len(v)
	for i, sz := 0, 0; i < size; i += sz {
		sz = min(size-i, len(zeros))
		if bytes.Compare(v[i:i+sz], zeros[:sz]) != 0 {
			return false
		}
	}
	return true
}

func (v Bytes) SetZero() {
	size := len(v)
	for i, sz := 0, 0; i < size; i += sz {
		sz = min(size-i, len(zeros))
		copy(v[i:i+sz], zeros[:sz])
	}
}

func (v Bytes) Int8At(offset int) int8 {
	return int8(v[offset])
}

func (v Bytes) SetInt8At(offset int, x int8) {
	v[offset] = byte(x)
}

func (v Bytes) Uint8At(offset int) uint8 {
	return uint8(v[offset])
}

func (v Bytes) SetUint8At(offset int, x uint8) {
	v[offset] = byte(x)
}

func (v Bytes) Int16At(offset int) int16 {
	return int16(binary.BigEndian.Uint16(v[offset : offset+2]))
}

func (v Bytes) SetInt16At(offset int, x int16) {
	binary.BigEndian.PutUint16(v[offset:offset+2], uint16(x))
}

func (v Bytes) Uint16At(offset int) uint16 {
	return binary.BigEndian.Uint16(v[offset : offset+2])
}

func (v Bytes) SetUint16At(offset int, x uint16) {
	binary.BigEndian.PutUint16(v[offset:offset+2], x)
}

func (v Bytes) Int32At(offset int) int32 {
	return int32(binary.BigEndian.Uint32(v[offset : offset+4]))
}

func (v Bytes) SetInt32At(offset int, x int32) {
	binary.BigEndian.PutUint32(v[offset:offset+4], uint32(x))
}

func (v Bytes) Uint32At(offset int) uint32 {
	return binary.BigEndian.Uint32(v[offset : offset+4])
}

func (v Bytes) SetUint32At(offset int, x uint32) {
	binary.BigEndian.PutUint32(v[offset:offset+4], x)
}

func (v Bytes) Int64At(offset int) int64 {
	return int64(binary.BigEndian.Uint64(v[offset : offset+8]))
}

func (v Bytes) SetInt64At(offset int, x int64) {
	binary.BigEndian.PutUint64(v[offset:offset+8], uint64(x))
}

func (v Bytes) Uint64At(offset int) uint64 {
	return binary.BigEndian.Uint64(v[offset : offset+8])
}

func (v Bytes) SetUint64At(offset int, x uint64) {
	binary.BigEndian.PutUint64(v[offset:offset+8], x)
}
