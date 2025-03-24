// Copyright (c) 2025 Visvasity LLC

//go:build ignore

package tests

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/visvasity/blockgen/blocks"
)

func TestNonDataLBAListBlock(t *testing.T) {
	block := make([]byte, 8*1024)
	b := blocks.NewNonDataLBAListBlock(block)
	if b == nil {
		t.Fatalf("wanted non-nil NonDataLBAListBlock")
	}
	if x := b.Header().BlockLSN(); x != 0 {
		t.Fatalf("BlockLSN must be zero")
	}
	if x := b.NextDBA(); x != 0 {
		t.Fatalf("NextDBA must be zero")
	}
	if n := b.LBAListCap(); n != 1014 {
		t.Fatalf("LBAList cap must be %d; got %d", 1014, n)
	}
	if n := b.LBAListLen(); n != 0 {
		t.Fatalf("LBAList length must be zero")
	}
	var lbas []uint64
	for i := 0; i < b.LBAListCap(); i++ {
		lba := rand.Uint64()
		lbas = append(lbas, lba)
		b.Writer().AppendLBAList(blocks.LBA(lba))
	}
	lsn := blocks.LSN(rand.Int64())
	dba := blocks.DBA(rand.Uint64())
	b.Header().Writer().SetBlockLSN(lsn)
	b.Writer().SetNextDBA(dba)

	bb, err := blocks.OpenNonDataLBAListBlock(block)
	if err != nil {
		t.Fatal(err)
	}
	if x := bb.Header().BlockLSN(); x != lsn {
		t.Fatalf("want %d, got %d", lsn, x)
	}
	if x := bb.NextDBA(); x != dba {
		t.Fatalf("want %d, got %d", dba, x)
	}
	var lbaList []uint64
	for i := 0; i < bb.LBAListLen(); i++ {
		lbaList = append(lbaList, uint64(bb.LBAListItemAt(i)))
	}
	if !slices.Equal(lbas, lbaList) {
		t.Fatalf("written slice values did not match read slice values")
	}

	// Try sorting the list items.

	bb.Writer().SortLBAListFunc(cmp.Compare[blocks.LBA])

	lbaList = []uint64{}
	for i := 0; i < bb.LBAListLen(); i++ {
		lbaList = append(lbaList, uint64(bb.LBAListItemAt(i)))
	}
	if !slices.IsSorted(lbaList) {
		t.Fatalf("wanted sorted lba list")
	}

	// Try removing items.

	nitems := bb.LBAListLen()
	bb.Writer().RemoveLBAListItemAt(0)
	if bb.LBAListLen() != nitems-1 {
		t.Fatalf("remove first item must reduce lba list size")
	}
	bb.Writer().RemoveLBAListItemAt(bb.LBAListLen() - 1)
	if bb.LBAListLen() != nitems-2 {
		t.Fatalf("remove last item must reduce lba list size")
	}
	bb.Writer().RemoveLBAListItemAt(rand.N(nitems - 2))
	if bb.LBAListLen() != nitems-3 {
		t.Fatalf("remove item must reduce lba list size")
	}
}
