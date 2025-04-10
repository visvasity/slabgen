// Copyright (c) 2025 Visvasity LLC

package slabutil

import (
	"fmt"

	"github.com/visvasity/slabgen/slabs"
)

func IntAt(v slabs.Bytes, offset int) int {
	switch SizeOfInt {
	case 8:
		return int(v.Int64At(offset))
	case 4:
		return int(v.Int32At(offset))
	case 2:
		return int(v.Int16At(offset))
	case 1:
		return int(v.Int8At(offset))
	}
	panic(fmt.Sprintf("IntAt: unhandled int size %d", SizeOfInt))
}

func SetIntAt(v slabs.Bytes, offset int, x int) {
	switch SizeOfInt {
	case 8:
		v.SetInt64At(offset, int64(x))
		return
	case 4:
		v.SetInt32At(offset, int32(x))
		return
	case 2:
		v.SetInt16At(offset, int16(x))
		return
	case 1:
		v.SetInt8At(offset, int8(x))
		return
	}
	panic(fmt.Sprintf("SetIntAt: unhandled int size %d", SizeOfInt))
}
