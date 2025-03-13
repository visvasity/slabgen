// Copyright (c) 2025 Visvasity LLC

package common

import "encoding/binary"

type BlockBytes []byte

func (v BlockBytes) IsZero() bool {
	return IsZero(v)
}

func (v BlockBytes) SetZero() {
	SetZero(v)
}

func (v BlockBytes) Copy(dst, src int) {
	copy(v[dst:], v[src:])
}

func (v BlockBytes) BoolAt(offset int) bool {
	return v[offset] != 0
}

func (v BlockBytes) SetBoolAt(offset int, x bool) {
	if x {
		v[offset] = 1
	} else {
		v[offset] = 0
	}
}

func (v BlockBytes) Int8At(offset int) int8 {
	return int8(v[offset])
}

func (v BlockBytes) SetInt8At(offset int, x int8) {
	v[offset] = byte(x)
}

func (v BlockBytes) Uint8At(offset int) uint8 {
	return uint8(v[offset])
}

func (v BlockBytes) SetUint8At(offset int, x uint8) {
	v[offset] = byte(x)
}

func (v BlockBytes) Int16At(offset int) int16 {
	return int16(binary.BigEndian.Uint16(v[offset : offset+2]))
}

func (v BlockBytes) SetInt16At(offset int, x int16) {
	binary.BigEndian.PutUint16(v[offset:offset+2], uint16(x))
}

func (v BlockBytes) Uint16At(offset int) uint16 {
	return binary.BigEndian.Uint16(v[offset : offset+2])
}

func (v BlockBytes) SetUint16At(offset int, x uint16) {
	binary.BigEndian.PutUint16(v[offset:offset+2], x)
}

func (v BlockBytes) Int32At(offset int) int32 {
	return int32(binary.BigEndian.Uint32(v[offset : offset+4]))
}

func (v BlockBytes) SetInt32At(offset int, x int32) {
	binary.BigEndian.PutUint32(v[offset:offset+4], uint32(x))
}

func (v BlockBytes) Uint32At(offset int) uint32 {
	return binary.BigEndian.Uint32(v[offset : offset+4])
}

func (v BlockBytes) SetUint32At(offset int, x uint32) {
	binary.BigEndian.PutUint32(v[offset:offset+4], x)
}

func (v BlockBytes) Int64At(offset int) int64 {
	return int64(binary.BigEndian.Uint64(v[offset : offset+8]))
}

func (v BlockBytes) SetInt64At(offset int, x int64) {
	binary.BigEndian.PutUint64(v[offset:offset+8], uint64(x))
}

func (v BlockBytes) Uint64At(offset int) uint64 {
	return binary.BigEndian.Uint64(v[offset : offset+8])
}

func (v BlockBytes) SetUint64At(offset int, x uint64) {
	binary.BigEndian.PutUint64(v[offset:offset+8], x)
}

// ByteSliceAt returns a copy of the bytes at the given offset.
func (v BlockBytes) ByteSliceAt(offset int, size int) []byte {
	bs := make([]byte, size)
	copy(bs, v[offset:offset+size])
	return bs
}

func (v BlockBytes) SetByteSliceAt(offset int, bs []byte) {
	copy(v[offset:offset+len(bs)], bs)
}
