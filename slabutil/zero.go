// Copyright (c) 2025 Visvasity LLC

package slabutil

import "github.com/visvasity/slabgen/slabs"

// IsZero returns true if input slice is all zeros.
func IsZero[T ~[]byte](bs T) bool {
	return slabs.Bytes(bs).IsZero()
}

// SetZero writes zeros into the input slice.
func SetZero[T ~[]byte](bs T) {
	slabs.Bytes(bs).SetZero()
}
