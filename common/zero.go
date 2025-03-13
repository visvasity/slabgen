// Copyright (c) 2025 Visvasity LLC

package common

import "bytes"

var zeros [4096]byte

// IsZero returns true if input slice is all zeros.
func IsZero[T ~[]byte](bs T) bool {
	size := len(bs)
	for i, sz := 0, 0; i < size; i += sz {
		sz = min(size-i, len(zeros))
		if bytes.Compare(bs[i:i+sz], zeros[:sz]) != 0 {
			return false
		}
	}
	return true
}

// SetZero writes zeros into the input slice.
func SetZero[T ~[]byte](bs T) {
	size := len(bs)
	for i, sz := 0, 0; i < size; i += sz {
		sz = min(size-i, len(zeros))
		copy(bs[i:i+sz], zeros[:sz])
	}
}
