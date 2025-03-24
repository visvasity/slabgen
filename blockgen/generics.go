// Copyright (c) 2025 Visvasity LLC

package blockgen

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"sync"
)

type Number interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 // TODO: ~float32 | ~float64
}

type Struct interface {
}

type Reader[S Struct, W any] interface {
	~[]byte
	BlockBytes() BlockBytes

	Writer() W

	CopyTo(*S)
}

type Writer[S Struct, R any] interface {
	BlockBytes() BlockBytes

	Reader() R

	CopyFrom(*S)
}

var SizeOfInt = int(reflect.TypeFor[int]().Size())
var SizeOfSlice = int(reflect.TypeOf([]int{}).Size())
var OffsetOfSliceCap = int(SizeOfSlice - 1*SizeOfInt)
var OffsetOfSliceLen = int(SizeOfSlice - 2*SizeOfInt)

type OffsetsMap struct {
	sync.Map // reflect.Type -> []int
}

// OffsetsFor returns slice of offsets for all fields in the input struct type.
func OffsetsFor[T any](offsetsMap *OffsetsMap) []int {
	stype := reflect.TypeFor[T]()
	if stype.Kind() != reflect.Struct {
		return nil
	}

	if offsetsMap != nil {
		if x, ok := offsetsMap.Load(stype); ok {
			return x.([]int)
		}
	}

	vs := make([]int, stype.NumField())
	for i := range vs {
		vs[i] = int(stype.Field(i).Offset)
	}

	if offsetsMap != nil {
		offsetsMap.Store(stype, vs)
	}
	return vs
}

func SizeFor[T any]() int {
	return int(reflect.TypeFor[T]().Size())
}

func AlignFor[T any]() int {
	return reflect.TypeFor[T]().Align()
}

func ElemSizeFor[T any]() int {
	stype := reflect.TypeFor[T]()
	if stype.Kind() == reflect.Array || stype.Kind() == reflect.Slice {
		return int(stype.Elem().Size())
	}
	var v T
	panic(fmt.Sprintf("input type %T is not a slice or an array", v))
}

func NumberAt[T Number](v BlockBytes, offset int) T {
	var x T
	if _, err := binary.Decode([]byte(v[offset:]), binary.BigEndian, &x); err != nil {
		panic(err)
	}
	return x
}

func SetNumberAt[T Number](v BlockBytes, offset int, x T) {
	if _, err := binary.Encode([]byte(v[offset:]), binary.BigEndian, x); err != nil {
		panic(err)
	}
}
