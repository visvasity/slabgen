// Copyright (c) 2025 Visvasity LLC

package input

import (
	"crypto/md5"
	"crypto/sha256"
	"math"
)

type DBA uint64
type LBA uint64
type PBA uint64

type ObjectID int64

type LSN int64

type BlockType uint16

const (
	ChecksumLen = 32
	DeferredLSN = math.MaxInt64
	InvalidPBA  = 0x0000_FFFF_FFFF_FFFF
)

const (
	ZeroBlockType            BlockType = 0
	SuperBlockType           BlockType = 1
	ObjectBlockType          BlockType = 2
	ObjectListBlockType      BlockType = 3
	DBARegionListBlockType   BlockType = 4
	FreeRegionListBlockType  BlockType = 5
	FreeDBAListBlockType     BlockType = 6
	FreePBAListBlockType     BlockType = 7
	FreeLBAListBlockType     BlockType = 8
	NonDataLBAListBlockType  BlockType = 9
	LBAMetadataBlockType     BlockType = 10
	DeferredPBAListBlockType BlockType = 11
)

type LinkedListType uint16

const (
	// Lists for SuperBlock.
	ObjectListType     LinkedListType = 1
	DBARegionListType  LinkedListType = 2
	FreeRegionListType LinkedListType = 3

	// Lists for Object blocks.
	FreeDBAListType     LinkedListType = 4
	FreePBAListType     LinkedListType = 5
	FreeLBAListType     LinkedListType = 6
	NonDataLBAListType  LinkedListType = 7
	DeferredPBAListType LinkedListType = 8
)

const (
	NonDataBlockFlag   = 0x1000
	TemporaryBlockFlag = 0x2000
)

type FlagsPBA uint64

func (v FlagsPBA) Flags() uint16 {
	return uint16(v >> 48)
}

func (v FlagsPBA) PBA() PBA {
	return PBA((v << 16) >> 16)
}

func (v FlagsPBA) WithFlags(flags uint16) FlagsPBA {
	return (FlagsPBA(flags) << 48) | FlagsPBA(v.PBA())
}

func (v FlagsPBA) WithPBA(pba PBA) FlagsPBA {
	return ((v >> 48) << 48) | FlagsPBA((pba<<16)>>16)
}

func (v FlagsPBA) IsTemporaryBlock() bool {
	return v.Flags()&TemporaryBlockFlag != 0
}

func (v FlagsPBA) IsDataBlock() bool {
	return v.Flags()&NonDataBlockFlag == 0
}

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
	MaxDBARegionListBlockItems      uint32
	MaxFreeDataRegionListBlockItems uint32
	MaxDeferredPBAListBlockItems    uint32
}

type ObjectOptions struct {
	NumRootBlocks uint8
}

type LBAMetadata struct {
	PBAWithFlags     FlagsPBA
	LastUpdateLSN    LSN
	UserDataChecksum [ChecksumLen]byte
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

	JournalRegionSlice []JournalRegion
}

type ObjectBlock struct {
	Header BlockHeader

	Options ObjectOptions

	EndLBA LBA

	FreeDBAList LinkedList
	FreePBAList LinkedList
	FreeLBAList LinkedList

	NonDataLBAList LinkedList

	DeferredPBAList LinkedList

	DataRegionSlice []DataRegion
}

type ObjectListBlock struct {
	Header BlockHeader

	NextDBA DBA

	ObjectIDDataSlice []ObjectIDData
}

type LBAMetadataBlock struct {
	Header BlockHeader

	TempsIncarnationID uint64

	MetadataSlice []LBAMetadata
}

type DBARegionListBlock struct {
	Header BlockHeader

	NextDBA DBA

	DBARegionSlice []Region
}

type FreeRegionListBlock struct {
	Header BlockHeader

	NextDBA DBA

	FreeRegionSlice []Region
}

type FreeDBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	DBARangeSlice []BlockRange
}

type FreePBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	PBARangeSlice []BlockRange
}

type FreeLBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	LBARangeSlice []BlockRange
}

type NonDataLBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	LBASlice []LBA
}

type DeferredPBAListBlock struct {
	Header BlockHeader

	NextDBA DBA

	PBASlice []PBA
}
