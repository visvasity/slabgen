// Copyright (c) 2025 Visvasity LLC

package input

import "github.com/visvasity/slabgen/slabs"

type TestPair[T slabs.Number] struct {
	First, Second T
}

type TestMixedPair[T slabs.Number, S slabs.Struct] struct {
	Number T
	Struct S
}

type TestInt64Alias = int64

type TestNamedInt64 int64

type TestMultiLevelNamedInt64 TestNamedInt64

type TestNamedArrayOfInt64 [10]int64

type TestNamedArrayAliasOfInt64 = [10]int64

type TestNamedArrayOfArrayOfInt64 [10][20]int64

type TestStruct struct{ Int64 int64 }

type TestStructWithSlice struct{ Int64Slice []int64 }

type TestGenericBlock[T1 slabs.Number, S1 slabs.Struct] struct {
	/////////////////////
	// Basic type fields
	/////////////////////

	Basic int64

	NamedBasic TestNamedInt64

	MultiLevelNamedBasic TestMultiLevelNamedInt64

	BasicAlias TestInt64Alias

	BasicTypeParam T1

	//////////////////////
	// Struct type fields
	//////////////////////

	// EmptyStruct struct{}

	// AnonymousStruct struct{ Int64 int64 }

	// AnonymousStructWithSlice struct{ Int64Slice []int64 }

	// StructWithSlice TestStructWithSlice

	NamedStruct TestStruct

	StructTypeParam S1

	GenericStruct TestPair[int32]

	GenericMixedStruct TestMixedPair[int32, S1]

	GenericStructTypeParam TestPair[T1]

	//////////////////////////////
	// Array of Basic type fields
	//////////////////////////////

	ArrayOfBasic [10]int64

	ArrayOfNamedBasic [10]TestNamedInt64

	ArrayOfMultiLevelNamedBasic [10]TestMultiLevelNamedInt64

	ArrayOfBasicTypeParam [10]T1

	// NamedArrayAliasOfBasic TestNamedArrayAliasOfInt64

	// NamedArrayOfBasic TestNamedArrayOfInt64

	// ArrayOfArrayOfBasic [10][20]int64

	// ArrayOfArrayOfNamedBasic [10][20]TestNamedInt64

	// ArrayOfArrayOfMultiLevelNamedBasic [10][20]TestMultiLevelNamedInt64

	// ArrayOfNamedArrayOfBasic [10]TestNamedArrayOfInt64

	// NamedArrayOfArrayOfBasic TestNamedArrayOfArrayOfInt64

	///////////////////////////////
	// Array of Struct type fields
	///////////////////////////////

	// ArrayOfAnonymousStruct [10]struct{ Int64 int64 }

	ArrayOfBasicStruct [10]TestStruct

	ArrayOfStructTypeParam [10]S1

	ArrayOfGenericStruct [10]TestPair[int64]

	ArrayOfGenericStructTypeParam [10]TestPair[T1]

	ArrayOfGenericStructMixedTypeParam [10]TestMixedPair[int16, S1]

	// ArrayOfStructWithSlice [10]TestStructWithSlice

	/////////////////////
	// Slice type fields
	/////////////////////

	// FinalSlice []int8
	// FinalSlice []TestNamedInt64
	// FinalSlice []TestStruct
	FinalSlice []S1
	// FinalSlice []T1
	// FinalSlice []TestPair[int8]
	// FinalSlice []TestPair[T1]
	// FinalSlice []TestMixedPair[uint16, S1]
}

/////////////////////////////////////////////////////////////////////////////

type TestFinalSliceKind1[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []int8
}

type TestFinalSliceKind2[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []TestNamedInt64
}

type TestFinalSliceKind3[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []TestStruct
}

type TestFinalSliceKind4[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []S1
}

type TestFinalSliceKind5[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []T1
}

type TestFinalSliceKind6[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []TestPair[int8]
}

type TestFinalSliceKind7[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []TestPair[T1]
}

type TestFinalSliceKind8[T1 slabs.Number, S1 slabs.Struct] struct {
	FinalSlice []TestMixedPair[uint16, S1]
}
