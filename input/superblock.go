// Copyright (c) 2025 Visvasity LLC

package input

import (
	"crypto/md5"
	"crypto/sha256"
)

type DBA uint64
type LBA uint64
type PBA uint64

type LSN int64

type ObjectID int64

type BlockType uint16

const (
	ZeroBlockType                BlockType = 0
	SuperBlockType               BlockType = 1
	ObjectBlockType              BlockType = 8
	RawFreePBAListBlockType      BlockType = 50
	RawLBAMetadataBlockType      BlockType = 51
	RawDeferredPBAQueueBlockType BlockType = 52
	RawNonDataLBAListBlockType   BlockType = 53
	RawFreeLBAListBlockType      BlockType = 54
	RawFreeDBAListBlockType      BlockType = 55
	RawFreeRegionListBlockType   BlockType = 56
	RawObjectListBlockType       BlockType = 57
	RawDBARegionListBlockType    BlockType = 58
)

type LinkedListType uint16

const (
	FreeDBAListType        LinkedListType = 0
	FreePBAListType        LinkedListType = 1
	FreeLBAListType        LinkedListType = 2
	NonDataLBAListType     LinkedListType = 3
	FreeDataRegionListType LinkedListType = 4
	ObjectListType         LinkedListType = 5
	DoubleBlocksListType   LinkedListType = 6
	DBARegionListType      LinkedListType = 7
)

type BlockRange struct {
	Block uint64
	Count int32
}

type Region struct {
	BeginPBA, EndPBA PBA
}

type DataRegion struct {
	BeginPBA, EndPBA PBA

	FirstLBA LBA
}

type LinkedList struct {
	HeadDBA DBA

	NumValues     int64
	NumLinkBlocks int32
	NumFreeItems  int32
}

type JournalRegion struct {
	JournalOffset int64

	FileOffset int64

	RegionSize int64
}

type BlockHeader struct {
	Checksum [sha256.Size]byte

	BlockLSN LSN

	BlockTypeID BlockType
}

type ObjectIDData struct {
	ID ObjectID

	NameMD5 [md5.Size]byte

	ObjectBlockDBA DBA

	DeletedLSN LSN
}

type StorageOptions struct {
	BlockSize uint32

	MaxLBAMetadataBlockItems        uint32
	MaxObjectListBlockItems         uint32
	MaxFreeDBAListBlockItems        uint32
	MaxFreePBAListBlockItems        uint32
	MaxFreeLBAListBlockItems        uint32
	MaxNonDataLBAListBlockItems     uint32
	MaxFreeDataRegionListBlockItems uint32
	MaxDeferredPBAListBlockItems    uint32
}

type ObjectOptions struct {
	NumRootBlocks int8
}

type LBAMetadata struct {
	CurrentPBA       PBA
	LastUpdateLSN    LSN
	UserDataChecksum [32]byte
}

type ZeroBlock struct {
	Header BlockHeader
}

type SuperBlock struct {
	Header BlockHeader

	Options StorageOptions

	EndPBA PBA

	MaxObjectID ObjectID

	DBARegionList LinkedList

	ObjectList LinkedList

	FreeDBAList LinkedList

	FreeDataRegionList LinkedList

	SyncedCacheLSN LSN

	JournalHeadOffset int64
	JournalTailOffset int64

	JournalRegionList []JournalRegion
}

type ObjectBlock struct {
	Header BlockHeader

	Options ObjectOptions

	FreeDBAList LinkedList
	FreePBAList LinkedList
	FreeLBAList LinkedList

	DeferredPBAList LinkedList

	DataRegionList []DataRegion
}

type ObjectListBlock struct {
	Header BlockHeader

	NextDBA DBA

	ObjectIDDataList []ObjectIDData
}

type LBAMetadataBlock struct {
	Header BlockHeader

	IncarnationID uint64

	MetadataList []LBAMetadata
}

type DBARegionListBlock struct {
	Header BlockHeader

	NextDBA DBA

	DBARegionList []Region
}

type FreeRegionListBlock struct {
	Header BlockHeader

	NextDBA DBA

	FreeRegionList []Region
}

type FreeDBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	DBARangeList []BlockRange
}

type FreePBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	PBARangeList []BlockRange
}

type FreeLBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	LBARangeList []BlockRange
}

type NonDataLBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	LBAList []LBA
}

type DeferredPBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	PBAList []PBA
}
