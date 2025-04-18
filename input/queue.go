// Copyright (c) 2025 Visvasity LLC

package input

import (
	"github.com/visvasity/slabgen/slabs"
	"github.com/visvasity/storage"
)

type HeaderBlock struct {
	FirstLBA storage.LBA
	LastLBA  storage.LBA

	ValueSize    int64
	ValueSizeCap int64
}

type DataBlock[T slabs.Struct] struct {
	NextLBA storage.LBA

	ValuesSlice []T
}
