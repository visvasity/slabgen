// Copyright (c) 2025 Visvasity LLC

package input

import (
	"github.com/visvasity/blockgen/blockgen"
	"github.com/visvasity/storage"
)

type HeaderBlock struct {
	FirstLBA storage.LBA
	LastLBA  storage.LBA

	ValueSize    int64
	ValueSizeCap int64
}

type DataBlock[T blockgen.Struct] struct {
	NextLBA storage.LBA

	ValuesSlice []T
}
