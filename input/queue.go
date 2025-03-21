// Copyright (c) 2025 Visvasity LLC

package input

type QueueDataBlock[T any] struct {
	NextLBA LBA

	TestNumericValue T

	ValuesSlice []T
}
