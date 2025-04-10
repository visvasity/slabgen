// Copyright (c) 2025 Visvasity LLC

package tests

import (
	"encoding/json"
	"testing"

	"github.com/visvasity/slabgen/input"
	"github.com/visvasity/slabgen/output"
)

func TestSuperBlock(t *testing.T) {
	s1 := &input.SuperBlock{
		JournalRegionSlice: make([]input.JournalRegion, 30),
	}
	randomize(s1)

	b1 := make([]byte, 8*1024)
	v1 := output.NewSuperBlock(b1)
	v1.Writer().CopyFrom(s1)

	b2 := make([]byte, 8*1024)
	copy(b2, b1)
	v2, err := output.OpenSuperBlock(b2)
	if err != nil {
		t.Fatal(err)
	}
	s2 := new(input.SuperBlock)
	v2.CopyTo(s2)

	if !deepEqual(s1, s2) {
		js1, _ := json.MarshalIndent(s1, "", "  ")
		js2, _ := json.MarshalIndent(s2, "", "  ")
		t.Logf("s1=%s", js1)
		t.Logf("s2=%s", js2)
		t.Fatalf("wanted s1 == s2, got unequal")
	}
}
