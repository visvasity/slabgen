// Copyright (c) 2025 Visvasity LLC

// For example, given this snippet,
//
//   package blktypes
//
//   import "github.com/visvasity/storage/journal"
//
//   type DBA DBA
//
//   type ObjectID int32
//
//   type LinkedList struct {
//   	HeadDBA DBA
//
//   	NumValues     int64
//   	NumLinkBlocks int32
//   	NumFreeItems  int32
//   }
//
//   type JournalRegion struct {
//   	JournalOffset int64
//   	FileOffset    int64
//   	RegionSize    int64
//   }
//
//   type SuperBlock struct {
//     ...
//
//     EndBlock uint64
//
//     MaxObjectID ObjectID
//
//     SyncedCacheLSN journal.LSN
//
//     JournalHead, JournalTail int64
//
//     ObjectList LinkedList
//
//     JournalRegionSlice []JournalRegion
//   }
//
// runnng this command
//
//   blockgen -inpkg ./blktypes -outdir ./blocks SuperBlock
//
// will create file superblock.blockgen.go, in ./blocks directory containing
// SuperBlock data type with the following interface:
//
//   //
//   // LinkedList member field type
//   //
//
//   type LinkedList []byte
//   type LinkedListWriter struct { LinkedList }
//
//   func (v LinkedList) Writer() LinkedListWriter
//   func (v LinkedListWriter) Reader() LinkedList
//
//   func (v LinkedList) NextDBA() blktypes.DBA
//   func (v LinkedList) NumValues() int64
//   func (v LinkedList) NumLinkBlocks() int32
//   func (v LinkedList) NumFreeItems() int32
//
//   func (v LinkedListWriter) SetNextDBA(blktypes.DBA)
//   func (v LinkedListWriter) SetNumValues(int64)
//   func (v LinkedListWriter) SetNumLinkBlocks(int32)
//   func (v LinkedListWriter) SetNumFreeItems(int32)
//
//   //
//   // JournalRegion member field type
//   //
//
//   type JournalRegion []byte
//   type JournalRegionWriter struct { JournalRegion }
//
//   func (v JournalRegion) Writer() JournalRegionWriter
//   func (v JournalRegionWriter) Reader() JournalRegion
//
//   func (v JournalRegion) JournalOffset() int64
//   func (v JournalRegion) FileOffset() int64
//   func (v JournalRegion) RegionSize() int64
//
//   func (v JournalRegionWriter) SetJournalOffset(int64)
//   func (v JournalRegionWriter) SetFileOffset(int64)
//   func (v JournalRegionWriter) SetRegionSize(int64)
//
//   //
//   // SuperBlock block type
//   //
//
//   type SuperBlock []byte
//   type SuperBlockWriter struct { SuperBlock }
//
//   func (v SuperBlock) Writer() SuperBlockWriter
//   func (v SuperBlockWriter) Reader() SuperBlock
//
//   func (v SuperBlock) EndBlock() uint64
//   func (v SuperBlock) MaxobjecTID() blktypes.ObjectID
//   func (v SuperBlock) SyncedCacheLSN() blktypes.SyncedCacheLSN
//   func (v SuperBlock) JournalHead() int64
//   func (v SuperBlock) JournalTail() int64
//
//   func (v SuperBlock) ObjectList() LinkedList
//
//   func (v SuperBlock) JournalRegionSliceLen() int
//   func (v SuperBlock) JournalRegionSliceCap() int
//   func (v SuperBlock) JournalRegionSliceItemAt(index int) JournalRegion
//   func (v SuperBlock) FindJournalRegionSliceFunc(cmp func(x JournalRegion) int) (int, bool)
//   func (v SuperBlock) AllJournalRegionSlice() iter.Seq2[int, JournalRegion]
//
//   func (v SuperBlockWriter) SetEndBlock(uint64)
//   func (v SuperBlockWriter) SetMaxObjectID(blktypes.ObjectID)
//   func (v SuperBlockWriter) SyncedCacheLSN(blktypes.SyncedCacheLSN)
//   func (v SuperBlockWriter) SetJournalHead(int64)
//   func (v SuperBlockWriter) SetJournalTail(int64)
//
//   func (v SuperBlockWriter) AppendJournalRegionSliceItem() JournalRegionWriter
//   func (v SuperBlockWriter) RemoveJournalRegionSliceItemAt(int)
//
//   func (v SuperBlockWriter) SwapJournalReigonItemsAt(i, j int)
//   func (v SuperBlockWriter) SortJournalRegionSliceFunc(cmp func(a, b JournalRegion)int)
//
//   func SuperBlockJournalRegionSliceCapForNumBytes(nbytes int) int

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

var (
	inPkg  = flag.String("inpkg", ".", "package path/name for the type definitions")
	outPkg = flag.String("outpkg", "", "package name for the generated files")
	outDir = flag.String("outdir", "", "output directory for the generated files")
)

// Usage is a replacement usage function for the flags package.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of blockgen:\n")
	fmt.Fprintf(os.Stderr, "\tblockgen -inpkg '...' -outpkg '...' -outdir '...' types... # Must be a single package\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("blockgen: ")

	flag.Usage = Usage
	flag.Parse()
	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	if *outDir == "" {
		log.Fatalf("output directory must be set with -outdir flag")
	}

	types := flag.Args()
	pkg, err := loadPackage(*inPkg)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: Check that outDir cannot be same as inPkg directory.

	if len(*outPkg) == 0 {
		s := filepath.Base(*outDir)
		outPkg = &s
	}

	g, err := newGenerator(pkg, *outPkg)
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range types {
		if err := g.generate(t); err != nil {
			log.Fatal(err)
		}
		// Generate New* and Open* methods only for top-level data types.
		if err := g.generateNewAndOpenMethods(t); err != nil {
			log.Fatal(err)
		}
	}

	for _, typ := range g.GetTypes() {
		// Format the output.
		src := g.GetSource(typ)

		// Write to file.
		outputName := filepath.Join(*outDir, strings.ToLower(typ)+".blockgen.go")
		if err := os.WriteFile(outputName, src, 0644); err != nil {
			log.Fatalf("writing output: %s", err)
		}
	}

	// Create a common.blockgen.go file for common stuff if any.
	src := g.GetSource("")
	outputName := filepath.Join(*outDir, "common.blockgen.go")
	if err := os.WriteFile(outputName, src, 0644); err != nil {
		log.Fatalf("writing common.blockgen.go: %s", err)
	}
}

func loadPackage(pkg string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.LoadTypes | packages.NeedTypesInfo | packages.NeedImports,
	}
	pkgs, err := packages.Load(cfg, pkg)
	if err != nil {
		return nil, err
	}
	return pkgs[0], nil
}

type FieldData struct {
	Index int16
	Name  string
	Kind  string // One of ["int8","int16","int32","int64","uint8","uint16","uint32","uint64","struct","slice","array"]

	Alignment int64
	Offset    int64
	Size      int64

	TypeName    string
	TypePkgPath string
	TypePkgName string

	Length int64 // -1 for slices, 0 for non-arrays and +ve value for arrays

	ElementSize int64
	ElementKind string // One of ["","int8","int16","int32","int64","uint8","uint16","uint32","uint64"] for slices and arrays
}

type StructData struct {
	StructName string
	Size       int64

	FieldList []*FieldData
}

func (sd *StructData) HasSliceField() bool {
	// Slice type field, if present, must be the last field.
	if n := len(sd.FieldList); n > 0 {
		return sd.FieldList[n-1].Length == -1
	}
	return false
}

type Generator struct {
	failedTypes   typeutil.Map // map[*types.Type]error
	checkedTypes  typeutil.Map // map[*types.Type]bool
	checkingTypes typeutil.Map // map[*types.Type]bool

	structsWithSlice map[string]bool

	sizer types.Sizes

	pkg     *packages.Package
	pkgName string

	common bytes.Buffer

	bufferMap map[string]*bytes.Buffer

	// importsMap holds a mapping from a package path name to list of typename
	// keys in the bufferMap that needs to import the package name. For example,
	//
	//   importsMap["github.com/visvasity/bytefield.v2"]["Point"] = "bytefield"
	//
	// entry indicates an import statement like,
	//
	//   import bytefield "github.com/visvasity/bytefield.v2"
	//
	// in the generated file named "point.blockgen.go".
	importsMap map[string]map[string]string

	// aliasPkgMap holds typename to it's source package from where this type
	// should be imported as an alias. For example,
	//
	//   aliasPkgMap["PBA"] == "storage"
	//
	// indicates that we should have the following import
	//
	//   type PBA = storage.PBA
	//
	// in the "common.blockgen.go" generated file. We expect an entry in the
	// importMap for the "storage" package.
	aliasPkgMap map[string]string

	structDataMap map[string]*StructData
}

func newGenerator(pkg *packages.Package, pkgName string) (*Generator, error) {
	g := &Generator{
		pkg:              pkg,
		pkgName:          pkgName,
		aliasPkgMap:      make(map[string]string),
		bufferMap:        make(map[string]*bytes.Buffer),
		structsWithSlice: make(map[string]bool),
		importsMap:       make(map[string]map[string]string),
		sizer:            types.SizesFor(runtime.Compiler, runtime.GOARCH),
		structDataMap:    make(map[string]*StructData),
	}
	return g, nil
}

func (g *Generator) readerName(typeName string) string {
	return typeName
}

func (g *Generator) writerName(typeName string) string {
	return typeName + "Writer"
}

func (g *Generator) getBuffer(typeName string) *bytes.Buffer {
	if len(typeName) == 0 {
		return &g.common
	}
	if b, ok := g.bufferMap[typeName]; ok {
		return b
	}
	b := new(bytes.Buffer)
	g.bufferMap[typeName] = b
	return b
}

func (g *Generator) addImport(typeName string, importName, packagePath string) error {
	vmap, ok := g.importsMap[packagePath]
	if !ok {
		vmap = make(map[string]string)
		g.importsMap[packagePath] = vmap
	}

	x, ok := vmap[typeName]
	if !ok {
		vmap[typeName] = importName
		g.importsMap[packagePath] = vmap
		return nil
	}

	if x != importName {
		return fmt.Errorf("multiple different import names for package %q by type %q", packagePath, typeName)
	}
	return nil
}

func (g *Generator) addAlias(typeName, pkgName, pkgPath string) error {
	g.aliasPkgMap[typeName] = pkgName
	return g.addImport("", pkgName, pkgPath)
}

func (g *Generator) P(typeName string, v ...any) {
	buf := g.getBuffer(typeName)
	for _, x := range v {
		switch x := x.(type) {
		default:
			fmt.Fprint(buf, x)
		}
	}
	fmt.Fprintln(buf)
}

func (g *Generator) GetTypes() []string {
	return slices.Collect(maps.Keys(g.bufferMap))
}

func (g *Generator) GetSource(typeName string) []byte {
	buf := g.getSourceWithImports(typeName)

	src, err := format.Source(buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Printf("warning: internal error: invalid Go generated: %s", err)
		log.Printf("warning: compile the package to analyze the error")
		return buf.Bytes()
	}
	return src
}

var fixedBasicTypeMap = map[types.BasicKind][3]string{
	types.Int8:  {"int8", "Int8At", "SetInt8At"},
	types.Int16: {"int16", "Int16At", "SetInt16At"},
	types.Int32: {"int32", "Int32At", "SetInt32At"},
	types.Int64: {"int64", "Int64At", "SetInt64At"},

	types.Uint8:  {"uint8", "Uint8At", "SetUint8At"},
	types.Uint16: {"uint16", "Uint16At", "SetUint16At"},
	types.Uint32: {"uint32", "Uint32At", "SetUint32At"},
	types.Uint64: {"uint64", "Uint64At", "SetUint64At"},
}

var basicKindMap = map[string]types.BasicKind{
	"int8":   types.Int8,
	"int16":  types.Int16,
	"int32":  types.Int32,
	"int64":  types.Int64,
	"uint8":  types.Uint8,
	"uint16": types.Uint16,
	"uint32": types.Uint32,
	"uint64": types.Uint64,
}

func (g *Generator) checkUnderlyingType(v *types.Var, vtype, utype types.Type) error {
	switch x := utype.(type) {
	default:
		return fmt.Errorf("field (%v) of type %T is not supported", v, vtype)
	case *types.Basic:
		if _, ok := fixedBasicTypeMap[x.Kind()]; !ok {
			return fmt.Errorf("basic field (%v) of type %T is not supported", v, vtype)
		}
	case *types.Array:
		if n := x.Len(); n == 0 {
			return fmt.Errorf("zero sized arrays are not supported")
		}
		if _, ok := x.Elem().(*types.Basic); ok {
			return nil
		}
		if _, ok := x.Elem().Underlying().(*types.Basic); ok {
			return nil
		}
		ntype, ok := x.Elem().(*types.Named)
		if !ok {
			return fmt.Errorf("array element type (%T) is not supported", x.Elem())
		}
		if err := g.checkType(ntype.Obj()); err != nil {
			return err
		}
		if _, ok := g.structsWithSlice[ntype.Obj().Id()]; ok {
			return fmt.Errorf("slices element type (%v) must be of fixed size", x.Elem())
		}
	case *types.Struct:
		if xxx := g.failedTypes.At(x); xxx != nil {
			return fmt.Errorf("underlying struct type (%v) is not supported", x)
		}
		ntype, ok := vtype.(*types.Named)
		if !ok {
			return fmt.Errorf("anonymous/inline struct field types are not supported")
		}
		return g.checkType(ntype.Obj())
	case *types.Slice:
		if _, ok := x.Elem().(*types.Basic); ok {
			return nil
		}
		if _, ok := x.Elem().Underlying().(*types.Basic); ok {
			return nil
		}
		ntype, ok := x.Elem().(*types.Named)
		if !ok {
			return fmt.Errorf("slice element type (%v) is not supported", x.Elem())
		}
		if err := g.checkType(ntype.Obj()); err != nil {
			return err
		}
		if _, ok := g.structsWithSlice[ntype.Obj().Id()]; ok {
			return fmt.Errorf("slices element type (%v) must be of fixed size", x.Elem())
		}
	}
	return nil
}

func (g *Generator) checkType(typeName *types.TypeName) (status error) {
	s, ok := typeName.Type().Underlying().(*types.Struct)
	if !ok {
		return fmt.Errorf("input type (%v) is not a named struct type", typeName)
	}

	if v := g.checkedTypes.At(s); v != nil {
		return nil
	}
	if v := g.failedTypes.At(s); v != nil {
		return fmt.Errorf("struct type is not supported: %w", v.(error))
	}
	if v := g.checkingTypes.At(s); v != nil {
		return fmt.Errorf("struct type (%v) with recursive references is not supported", s)
	}

	g.checkingTypes.Set(s, true)
	defer func() {
		if status == nil {
			g.checkedTypes.Set(s, true)
		} else {
			g.failedTypes.Set(s, status)
		}
		g.checkingTypes.Delete(s)
	}()

	for i := 0; i < s.NumFields(); i++ {
		v := s.Field(i)
		if v.Anonymous() {
			return fmt.Errorf("anonymous fields (%v) are not supported", v)
		}

		vtype := v.Type()
		if _, ok := vtype.(*types.Slice); ok {
			if i != s.NumFields()-1 {
				return fmt.Errorf("slice field (%v) must be the last field of a struct", v)
			}
			g.structsWithSlice[typeName.Id()] = true
		}

		if err := g.checkUnderlyingType(v, vtype, vtype.Underlying()); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) prepareStructData(tname *types.TypeName) (*StructData, error) {
	skey := tname.Pkg().Path() + "." + tname.Name()
	sdata := &StructData{StructName: tname.Name()}

	stype, ok := tname.Type().Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("input type %q is not a struct", tname.Name())
	}

	var fs []*types.Var
	for i := 0; i < stype.NumFields(); i++ {
		f := stype.Field(i)
		fdata := &FieldData{
			Index:     int16(i),
			Name:      f.Name(),
			Alignment: g.sizer.Alignof(f.Type()),
			Size:      g.sizer.Sizeof(f.Type()),
		}

		// Set Kind and Length fields.
		switch x := f.Type().Underlying().(type) {
		case *types.Basic:
			fdata.Kind = fixedBasicTypeMap[x.Kind()][0]
		case *types.Struct:
			fdata.Kind = "struct"
		case *types.Array:
			fdata.Kind = "array"
			fdata.Length = x.Len()
		case *types.Slice:
			fdata.Kind = "slice"
			fdata.Length = -1
		}

		// log.Printf("struct=%s field=%s f.Type()=%T", sdata.StructName, fdata.Name, f.Type())

		// Set TypeName, TypePkgPath, TypePkgName fields.
		switch x := f.Type().(type) {
		case *types.Alias:
			fdata.TypeName = x.Obj().Name()
			fdata.TypePkgPath = x.Obj().Pkg().Path()
			fdata.TypePkgName = x.Obj().Pkg().Name()
		case *types.Named:
			fdata.TypeName = x.Obj().Name()
			fdata.TypePkgPath = x.Obj().Pkg().Path()
			fdata.TypePkgName = x.Obj().Pkg().Name()
		case *types.Array:
			if v, ok := x.Elem().Underlying().(*types.Basic); ok {
				fdata.ElementKind = fixedBasicTypeMap[v.Kind()][0]
				fdata.ElementSize = g.sizer.Sizeof(x.Elem())
			}
			if v, ok := x.Elem().(*types.Named); ok {
				fdata.TypeName = v.Obj().Name()
				fdata.TypePkgPath = v.Obj().Pkg().Path()
				fdata.TypePkgName = v.Obj().Pkg().Name()
				fdata.ElementSize = g.sizer.Sizeof(x.Elem())
			}
		case *types.Slice:
			if v, ok := x.Elem().Underlying().(*types.Basic); ok {
				fdata.ElementKind = fixedBasicTypeMap[v.Kind()][0]
				fdata.ElementSize = g.sizer.Sizeof(x.Elem())
			}
			if v, ok := x.Elem().(*types.Named); ok {
				fdata.TypeName = v.Obj().Name()
				fdata.TypePkgPath = v.Obj().Pkg().Path()
				fdata.TypePkgName = v.Obj().Pkg().Name()
				fdata.ElementSize = g.sizer.Sizeof(x.Elem())
			}
		}

		fs = append(fs, f)
		sdata.FieldList = append(sdata.FieldList, fdata)
	}

	// Set Offset field.
	offsets := g.sizer.Offsetsof(fs)
	for i, offset := range offsets {
		sdata.FieldList[i].Offset = offset
	}

	// Set the StructData.Size field.
	sdata.Size = g.sizer.Sizeof(stype)

	// js, _ := json.MarshalIndent(sdata, "", "  ")
	// log.Printf("%s", js)

	g.structDataMap[skey] = sdata
	return sdata, nil
}

func (g *Generator) getStructData(typeName string) (*StructData, error) {
	scope := g.pkg.Types.Scope()
	object := scope.Lookup(typeName)
	if object == nil {
		return nil, fmt.Errorf("typename %q doesn't exist", typeName)
	}
	tname, ok := object.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("generator type %q is not a typename", typeName)
	}
	key := tname.Pkg().Path() + "." + tname.Name()
	v, ok := g.structDataMap[key]
	if !ok {
		return nil, fmt.Errorf("could not find struct-data for %q", typeName)
	}
	return v, nil
}

func (g *Generator) getStructFieldData(tname *types.TypeName, i int) (*FieldData, error) {
	key := tname.Pkg().Path() + "." + tname.Name()
	return g.structDataMap[key].FieldList[i], nil
}

func (g *Generator) generate(typeName string) error {
	scope := g.pkg.Types.Scope()
	object := scope.Lookup(typeName)
	if object == nil {
		return fmt.Errorf("typename %q doesn't exist", typeName)
	}
	tn, ok := object.(*types.TypeName)
	if !ok {
		return fmt.Errorf("generator type %q is not a typename", typeName)
	}
	if err := g.checkType(tn); err != nil {
		return err
	}
	s := tn.Type().Underlying().(*types.Struct)

	// Generate code for all dependent struct types.
	for i := 0; i < s.NumFields(); i++ {
		v := s.Field(i)
		switch x := v.Type().Underlying().(type) {
		case *types.Slice:
			if ntype, ok := x.Elem().(*types.Named); ok {
				subName := ntype.Obj().Name()
				if _, ok := g.bufferMap[subName]; ok {
					continue
				}
				if _, ok := x.Elem().Underlying().(*types.Struct); ok {
					if err := g.generate(subName); err != nil {
						return err
					}
				}
			}
		case *types.Array:
			if ntype, ok := x.Elem().(*types.Named); ok {
				subName := ntype.Obj().Name()
				if _, ok := g.bufferMap[subName]; ok {
					continue
				}
				if _, ok := x.Elem().Underlying().(*types.Struct); ok {
					if err := g.generate(subName); err != nil {
						return err
					}
				}
			}
		case *types.Struct:
			if ntype, ok := v.Type().(*types.Named); ok {
				subName := ntype.Obj().Name()
				if _, ok := g.bufferMap[subName]; ok {
					continue
				}
				if err := g.generate(subName); err != nil {
					return err
				}
			}
		}
	}

	// Populate the field data map for the struct.
	sdata, err := g.prepareStructData(tn)
	if err != nil {
		return err
	}

	if err := g.generateReaderWriterTypes(sdata.StructName); err != nil {
		return err
	}
	if err := g.generateReaderWriterMethods(sdata.StructName); err != nil {
		return err
	}
	if err := g.generateZeroMethods(sdata); err != nil {
		return err
	}
	if err := g.generatePrintMethods(sdata); err != nil {
		return err
	}
	for i, fdata := range sdata.FieldList {
		switch fdata.Kind {
		case "struct":
			if err := g.generateStructMethods(sdata, i); err != nil {
				return err
			}

		case "array":
			if len(fdata.ElementKind) != 0 {
				if len(fdata.TypeName) == 0 {
					if err := g.generateBasicArrayMethods(sdata, i); err != nil {
						return err
					}
				} else {
					if err := g.generateBasicNamedArrayMethods(sdata, i); err != nil {
						return err
					}
				}
				if err := g.generateAssignArrayMethods(sdata, i); err != nil {
					return err
				}
				if err := g.generateCopyMethods(sdata, i); err != nil {
					return err
				}
			} else {
				if err := g.generateNamedStructArrayMethods(sdata, i); err != nil {
					return err
				}
			}
			if err := g.generateIteratorMethods(sdata, i); err != nil {
				return err
			}
			if err := g.generateSortAndFindMethods(sdata, i); err != nil {
				return err
			}
		case "slice":
			if err := g.generateSliceMethods(sdata, i); err != nil {
				return err
			}
			if len(fdata.ElementKind) != 0 {
				if len(fdata.TypeName) == 0 {
					if err := g.generateBasicSliceMethods(sdata, i); err != nil {
						return err
					}
				} else {
					if err := g.generateBasicNamedSliceMethods(sdata, i); err != nil {
						return err
					}
				}
				if err := g.generateCopyMethods(sdata, i); err != nil {
					return err
				}
			} else {
				if err := g.generateNamedStructSliceMethods(sdata, i); err != nil {
					return err
				}
				if err := g.generateCoalesceMethod(sdata, i); err != nil {
					return err
				}
			}
			if err := g.generateSliceRemoveMethod(sdata, i); err != nil {
				return err
			}
			if err := g.generateIteratorMethods(sdata, i); err != nil {
				return err
			}
			if err := g.generateSortAndFindMethods(sdata, i); err != nil {
				return err
			}
		default: // Basic intX or uintX
			if len(fdata.TypeName) == 0 {
				if err := g.generateBasicMethods(sdata, i); err != nil {
					return err
				}
			} else {
				if err := g.generateBasicNamedMethods(sdata, i); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (g *Generator) getImports(typeName string) [][2]string {
	var imports [][2]string
	for pkgPath, vmap := range g.importsMap {
		imp, ok := vmap[typeName]
		if !ok {
			continue
		}
		imports = append(imports, [2]string{imp, pkgPath})
	}
	return imports
}

func (g *Generator) getSourceWithImports(typeName string) *bytes.Buffer {
	buf := new(bytes.Buffer)

	fmt.Fprintln(buf, "// Code generated by github.com/visvasity/blockgen. DO NOT EDIT.")
	fmt.Fprintln(buf)
	fmt.Fprintln(buf, "package", g.pkgName)
	fmt.Fprintln(buf)

	imports := g.getImports(typeName)
	if len(imports) != 0 {
		fmt.Fprintln(buf, "import (")
		for _, imp := range imports {
			if len(imp[0]) == 0 {
				fmt.Fprintf(buf, "%q\n", imp[1])
			} else {
				fmt.Fprintf(buf, "%s %q\n", imp[0], imp[1])
			}
		}
		fmt.Fprintln(buf, ")")
	}
	fmt.Fprintln(buf)

	if len(typeName) == 0 {
		g.generateCommonTypeAliases()
	}

	io.Copy(buf, g.getBuffer(typeName))
	return buf
}

func (g *Generator) generateReaderWriterTypes(typeName string) error {
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addImport(typeName, "", "github.com/visvasity/blockgen/common"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "// Reader type defines accessor methods for read-only access.")
	g.P(typeName, "type ", readerTypeName, " common.BlockBytes")
	g.P(typeName)
	g.P(typeName, "// Writer type extends the reader with mutable methods.")
	g.P(typeName, "type ", writerTypeName, " struct { ", readerTypeName, " }")
	g.P(typeName)

	return nil
}

func (g *Generator) generateReaderWriterMethods(typeName string) error {
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	g.P(typeName)
	g.P(typeName, "// BlockBytes returns access to the underlying byte slice.")
	g.P(typeName, "func (v ", readerTypeName, ") BlockBytes() common.BlockBytes {")
	g.P(typeName, "  return common.BlockBytes(v)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "// Writer returns the ", typeName, " writer for read-write access to it's fields.")
	g.P(typeName, "func (v ", readerTypeName, ") Writer() ", writerTypeName, " {")
	g.P(typeName, "  return ", writerTypeName, "{v}")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "// Reader returns the ", typeName, " reader with read-only access to it's fields.")
	g.P(typeName, "func (v ", writerTypeName, ") Reader() ", readerTypeName, " {")
	g.P(typeName, "  return v.", readerTypeName)
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateZeroMethods(sdata *StructData) error {
	typeName := sdata.StructName
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") IsZero() bool {")
	g.P(typeName, "  return common.IsZero(v[:", sdata.Size, "])")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") SetZero() {")
	g.P(typeName, "  common.SetZero(v.BlockBytes()[:", sdata.Size, "])")
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}

func (g *Generator) generateNewAndOpenMethods(typeName string) error {
	sdata, err := g.getStructData(typeName)
	if err != nil {
		return err
	}
	readerTypeName := g.readerName(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	if sdata.HasSliceField() {
		fdata := sdata.FieldList[len(sdata.FieldList)-1]
		g.P(typeName)
		g.P(typeName, "func ", readerTypeName, fdata.Name, "CapForNumBytes(nbytes int) int {")
		g.P(typeName, "  return (nbytes - ", sdata.Size, ") /", fdata.ElementSize)
		g.P(typeName, "}")
		g.P(typeName)
	}

	g.P(typeName)
	g.P(typeName, "// New", readerTypeName, " creates a zero-initialized ", typeName, ". Returns nil if input block size is too small.")
	g.P(typeName, "func New", readerTypeName, "(block []byte) ", readerTypeName, " {")
	g.P(typeName, "  size := len(block)")
	g.P(typeName, "  if size < ", sdata.Size, " {")
	g.P(typeName, "    return nil")
	g.P(typeName, "  }")
	g.P(typeName, "  common.SetZero(block)")
	g.P(typeName, "  v := ", readerTypeName, "(block)")
	if sdata.HasSliceField() {
		fdata := sdata.FieldList[len(sdata.FieldList)-1]
		g.P(typeName, "  // ", readerTypeName, " type has a slice field; we must set a cap on it.")
		g.P(typeName, "  n := (size - ", sdata.Size, ") / ", fdata.ElementSize)
		g.P(typeName, "  v.Writer().internalSet", fdata.Name, "Cap(n)")
	}
	g.P(typeName, "  return v")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func Open", readerTypeName, "(block []byte) (", readerTypeName, ", error) {")
	g.P(typeName, "  size := len(block)")
	g.P(typeName, "  if size < ", sdata.Size, " {")
	g.P(typeName, `    return nil, fmt.Errorf("input size is too small")`)
	g.P(typeName, "  }")
	g.P(typeName, "  v := ", readerTypeName, "(block)")
	if sdata.HasSliceField() {
		fdata := sdata.FieldList[len(sdata.FieldList)-1]
		g.P(typeName, "  // ", readerTypeName, " type has a slice field; validate it's len and cap.")
		g.P(typeName, "  n := (size - ", sdata.Size, ") / ", fdata.ElementSize)
		g.P(typeName, "  if x := v.", fdata.Name, "Cap(); x != n {")
		g.P(typeName, `    return nil, fmt.Errorf("slice field cap must be %d, found %d", n, x)`)
		g.P(typeName, "  }")
		g.P(typeName, "  if x := v.", fdata.Name, "Len(); x < 0 || x > n {")
		g.P(typeName, `    return nil, fmt.Errorf("slice field len is %d, must be between [%d-%d)", x, 0, n)`)
		g.P(typeName, "  }")
	}
	g.P(typeName, "  return v, nil")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateBasicMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	fmethods := fixedBasicTypeMap[basicKindMap[fdata.Kind]]

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "() ", fdata.Kind, " {")
	g.P(typeName, "  return v.BlockBytes().", fmethods[1], "(", fdata.Offset, ")")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Set", fdata.Name, "(x ", fdata.Kind, ") {")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", fdata.Offset, ", x)")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateBasicNamedMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addAlias(fdata.TypeName, fdata.TypePkgName, fdata.TypePkgPath); err != nil {
		return err
	}

	fmethods := fixedBasicTypeMap[basicKindMap[fdata.Kind]]

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "() ", fdata.TypeName, " {")
	g.P(typeName, "  return ", fdata.TypeName, "(v.BlockBytes().", fmethods[1], "(", fdata.Offset, "))")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Set", fdata.Name, "(x ", fdata.TypeName, ") {")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", fdata.Offset, ", ", fdata.Kind, "(x))")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateBasicArrayMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	fmethods := fixedBasicTypeMap[basicKindMap[fdata.ElementKind]]

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "Len() int {")
	g.P(typeName, "  return int(", fdata.Length, ")")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "ItemAt(i int) ", fdata.ElementKind, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.Name, `Len()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  return v.BlockBytes().", fmethods[1], "(", fdata.Offset, "+ i * ", fdata.ElementSize, ")")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Set", fdata.Name, "ItemAt(i int, x ", fdata.ElementKind, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.Name, `Len()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", fdata.Offset, "+ i * ", fdata.ElementSize, ", x)")
	g.P(typeName, "}")
	g.P(typeName)

	// TODO: We can add CopyTo and CopyFrom methods as well because these are the builtin types.

	return nil
}

func (g *Generator) generateBasicNamedArrayMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}
	if err := g.addAlias(fdata.TypeName, fdata.TypePkgName, fdata.TypePkgPath); err != nil {
		return err
	}

	fmethods := fixedBasicTypeMap[basicKindMap[fdata.ElementKind]]

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "Len() int {")
	g.P(typeName, "  return int(", fdata.Length, ")")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "ItemAt(i int) ", fdata.TypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.Name, `Len()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  return ", fdata.TypeName, "(v.BlockBytes().", fmethods[1], "(", fdata.Offset, "+ i * ", fdata.ElementSize, "))")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Set", fdata.Name, "ItemAt(i int, x ", fdata.TypeName, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.Name, `Len()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", fdata.Offset, "+ i * ", fdata.ElementSize, ", ", fmethods[0], "(x))")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateAssignArrayMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	elemType := fdata.TypeName
	if len(elemType) == 0 {
		elemType = fdata.ElementKind
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "() (xs [", fdata.Length, "]", elemType, ") {")
	g.P(typeName, "  for i := range xs {")
	g.P(typeName, "    xs[i] = v.", fdata.Name, "ItemAt(i)")
	g.P(typeName, "  }")
	g.P(typeName, "  return")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Set", fdata.Name, "(xs [", fdata.Length, "]", elemType, ") {")
	g.P(typeName, "  for i := range xs {")
	g.P(typeName, "    v.Set", fdata.Name, "ItemAt(i, xs[i])")
	g.P(typeName, "  }")
	g.P(typeName, "  return")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateNamedStructArrayMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "Len() int {")
	g.P(typeName, "  return int(", fdata.Length, ")")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "ItemAt(i int) ", fdata.TypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.Name, `Len()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  return ", fdata.TypeName, "([", fdata.Offset, "+ i * ", fdata.ElementSize, ":])")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateStructMethods(sdata *StructData, findex int) error {
	typeName := sdata.StructName
	fdata := sdata.FieldList[findex]

	readerTypeName := g.readerName(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "() ", fdata.TypeName, " {")
	g.P(typeName, "  return ", fdata.TypeName, "(v.BlockBytes()[", fdata.Offset, ":])")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateSliceMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	// A slice holds 3 WORDs [pointer,len,cap]. We use the two words: len and
	// cap. WORD size is different based on the Compiler and GOARCH values, so we
	// must pick methods appropriately.
	wordsize := fdata.Size / 3

	var xmethods [3]string
	switch wordsize {
	case 8:
		xmethods = [3]string{"int64", "Int64At", "SetInt64At"}
	case 4:
		xmethods = [3]string{"int32", "Int32At", "SetInt32At"}
	case 2:
		xmethods = [3]string{"int16", "Int16At", "SetInt16At"}
	case 1:
		xmethods = [3]string{"int8", "Int8At", "SetInt8At"}
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "Len() int {")
	g.P(typeName, "  return int(v.BlockBytes().", xmethods[1], "(", fdata.Offset+1*wordsize, "))")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "Cap() int {")
	g.P(typeName, "  return int(v.BlockBytes().", xmethods[1], "(", fdata.Offset+2*wordsize, "))")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") internalSet", fdata.Name, "Len(x int) {")
	g.P(typeName, "  v.BlockBytes().", xmethods[2], "(", fdata.Offset+1*wordsize, ", ", xmethods[0], "(x))")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") internalSet", fdata.Name, "Cap(x int) {")
	g.P(typeName, "  v.BlockBytes().", xmethods[2], "(", fdata.Offset+2*wordsize, ", ", xmethods[0], "(x))")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateBasicSliceMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	fmethods := fixedBasicTypeMap[basicKindMap[fdata.ElementKind]]

	base := fdata.Offset + fdata.Size

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "ItemAt(i int) ", fdata.ElementKind, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  return v.BlockBytes().", fmethods[1], "(", base, "+ i * ", fdata.ElementSize, ")")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Set", fdata.Name, "ItemAt(i int, x ", fdata.ElementKind, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", base, "+ i * ", fdata.ElementSize, ", x)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Append", fdata.Name, "(x ", fdata.ElementKind, ") {")
	g.P(typeName, "  n := v.", fdata.Name, "Len()")
	g.P(typeName, "  if n == v.", fdata.Name, "Cap() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice is already full with %d items", n))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.internalSet", fdata.Name, "Len(n+1)")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", base, "+ n * ", fdata.ElementSize, ", x)")
	g.P(typeName, "}")
	g.P(typeName)

	// TODO: We can add CopyTo and CopyFrom methods as well because these are the builtin types.

	return nil
}

func (g *Generator) generateBasicNamedSliceMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}
	if err := g.addAlias(fdata.TypeName, fdata.TypePkgName, fdata.TypePkgPath); err != nil {
		return err
	}

	base := fdata.Offset + fdata.Size
	fmethods := fixedBasicTypeMap[basicKindMap[fdata.ElementKind]]

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "ItemAt(i int) ", fdata.TypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  return ", fdata.TypeName, "(v.BlockBytes().", fmethods[1], "(", base, "+ i * ", fdata.ElementSize, "))")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Set", fdata.Name, "ItemAt(i int, x ", fdata.TypeName, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", base, "+ i * ", fdata.ElementSize, ", ", fdata.ElementKind, "(x))")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Append", fdata.Name, "(x ", fdata.TypeName, ") {")
	g.P(typeName, "  n := v.", fdata.Name, "Len()")
	g.P(typeName, "  if n == v.", fdata.Name, "Cap() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice is already full with %d items", n))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.internalSet", fdata.Name, "Len(n+1)")
	g.P(typeName, "  v.BlockBytes().", fmethods[2], "(", base, "+ n * ", fdata.ElementSize, ", ", fdata.ElementKind, "(x))")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateNamedStructSliceMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	base := fdata.Offset + fdata.Size

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") ", fdata.Name, "ItemAt(i int) ", fdata.TypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.Name, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  return ", fdata.TypeName, "(v[", base, " + i * ", fdata.ElementSize, ":])")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Append", fdata.Name, "() ", g.writerName(fdata.TypeName), " {")
	g.P(typeName, "  n := v.", fdata.Name, "Len()")
	g.P(typeName, "  if n == v.", fdata.Name, "Cap() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice is already full with %d items", n))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.internalSet", fdata.Name, "Len(n+1)")
	g.P(typeName, "  return v.", fdata.Name, "ItemAt(n).Writer()")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateSliceRemoveMethod(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	writerTypeName := g.writerName(typeName)

	elementType := fdata.TypeName
	if elementType == "" {
		elementType = fdata.ElementKind
	}

	base := fdata.Offset
	if fdata.Length < 0 {
		base += fdata.Size
	}

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Remove", fdata.Name, "ItemAt(i int) {")
	g.P(typeName, "  n := v.", fdata.Name, "Len()")
	g.P(typeName, "  if i < 0 || i >= n {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  beg := ", base, "+i*", fdata.ElementSize)
	g.P(typeName, "  end := ", base, "+n*", fdata.ElementSize)
	g.P(typeName, "  copy(v.BlockBytes()[beg:], v.BlockBytes()[beg+", fdata.ElementSize, ":end])")
	g.P(typeName, "  common.SetZero(v.BlockBytes()[end-", fdata.ElementSize, ":end])")
	g.P(typeName, "  v.internalSet", fdata.Name, "Len(n-1)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Delete", fdata.Name, "Items(i, j int) {")
	g.P(typeName, "  n := v.", fdata.Name, "Len()")
	g.P(typeName, "  if i < 0 || i >= n {")
	g.P(typeName, `    panic(fmt.Sprintf("first slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  if j < 0 || j >= n {")
	g.P(typeName, `    panic(fmt.Sprintf("second slice index %d is out of range [0:%d:%d]", i, v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  if j < i {")
	g.P(typeName, `    panic(fmt.Sprintf("invalid slice indices %d < %d", j, i))`)
	g.P(typeName, "  }")
	g.P(typeName, "  if i == j {")
	g.P(typeName, "    return")
	g.P(typeName, "  }")
	g.P(typeName, "  ioff := ", base, "+i*", fdata.ElementSize)
	g.P(typeName, "  joff := ", base, "+j*", fdata.ElementSize)
	g.P(typeName, "  end := ", base, "+n*", fdata.ElementSize)
	g.P(typeName, "  copy(v.BlockBytes()[ioff:end], v.BlockBytes()[joff:end])")
	g.P(typeName, "  common.SetZero(v.BlockBytes()[end-(joff-ioff):end])")
	g.P(typeName, "  v.internalSet", fdata.Name, "Len(n-(j-i))")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateIteratorMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)

	if err := g.addImport(typeName, "", "iter"); err != nil {
		return err
	}

	elementType := fdata.TypeName
	if elementType == "" {
		elementType = fdata.ElementKind
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") All", fdata.Name, "() iter.Seq2[int,", elementType, "] {")
	g.P(typeName, "  return func(yield func(int, ", elementType, ") bool) {")
	g.P(typeName, "    for i := 0; i < v.", fdata.Name, "Len(); i++ {")
	g.P(typeName, "      if !yield(i, v.", fdata.Name, "ItemAt(i)) {")
	g.P(typeName, "        return")
	g.P(typeName, "      }")
	g.P(typeName, "    }")
	g.P(typeName, "  }")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateCommonTypeAliases() {
	keys := slices.Collect(maps.Keys(g.aliasPkgMap))
	slices.Sort(keys)

	g.P("")
	g.P("", "type (")
	for _, k := range keys {
		v := g.aliasPkgMap[k]
		g.P("", "  ", k, " = ", v, ".", k)
	}
	g.P("", ")")
	g.P("")
}

func (g *Generator) generateCoalesceMethod(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	writerTypeName := g.writerName(typeName)

	elementType := fdata.TypeName
	if elementType == "" {
		elementType = fdata.ElementKind
	}

	base := fdata.Offset
	if fdata.Length < 0 {
		base += fdata.Size
	}

	freader := g.readerName(elementType)
	fwriter := g.writerName(elementType)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Coalesce", fdata.Name, "Func(merge func(w ", fwriter, ", r ", freader, ") bool) int {")
	g.P(typeName, "  nmerges := 0")
	g.P(typeName, "  for i := 0; i < v.", fdata.Name, "Len()-1; {")
	g.P(typeName, "    if merge(v.", fdata.Name, "ItemAt(i).Writer(), v.", fdata.Name, "ItemAt(i+1)) {")
	g.P(typeName, "      v.Remove", fdata.Name, "ItemAt(i + 1)")
	g.P(typeName, "      nmerges++")
	g.P(typeName, "      continue")
	g.P(typeName, "    }")
	g.P(typeName, "    i++")
	g.P(typeName, "  }")
	g.P(typeName, "  return nmerges")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateSortAndFindMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	if err := g.addImport(typeName, "", "sort"); err != nil {
		return err
	}

	elementType := fdata.TypeName
	if elementType == "" {
		elementType = fdata.ElementKind
	}

	base := fdata.Offset
	if fdata.Length < 0 {
		base += fdata.Size
	}

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Swap", fdata.Name, "Items(i, j int) {")
	g.P(typeName, "  tmp := make([]byte, ", fdata.ElementSize, ")")
	g.P(typeName, "  ioff := ", base, "+ i * ", fdata.ElementSize)
	g.P(typeName, "  joff := ", base, "+ j * ", fdata.ElementSize)
	g.P(typeName, "  copy(tmp, v.BlockBytes()[ioff:ioff+", fdata.ElementSize, "])")
	g.P(typeName, "  copy(v.BlockBytes()[ioff:ioff+", fdata.ElementSize, "], v.BlockBytes()[joff:joff+", fdata.ElementSize, "])")
	g.P(typeName, "  copy(v.BlockBytes()[joff:joff+", fdata.ElementSize, "], tmp)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Sort", fdata.Name, "Func(cmp func(a, b ", elementType, ") int) {")
	g.P(typeName, "  helper := common.SortHelper{")
	g.P(typeName, "    LenFunc: v.", fdata.Name, "Len,")
	g.P(typeName, "    SwapFunc: v.Swap", fdata.Name, "Items,")
	g.P(typeName, "    CompareFunc: func(i,j int)int{return cmp(v.", fdata.Name, "ItemAt(i), v.", fdata.Name, "ItemAt(j))},")
	g.P(typeName, "  }")
	g.P(typeName, "  sort.Sort(&helper)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") Find", fdata.Name, "Func(cmp func(x ", elementType, ") int) (int, bool) {")
	g.P(typeName, "  return sort.Find(v.", fdata.Name, "Len(), func(i int) int { return cmp(v.", fdata.Name, "ItemAt(i)) })")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateCopyMethods(sdata *StructData, findex int) error {
	typeName, fdata := sdata.StructName, sdata.FieldList[findex]
	readerTypeName := g.readerName(typeName)
	writerTypeName := g.writerName(typeName)

	elementType := fdata.TypeName
	if elementType == "" {
		elementType = fdata.ElementKind
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") Copy", fdata.Name, "To(xs []", elementType, ") {")
	g.P(typeName, "  n := min(len(xs), v.", fdata.Name, "Len())")
	g.P(typeName, "  for i := 0; i < n; i++ {")
	g.P(typeName, "    xs[i] = v.", fdata.Name, "ItemAt(i)")
	g.P(typeName, "  }")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerTypeName, ") Copy", fdata.Name, "From(xs []", elementType, ") {")
	g.P(typeName, "  n := min(len(xs), v.", fdata.Name, "Len())")
	g.P(typeName, "  for i := 0; i < n; i++ {")
	g.P(typeName, "    v.Set", fdata.Name, "ItemAt(i, xs[i])")
	g.P(typeName, "  }")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generatePrintMethods(sdata *StructData) error {
	typeName := sdata.StructName
	readerTypeName := g.readerName(typeName)

	if err := g.addImport(typeName, "", "strings"); err != nil {
		return err
	}
	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerTypeName, ") String() string {")
	g.P(typeName, "  var sb strings.Builder")
	for i, fdata := range sdata.FieldList {
		if i != 0 {
			g.P(typeName, `  fmt.Fprintf(&sb, " ")`)
		}

		switch fdata.Kind {
		case "array":
			if fdata.ElementKind == "uint8" {
				g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.Name, `=[%d]{%x}", v.`, fdata.Name, `Len(), v.`, fdata.Name, `())`)
				continue
			}
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.Name, `=[%d]{", v.`, fdata.Name, `Len())`)
			g.P(typeName, `  for i := 0; i < v.`, fdata.Name, `Len(); i++ {`)
			g.P(typeName, `    if i == 0 {`)
			if fdata.ElementKind != "" {
				g.P(typeName, `      fmt.Fprintf(&sb, "%d", v.`, fdata.Name, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, "{%v}", v.`, fdata.Name, `ItemAt(i))`)
			}
			g.P(typeName, `    } else {`)
			if fdata.ElementKind != "" {
				g.P(typeName, `      fmt.Fprintf(&sb, " %d", v.`, fdata.Name, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, " {%v}", v.`, fdata.Name, `ItemAt(i))`)
			}
			g.P(typeName, `    }`)
			g.P(typeName, `  }`)
			g.P(typeName, `  fmt.Fprintf(&sb, "}")`)
		case "slice":
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.Name, `=[:%d:%d]{", v.`, fdata.Name, `Len(), v.`, fdata.Name, `Cap())`)
			g.P(typeName, `  for i := 0; i < v.`, fdata.Name, `Len(); i++ {`)
			g.P(typeName, `    if i == 0 {`)
			if fdata.ElementKind != "" {
				g.P(typeName, `      fmt.Fprintf(&sb, "%d", v.`, fdata.Name, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, "{%v}", v.`, fdata.Name, `ItemAt(i))`)
			}
			g.P(typeName, `    } else {`)
			if fdata.ElementKind != "" {
				g.P(typeName, `      fmt.Fprintf(&sb, " %d", v.`, fdata.Name, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, " {%v}", v.`, fdata.Name, `ItemAt(i))`)
			}
			g.P(typeName, `    }`)
			g.P(typeName, `  }`)
			g.P(typeName, `  fmt.Fprintf(&sb, "}")`)
		case "struct":
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.Name, `={%v}", v.`, fdata.Name, `())`)
		default:
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.Name, `=%d", v.`, fdata.Name, `())`)
		}
	}
	g.P(typeName, "  return sb.String()")
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}
