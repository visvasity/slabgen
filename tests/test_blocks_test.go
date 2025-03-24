// Copyright (c) 2025 Visvasity LLC

package tests

import (
	"math/rand"
	"testing"

	"github.com/visvasity/blockgen/input"
	"github.com/visvasity/blockgen/output"
)

func TestPair(t *testing.T) {
	block := make([]byte, 1024)
	for i := range block {
		block[i] = byte(rand.Intn(256))
	}

	p := output.NewTestPair[int64](block)
	if !p.IsZero() {
		t.Fatalf("NewTestPairReader must return a zero value")
	}

	var pair input.TestPair[int64]
	randomize(&pair)
	// t.Logf("pair=%#v", pair)

	p.Writer().CopyFrom(&pair)
	if p.First() != pair.First {
		t.Fatalf("want %d, got %d", pair.First, p.First())
	}
	if p.Second() != pair.Second {
		t.Fatalf("want %d, got %d", pair.Second, p.Second())
	}

	block2 := make([]byte, 1024)
	copy(block2, block)
	p2, err := output.OpenTestPair[int64](block2)
	if err != nil {
		t.Fatal(err)
	}
	if p2.First() != p.First() {
		t.Fatalf("want %d, got %d", p.First(), p2.First())
	}
	if p2.Second() != p.Second() {
		t.Fatalf("want %d, got %d", p.Second(), p2.Second())
	}
}
