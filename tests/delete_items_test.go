// Copyright (c) 2025 Visvasity LLC

//go:build ignore

package tests

import (
	"testing"

	"github.com/visvasity/blockgen/output"
)

func TestDeleteItems(t *testing.T) {
	block := make([]byte, 8*1024)
	b := output.NewSuperBlock(block)
	if b == nil {
		t.Fatalf("wanted non-nil SuperBlock object")
	}
	v0 := b.Writer().AddJournalRegionSliceItem()
	v0.SetJournalOffset(0)
	v0.SetFileOffset(0)
	v0.SetRegionSize(1)
	v1 := b.Writer().AddJournalRegionSliceItem()
	v1.SetJournalOffset(1)
	v1.SetFileOffset(1)
	v1.SetRegionSize(1)
	v2 := b.Writer().AddJournalRegionSliceItem()
	v2.SetJournalOffset(2)
	v2.SetFileOffset(2)
	v2.SetRegionSize(1)
	v3 := b.Writer().AddJournalRegionSliceItem()
	v3.SetJournalOffset(3)
	v3.SetFileOffset(3)
	v3.SetRegionSize(1)
	if n := b.JournalRegionSliceLen(); n != 4 {
		t.Fatalf("wanted 4, got %d", n)
	}

	// Delete a range.
	b.Writer().DeleteJournalRegionSliceItems(1, 3)

	if n := b.JournalRegionSliceLen(); n != 2 {
		t.Fatalf("wanted 2, got %d", n)
	}
	x0 := b.JournalRegionSliceItemAt(0)
	if got := x0.JournalOffset(); got != 0 {
		t.Fatalf("wanted 0, got %d", got)
	}
	x1 := b.JournalRegionSliceItemAt(1)
	if got := x1.JournalOffset(); got != 3 {
		t.Fatalf("wanted 3, got %d", got)
	}
}
