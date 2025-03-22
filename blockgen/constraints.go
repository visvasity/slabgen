// Copyright (c) 2025 Visvasity LLC

package blockgen

type AnyReader[W any] interface {
	~[]byte
	Writer() W
	BlockBytes() BlockBytes
}

type AnyWriter[R any] interface {
	Reader() R
	BlockBytes() BlockBytes
}

type FixSizeNumber interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}
