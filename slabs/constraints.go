// Copyright (c) 2025 Visvasity LLC

package slabs

type Number interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 // TODO: ~float32 | ~float64
}

type Struct interface {
}

type Reader[S Struct, W any] interface {
	~[]byte
	SlabBytes() Bytes

	Writer() W
	CopyTo(*S)
}

type Writer[S Struct, R any] interface {
	SlabBytes() Bytes

	Reader() R
	CopyFrom(*S)
}
