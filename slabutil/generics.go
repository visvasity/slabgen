// Copyright (c) 2025 Visvasity LLC

package slabutil

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"sync"

	"github.com/visvasity/slabgen/slabs"
)

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

func NumberAt[T slabs.Number](v []byte, offset int) T {
	var x T
	if _, err := binary.Decode([]byte(v[offset:]), binary.BigEndian, &x); err != nil {
		panic(err)
	}
	return x
}

func SetNumberAt[T slabs.Number](v []byte, offset int, x T) {
	if _, err := binary.Encode([]byte(v[offset:]), binary.BigEndian, x); err != nil {
		panic(err)
	}
}
