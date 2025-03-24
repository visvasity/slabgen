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
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/visvasity/blockgen/typecheck"
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
		if err := g.AddType(t); err != nil {
			log.Fatal(err)
		}
	}
	if err := g.generate(); err != nil {
		log.Fatal(err)
	}
	for _, t := range types {
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

type Generator struct {
	failedTypes   typeutil.Map // map[*types.Type]error
	checkedTypes  typeutil.Map // map[*types.Type]*StructData
	checkingTypes typeutil.Map // map[*types.Type]bool

	structsWithSlice map[string]bool

	sizer types.Sizes

	pkg     *packages.Package
	pkgName string

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

	checker *typecheck.Checker

	structDataMap map[string]*StructData
}

type StructData struct {
	*typecheck.StructData

	ReaderWriterTypeParams []*typecheck.TypeParamData
}

func newGenerator(pkg *packages.Package, pkgName string) (*Generator, error) {
	g := &Generator{
		checker:       typecheck.New(),
		pkg:           pkg,
		pkgName:       pkgName,
		bufferMap:     make(map[string]*bytes.Buffer),
		importsMap:    make(map[string]map[string]string),
		structDataMap: make(map[string]*StructData),
	}
	return g, nil
}

func (g *Generator) getBuffer(typeName string) *bytes.Buffer {
	if len(typeName) == 0 {
		return nil
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

// var fixedBasicTypeMap = map[types.BasicKind][3]string{
// 	types.Int8:  {"int8", "Int8At", "SetInt8At"},
// 	types.Int16: {"int16", "Int16At", "SetInt16At"},
// 	types.Int32: {"int32", "Int32At", "SetInt32At"},
// 	types.Int64: {"int64", "Int64At", "SetInt64At"},

// 	types.Uint8:  {"uint8", "Uint8At", "SetUint8At"},
// 	types.Uint16: {"uint16", "Uint16At", "SetUint16At"},
// 	types.Uint32: {"uint32", "Uint32At", "SetUint32At"},
// 	types.Uint64: {"uint64", "Uint64At", "SetUint64At"},
// }

var basicMethodTypeMap = map[string][3]string{
	"int8":  {"int8", "Int8At", "SetInt8At"},
	"int16": {"int16", "Int16At", "SetInt16At"},
	"int32": {"int32", "Int32At", "SetInt32At"},
	"int64": {"int64", "Int64At", "SetInt64At"},

	"uint8":  {"uint8", "Uint8At", "SetUint8At"},
	"uint16": {"uint16", "Uint16At", "SetUint16At"},
	"uint32": {"uint32", "Uint32At", "SetUint32At"},
	"uint64": {"uint64", "Uint64At", "SetUint64At"},

	"byte": {"uint8", "Uint8At", "SetUint8At"},
}

// var intMethodTypeMap = map[int][3]string{
// 	1: {"int8", "Int8At", "SetInt8At"},
// 	2: {"int16", "Int16At", "SetInt16At"},
// 	4: {"int32", "Int32At", "SetInt32At"},
// 	8: {"int64", "Int64At", "SetInt64At"},
// }

// var basicKindMap = map[string]types.BasicKind{
// 	"int8":   types.Int8,
// 	"int16":  types.Int16,
// 	"int32":  types.Int32,
// 	"int64":  types.Int64,
// 	"uint8":  types.Uint8,
// 	"uint16": types.Uint16,
// 	"uint32": types.Uint32,
// 	"uint64": types.Uint64,
// }

// // func (g *Generator) getStructFieldData(tname *types.TypeName, i int) (*typecheck.FieldData, error) {
// // 	key := tname.Pkg().Path() + "." + tname.Name()
// // 	return g.structDataMap[key].FieldList[i], nil
// // }

// var knownTypesMap = map[string]types.Type{
// 	"int8":    types.Typ[types.Int8],
// 	"int16":   types.Typ[types.Int16],
// 	"int32":   types.Typ[types.Int32],
// 	"int64":   types.Typ[types.Int64],
// 	"uint8":   types.Typ[types.Uint8],
// 	"uint16":  types.Typ[types.Uint16],
// 	"uint32":  types.Typ[types.Uint32],
// 	"uint64":  types.Typ[types.Uint64],
// 	"float32": types.Typ[types.Float32],
// 	"float64": types.Typ[types.Float64],
// }

// func (g *Generator) instantiateType(typeArg string) (string, *types.Named, error) {
// 	scope := g.pkg.Types.Scope()
// 	if !strings.ContainsRune(typeArg, '=') {
// 		object := scope.Lookup(typeArg)
// 		if object == nil {
// 			return "", nil, fmt.Errorf("could not find type for type name arg %q", typeArg)
// 		}
// 		named, ok := object.Type().(*types.Named)
// 		if !ok {
// 			return "", nil, fmt.Errorf("type name arg %q is not a named type", typeArg)
// 		}
// 		return object.Name(), named, nil
// 	}

// 	typeName, a, _ := strings.Cut(typeArg, "=")
// 	genericName, b, ok := strings.Cut(a, "[")
// 	if !ok {
// 		return "", nil, fmt.Errorf("could not find [ character in generic type arg %q", typeArg)
// 	}
// 	c, _, ok := strings.Cut(b, "]")
// 	if !ok {
// 		return "", nil, fmt.Errorf("could not find ] character in generic type arg %q", typeArg)
// 	}
// 	typeArgs := strings.Split(c, ",")

// 	origObj := scope.Lookup(genericName)
// 	if origObj == nil {
// 		return "", nil, fmt.Errorf("could not find generic type name %q", genericName)
// 	}
// 	orig, ok := origObj.(*types.TypeName)
// 	if !ok {
// 		return "", nil, fmt.Errorf("generic type name %q is not of a *types.TypeName type (%T)", genericName, origObj)
// 	}

// 	var targs []types.Type
// 	for _, arg := range typeArgs {
// 		if v, ok := knownTypesMap[arg]; ok {
// 			targs = append(targs, v)
// 			continue
// 		}
// 		obj := scope.Lookup(arg)
// 		if obj == nil {
// 			return "", nil, fmt.Errorf("could not find generic type argment %q in %q", arg, typeName)
// 		}
// 		targs = append(targs, obj.Type())
// 	}

// 	ftype, err := types.Instantiate(nil, orig.Type(), targs, true /* validate */)
// 	if err != nil {
// 		return "", nil, fmt.Errorf("could not instantiate generic type %q: %w", typeName, err)
// 	}
// 	named, ok := ftype.(*types.Named)
// 	if !ok {
// 		return "", nil, fmt.Errorf("type name arg %q is not a named type", typeArg)
// 	}
// 	knownTypesMap[typeName] = named
// 	return typeName, named, nil
// }

func clone(sdata *typecheck.StructData) *typecheck.StructData {
	js, _ := json.Marshal(sdata)
	v := new(typecheck.StructData)
	if err := json.Unmarshal(js, v); err != nil {
		panic(err)
	}
	return v
}

func (g *Generator) prepareOutputStructDataMap() {
	for sname, sdata := range g.checker.StructDataMap() {
		if _, ok := g.structDataMap[sname]; ok {
			continue
		}

		v := &StructData{StructData: clone(sdata)}
		for _, fdata := range v.StructData.Fields {
			if fdata.TypeParams != nil {

			}
		}

		for _, tp := range sdata.TypeParams {
			if tp.Constraint == "Struct" && tp.PkgPath == "github.com/visvasity/blockgen/blockgen" {
				v.ReaderWriterTypeParams = append(v.ReaderWriterTypeParams,
					&typecheck.TypeParamData{
						Name:       tp.Name + "Reader",
						Constraint: "Reader[" + tp.Name + "," + tp.Name + "Writer]",
						PkgPath:    "github.com/visvasity/blockgen/blockgen",
						PkgName:    "blockgen",
					},
					&typecheck.TypeParamData{
						Name:       tp.Name + "Writer",
						Constraint: "Writer[" + tp.Name + "," + tp.Name + "Reader]",
						PkgPath:    "github.com/visvasity/blockgen/blockgen",
						PkgName:    "blockgen",
					},
				)
			}
		}
		g.structDataMap[sname] = v
	}
}

func (g *Generator) AddType(name string) error {
	scope := g.pkg.Types.Scope()
	object := scope.Lookup(name)
	if object == nil {
		return fmt.Errorf("could not find type for type name arg %q", name)
	}
	named, ok := object.Type().(*types.Named)
	if !ok {
		return fmt.Errorf("type name arg %q is not a named type", name)
	}

	if err := g.checker.Check(named.Obj()); err != nil {
		return err
	}
	g.prepareOutputStructDataMap()
	return nil
}

func (g *Generator) generate() error {
	for sname, sdata := range g.structDataMap {
		_ = sname
		// js, _ := json.MarshalIndent(sdata, "", "  ")
		// log.Printf("%s=%s", sname, js)

		if err := g.generateReaderWriterTypes(sdata); err != nil {
			return err
		}
		if err := g.generateReaderWriterMethods(sdata); err != nil {
			return err
		}
		if err := g.generateZeroMethods(sdata); err != nil {
			return err
		}
		if err := g.generatePrintMethods(sdata); err != nil {
			return err
		}
		if err := g.generateCopyMethods(sdata); err != nil {
			return err
		}
		for _, fdata := range sdata.Fields {
			switch fdata.Kind {
			case "basic":
				if err := g.generateNumberFieldMethods(sdata, fdata); err != nil {
					return err
				}
			case "struct":
				if err := g.generateStructFieldMethods(sdata, fdata); err != nil {
					return err
				}
			case "slice":
				if err := g.generateSliceMethods(sdata); err != nil {
					return err
				}
				if fdata.SliceKind == "basic" {
					if err := g.generateSliceOfNumbersFieldMethods(sdata, fdata); err != nil {
						return err
					}
				}
				if fdata.SliceKind == "struct" {
					if err := g.generateSliceOfStructsFieldMethods(sdata, fdata); err != nil {
						return err
					}
				}
			case "array":
				if fdata.ArrayKind == "basic" {
					if err := g.generateArrayOfNumbersFieldMethods(sdata, fdata); err != nil {
						return err
					}
				}
				if fdata.ArrayKind == "struct" {
					if err := g.generateArrayOfStructsFieldMethods(sdata, fdata); err != nil {
						return err
					}
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
	if len(typeName) == 0 {
		return nil
	}

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

	io.Copy(buf, g.getBuffer(typeName))
	return buf
}

func (g *Generator) IsGenericStruct(sdata *StructData) bool {
	return len(sdata.TypeParams) != 0
}

func (g *Generator) IsGenericField(sdata *StructData, fdata *typecheck.FieldData) bool {
	return len(fdata.TypeArgs) != 0 || len(fdata.TypeParams) != 0
}

func (g *Generator) IsGenericTypeParamField(sdata *StructData, fdata *typecheck.FieldData) bool {
	if !g.IsGenericField(sdata, fdata) {
		return false
	}
	for _, tp := range sdata.TypeParams {
		if tp.Name == fdata.TypeName {
			return true
		}
	}
	return false
}

type structTypeNameOptions struct {
	PkgName                       bool
	TypeArgs                      bool
	TypeArgPkgNames               bool
	TypeParams                    bool // Overrides TypeArgs
	TypeConstraints               bool // Overrides TypeParams and TypeArgs
	TypeConstraintPkgNames        bool
	IncludeReaderWriterTypeParams bool
}

type fieldTypeNameOptions struct {
	PkgName                     bool
	TypeArgs                    bool
	TypeArgPkgNames             bool
	IncludeReaderWriterTypeArgs bool
}

var (
	inputStructNameOpts = structTypeNameOptions{PkgName: true, TypeParams: true}
	receiverNameOptions = structTypeNameOptions{TypeParams: true, IncludeReaderWriterTypeParams: true}
)

func (g *Generator) prepStructTypeName(sdata *StructData, prefix, suffix string, opts *structTypeNameOptions) string {
	if opts == nil {
		opts = &structTypeNameOptions{}
	}
	name := prefix + sdata.StructName + suffix
	if opts.PkgName && len(sdata.PkgName) != 0 {
		name = sdata.PkgName + "." + name
	}

	if opts.TypeConstraints && len(sdata.TypeParams) != 0 {
		tps := sdata.TypeParams
		if opts.IncludeReaderWriterTypeParams {
			tps = append(tps, sdata.ReaderWriterTypeParams...)
		}
		vs := make([]string, len(tps))
		for i, tp := range tps {
			cname := tp.Constraint
			if opts.TypeConstraintPkgNames && len(tp.PkgName) != 0 {
				cname = tp.PkgName + "." + cname
			}
			vs[i] = tp.Name + " " + cname
		}
		return name + "[" + strings.Join(vs, ",") + "]"
	}

	if opts.TypeParams && len(sdata.TypeParams) != 0 {
		tps := sdata.TypeParams
		if opts.IncludeReaderWriterTypeParams {
			tps = append(tps, sdata.ReaderWriterTypeParams...)
		}

		vs := make([]string, len(tps))
		for i, tp := range tps {
			vs[i] = tp.Name
		}
		return name + "[" + strings.Join(vs, ",") + "]"
	}
	return name
}

func (g *Generator) prepFieldTypeName(sdata *StructData, fdata *typecheck.FieldData, prefix, suffix string, opts *fieldTypeNameOptions) string {
	if opts == nil {
		opts = &fieldTypeNameOptions{}
	}
	name := prefix + fdata.TypeName + suffix
	if opts.PkgName && len(fdata.TypePkgName) != 0 {
		name = fdata.TypePkgName + "." + name
	}

	var rwArgs []*typecheck.TypeArgData
	if opts.IncludeReaderWriterTypeArgs {
		for i, ta := range fdata.TypeArgs {
			if fdata.TypeParams[i].Constraint == "Struct" && fdata.TypeParams[i].PkgPath == "github.com/visvasity/blockgen/blockgen" {
				rwArgs = append(rwArgs, &typecheck.TypeArgData{Name: ta.Name + "Reader"})
				rwArgs = append(rwArgs, &typecheck.TypeArgData{Name: ta.Name + "Writer"})
			}
		}
	}

	if opts.TypeArgs && len(fdata.TypeArgs) != 0 {
		tas := fdata.TypeArgs
		if opts.IncludeReaderWriterTypeArgs {
			tas = append(tas, rwArgs...)
		}
		vs := make([]string, len(tas))
		for i, ta := range tas {
			if opts.TypeArgPkgNames && len(ta.PkgName) != 0 {
				vs[i] = ta.PkgName + "." + ta.Name
			} else {
				vs[i] = ta.Name
			}
		}
		return name + "[" + strings.Join(vs, ",") + "]"
	}
	return name
}

func (g *Generator) sliceField(sdata *StructData) *typecheck.FieldData {
	if n := len(sdata.Fields); n > 0 && sdata.Fields[n-1].Kind == "slice" {
		return sdata.Fields[n-1]
	}
	return nil
}

func (g *Generator) generateReaderWriterTypes(sdata *StructData) error {
	if err := g.addImport(sdata.StructName, "", "github.com/visvasity/blockgen/blockgen"); err != nil {
		return err
	}

	defineNameOptions := structTypeNameOptions{TypeParams: true, TypeConstraints: true, TypeConstraintPkgNames: true, IncludeReaderWriterTypeParams: true}
	readerDefinition := g.prepStructTypeName(sdata, "", "Reader", &defineNameOptions)
	writerDefinition := g.prepStructTypeName(sdata, "", "Writer", &defineNameOptions)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)

	g.P(sdata.StructName)
	g.P(sdata.StructName, "// Reader type defines accessor methods for read-only access.")
	g.P(sdata.StructName, "type ", readerDefinition, " blockgen.BlockBytes")
	g.P(sdata.StructName)
	g.P(sdata.StructName, "// Writer type extends the reader with mutable methods.")
	g.P(sdata.StructName, "type ", writerDefinition, " struct { ", readerReceiver, " }")
	g.P(sdata.StructName)

	return g.generateFieldMetadataVars(sdata)
}

func (g *Generator) generateReaderWriterMethods(sdata *StructData) error {
	typeName := sdata.StructName
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)

	g.P(typeName)
	g.P(typeName, "// BlockBytes returns access to the underlying byte slice.")
	g.P(typeName, "func (v ", readerReceiver, ") BlockBytes() blockgen.BlockBytes {")
	g.P(typeName, "  return blockgen.BlockBytes(v)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "// Writer returns the ", typeName, " writer for read-write access to it's fields.")
	g.P(typeName, "func (v ", readerReceiver, ") Writer() ", writerReceiver, " {")
	g.P(typeName, "  return ", writerReceiver, "{v}")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "// Reader returns the ", typeName, " reader with read-only access to it's fields.")
	g.P(typeName, "func (v ", writerReceiver, ") Reader() ", readerReceiver, " {")
	g.P(typeName, "  return v.", g.prepStructTypeName(sdata, "", "Reader", nil))
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}

func (g *Generator) structSizeVarName(sdata *StructData) string {
	return "structSizeOf" + sdata.StructName
}

func (g *Generator) fieldOffsetsVarName(sdata *StructData) string {
	return "fieldOffsetsOf" + sdata.StructName
}

func (g *Generator) fieldOffsetsMapVarName(sdata *StructData) string {
	return "fieldOffsetsOf" + sdata.StructName
}

func (g *Generator) sliceDataAlignVarName(sdata *StructData) string {
	return "sliceDataAlignOf" + sdata.StructName
}

func (g *Generator) sliceElementSizeVarName(sdata *StructData) string {
	return "sliceElemSizeOf" + sdata.StructName
}

// func (g *Generator) arrayElementSizeVarName(sdata *typecheck.StructData, fdata *typecheck.FieldData) string {
// 	return "arrayElemSizeOf" + sdata.StructName + fdata.Name
// }

func (g *Generator) generateFieldMetadataVars(sdata *StructData) error {
	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)

	if !g.IsGenericStruct(sdata) {
		if err := g.addImport(typeName, "", "github.com/visvasity/blockgen/blockgen"); err != nil {
			return err
		}
		if err := g.addImport(typeName, sdata.PkgName, sdata.PkgPath); err != nil {
			return err
		}

		g.P(typeName)
		g.P(typeName, "var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
		g.P(typeName, "var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](nil)")

		if sfdata := g.sliceField(sdata); sfdata != nil {
			sliceFieldTypeName := g.prepFieldTypeName(sdata, sfdata, "", "", &fieldTypeNameOptions{PkgName: true, TypeArgs: true, TypeArgPkgNames: true})
			g.P(typeName, "var ", g.sliceDataAlignVarName(sdata), " = blockgen.AlignFor[[]", sliceFieldTypeName, "]()")
			g.P(typeName, "var ", g.sliceElementSizeVarName(sdata), " = blockgen.ElemSizeFor[[]", sliceFieldTypeName, "]()")
		}
		g.P(typeName)
		return nil
	}

	g.P(typeName)
	g.P(typeName, "var ", g.fieldOffsetsMapVarName(sdata), " blockgen.OffsetsMap")
	g.P(typeName)
	return nil
}

func (g *Generator) generateZeroMethods(sdata *StructData) error {
	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)

	if g.IsGenericStruct(sdata) {
		if err := g.addImport(typeName, "", "github.com/visvasity/blockgen/blockgen"); err != nil {
			return err
		}
		if err := g.addImport(typeName, sdata.PkgName, sdata.PkgPath); err != nil {
			return err
		}
	}

	g.P(typeName)
	g.P(typeName, "// IsZero returns true if all underlying bytes are zero.")
	g.P(typeName, "func (v ", readerReceiver, ") IsZero() bool {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  return blockgen.IsZero(v[:", g.structSizeVarName(sdata), "])")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "// SetZero sets all underlying bytes to zero.")
	g.P(typeName, "func (v ", writerReceiver, ") SetZero() {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  blockgen.SetZero(v.BlockBytes()[:", g.structSizeVarName(sdata), "])")
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}

func (g *Generator) generateNumberFieldMethods(sdata *StructData, fdata *typecheck.FieldData) error {
	if fdata.Kind != "basic" {
		return os.ErrInvalid
	}

	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)
	fieldTypeName := g.prepFieldTypeName(sdata, fdata, "", "", &fieldTypeNameOptions{PkgName: true, TypeArgs: true, TypeArgPkgNames: true})

	if fdata.TypePkgName != "" {
		if err := g.addImport(typeName, fdata.TypePkgName, fdata.TypePkgPath); err != nil {
			return err
		}
	}

	fmethods := basicMethodTypeMap[fdata.BasicKind]

	if !g.IsGenericStruct(sdata) {
		g.P(typeName)
		g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "() ", fieldTypeName, " {")
		g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "]")
		g.P(typeName, "  return ", fieldTypeName, "(v.BlockBytes().", fmethods[1], "(offset))")
		g.P(typeName, "}")
		g.P(typeName)
	} else {
		g.P(typeName)
		g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "() ", fieldTypeName, " {")
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
		g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "]")
		if fdata.BasicKind == "generic" {
			g.P(typeName, "  return blockgen.NumberAt[", fieldTypeName, "](v.BlockBytes(), offset)")
		} else {
			g.P(typeName, "  return ", fieldTypeName, "(v.BlockBytes().", fmethods[1], "(offset))")
		}
		g.P(typeName, "}")
		g.P(typeName)
	}

	if !g.IsGenericStruct(sdata) {
		g.P(typeName)
		g.P(typeName, "func (v ", writerReceiver, ") Set", fdata.FieldName, "(x ", fieldTypeName, ") {")
		g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "]")
		g.P(typeName, "  v.BlockBytes().", fmethods[2], "(offset, ", fmethods[0], "(x))")
		g.P(typeName, "}")
		g.P(typeName)
	} else {
		g.P(typeName)
		g.P(typeName, "func (v ", writerReceiver, ") Set", fdata.FieldName, "(x ", fieldTypeName, ") {")
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
		g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "]")
		if fdata.BasicKind == "generic" {
			g.P(typeName, "  blockgen.SetNumberAt[", fieldTypeName, "](v.BlockBytes(), offset, x)")
		} else {
			g.P(typeName, "  v.BlockBytes().", fmethods[2], "(offset, ", fmethods[0], "(x))")
		}
		g.P(typeName, "}")
		g.P(typeName)
	}
	return nil
}

func (g *Generator) generateStructFieldMethods(sdata *StructData, fdata *typecheck.FieldData) error {
	if fdata.Kind != "struct" {
		return os.ErrInvalid
	}

	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	fieldReaderTypeName := g.prepFieldTypeName(sdata, fdata, "", "Reader", &fieldTypeNameOptions{TypeArgs: true, TypeArgPkgNames: true, IncludeReaderWriterTypeArgs: true})
	if g.IsGenericTypeParamField(sdata, fdata) {
		fieldReaderTypeName = fdata.TypeName + "Reader"
	}

	if fdata.TypePkgName != "" {
		if err := g.addImport(typeName, fdata.TypePkgName, fdata.TypePkgPath); err != nil {
			return err
		}
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "() ", fieldReaderTypeName, " {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "]")
	g.P(typeName, "  return ", fieldReaderTypeName, "(v.BlockBytes()[offset:])")
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}

func (g *Generator) generateArrayOfNumbersFieldMethods(sdata *StructData, fdata *typecheck.FieldData) error {
	if fdata.Kind != "array" || fdata.ArrayKind != "basic" {
		return os.ErrInvalid
	}

	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)
	fieldTypeName := g.prepFieldTypeName(sdata, fdata, "", "", &fieldTypeNameOptions{PkgName: true, TypeArgs: true, TypeArgPkgNames: true})

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "Len() int {")
	g.P(typeName, "  return int(", fdata.ArrayLens[0], ")")
	g.P(typeName, "}")
	g.P(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	fmethods := basicMethodTypeMap[fdata.BasicKind]

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "ItemAt(i int) ", fieldTypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.FieldName, `Len()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[1]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "] + i * elemSize")
	if fdata.BasicKind == "generic" {
		g.P(typeName, "  return blockgen.NumberAt[", fieldTypeName, "](v.BlockBytes(), offset)")
	} else {
		g.P(typeName, "  return ", fieldTypeName, "(v.BlockBytes().", fmethods[1], "(offset))")
	}
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Set", fdata.FieldName, "ItemAt(i int, x ", fieldTypeName, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.FieldName, `Len()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[1]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "] + i * elemSize")
	if fdata.BasicKind == "generic" {
		g.P(typeName, "  blockgen.SetNumberAt[", fieldTypeName, "](v.BlockBytes(), offset, x)")
	} else {
		g.P(typeName, "  v.BlockBytes().", fmethods[2], "(offset, ", fmethods[0], "(x))")
	}
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateArrayOfStructsFieldMethods(sdata *StructData, fdata *typecheck.FieldData) error {
	if fdata.Kind != "array" || fdata.ArrayKind != "struct" {
		return os.ErrInvalid
	}

	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)
	fieldReaderTypeName := g.prepFieldTypeName(sdata, fdata, "", "Reader", &fieldTypeNameOptions{TypeArgs: true, TypeArgPkgNames: true, IncludeReaderWriterTypeArgs: true})
	if g.IsGenericTypeParamField(sdata, fdata) {
		fieldReaderTypeName = fdata.TypeName + "Reader" // g.prepFieldTypeName(sdata, fdata, "", "", &structTypeNameOptions{TypeArgs: true})
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "Len() int {")
	g.P(typeName, "  return int(", fdata.ArrayLens[0], ")")
	g.P(typeName, "}")
	g.P(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "ItemAt(i int) ", fieldReaderTypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.FieldName, `Len()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[1]", fieldReaderTypeName, "]()")
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "] + i * elemSize")
	g.P(typeName, "  return ", fieldReaderTypeName, "(v.BlockBytes()[offset:])")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Set", fdata.FieldName, "ItemAt(i int, x ", fieldReaderTypeName, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("array index %d is out of range [0:%d]", i, v.`, fdata.FieldName, `Len()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[1]", fieldReaderTypeName, "]()")
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", fdata.Index, "] + i * elemSize")
	g.P(typeName, "  copy(v.BlockBytes()[offset:offset+elemSize], x[:elemSize])")
	g.P(typeName, "}")
	g.P(typeName)

	// TODO: We can add CopyTo and CopyFrom methods as well because these are the builtin types.

	return nil
}

func (g *Generator) generateSliceMethods(sdata *StructData) error {
	sfdata := g.sliceField(sdata)
	if sfdata == nil {
		return nil
	}

	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)
	fieldTypeName := g.prepFieldTypeName(sdata, sfdata, "", "", &fieldTypeNameOptions{PkgName: true, TypeArgs: true, TypeArgPkgNames: true})

	capFuncName := g.prepStructTypeName(sdata, "", "SliceFieldCap", &structTypeNameOptions{TypeConstraints: true, TypeConstraintPkgNames: true})

	g.P(typeName)
	g.P(typeName, "// ", typeName, "SliceFieldCap returns the slice field capacity for the given underlying byte slice size.")
	g.P(typeName, "func ", capFuncName, "(nbytes int) int {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
		g.P(typeName, "  var ", g.sliceElementSizeVarName(sdata), " = blockgen.ElemSizeFor[", fieldTypeName, "]()")
	}
	g.P(typeName, "  // TODO: We should also add the required alignment offset to the struct-size.")
	g.P(typeName, "  return (nbytes - ", g.structSizeVarName(sdata), ") /", g.sliceElementSizeVarName(sdata))
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "// ", sfdata.FieldName, "Len method returns number of elements in the slice field.")
	g.P(typeName, "func (v ", readerReceiver, ") ", sfdata.FieldName, "Len() int {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", sfdata.Index, "] + blockgen.OffsetOfSliceLen")
	g.P(typeName, "  return blockgen.IntAt(v.BlockBytes(), offset)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") internalSet", sfdata.FieldName, "Len(x int) {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", sfdata.Index, "] + blockgen.OffsetOfSliceLen")
	g.P(typeName, "  blockgen.SetIntAt(v.BlockBytes(), offset, x)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "// ", sfdata.FieldName, "Cap method returns maximum number of elements for the slice field.")
	g.P(typeName, "func (v ", readerReceiver, ") ", sfdata.FieldName, "Cap() int {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", sfdata.Index, "] + blockgen.OffsetOfSliceCap")
	g.P(typeName, "  return blockgen.IntAt(v.BlockBytes(), offset)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") internalSet", sfdata.FieldName, "Cap(x int) {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.fieldOffsetsVarName(sdata), " = blockgen.OffsetsFor[", inputStructName, "](&", g.fieldOffsetsMapVarName(sdata), ")")
	}
	g.P(typeName, "  var offset = ", g.fieldOffsetsVarName(sdata), "[", sfdata.Index, "] + blockgen.OffsetOfSliceCap")
	g.P(typeName, "  blockgen.SetIntAt(v.BlockBytes(), offset, x)")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Resize", sfdata.FieldName, "(size int) int {")
	g.P(typeName, "  if cap := v.", sfdata.FieldName, "Cap(); size > cap {")
	g.P(typeName, "    size = cap")
	g.P(typeName, "  }")
	g.P(typeName, "  n := v.", sfdata.FieldName, "Len()")
	g.P(typeName, "  if size == n {")
	g.P(typeName, "    return size")
	g.P(typeName, "  }")
	g.P(typeName, "  if size < n {")
	g.P(typeName, "    v.Delete", sfdata.FieldName, "Items(size, n)")
	g.P(typeName, "    return size")
	g.P(typeName, "  }")
	g.P(typeName, "  v.internalSet", sfdata.FieldName, "Len(size)")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName, "  var begin = ", g.structSizeVarName(sdata), " + n * elemSize")
	g.P(typeName, "  var end = ", g.structSizeVarName(sdata), " + size * elemSize")
	g.P(typeName, "  blockgen.SetZero(v.BlockBytes()[begin:end])")
	g.P(typeName, "  return size")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Delete", sfdata.FieldName, "Items(i, j int) {")
	g.P(typeName, "  n := v.", sfdata.FieldName, "Len()")
	g.P(typeName, "  if i < 0 || i >= n {")
	g.P(typeName, `    panic(fmt.Sprintf("first slice index %d is out of range [0:%d:%d]", i, v.`, sfdata.FieldName, `Len(), v.`, sfdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  if j < 0 || j >= n {")
	g.P(typeName, `    panic(fmt.Sprintf("second slice index %d is out of range [0:%d:%d]", i, v.`, sfdata.FieldName, `Len(), v.`, sfdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  if j < i {")
	g.P(typeName, `    panic(fmt.Sprintf("invalid slice indices %d < %d", j, i))`)
	g.P(typeName, "  }")
	g.P(typeName, "  if i == j {")
	g.P(typeName, "    return")
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName)
	g.P(typeName, "  ioff := ", g.structSizeVarName(sdata), " + i * elemSize")
	g.P(typeName, "  joff := ", g.structSizeVarName(sdata), " + j * elemSize")
	g.P(typeName, "  end := ", g.structSizeVarName(sdata), " + n * elemSize")
	g.P(typeName)
	g.P(typeName, "  copy(v.BlockBytes()[ioff:end], v.BlockBytes()[joff:end])")
	g.P(typeName, "  blockgen.SetZero(v.BlockBytes()[end-(joff-ioff):end])")
	g.P(typeName, "  v.internalSet", sfdata.FieldName, "Len(n-(j-i))")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generateSliceOfNumbersFieldMethods(sdata *StructData, fdata *typecheck.FieldData) error {
	if fdata.Kind != "slice" || fdata.SliceKind != "basic" {
		return os.ErrInvalid
	}

	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)
	fieldTypeName := g.prepFieldTypeName(sdata, fdata, "", "", &fieldTypeNameOptions{PkgName: true, TypeArgs: true, TypeArgPkgNames: true})

	fmethods := basicMethodTypeMap[fdata.BasicKind]

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "ItemAt(i int) ", fieldTypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [:%d:%d]", i, v.`, fdata.FieldName, `Len(), v.`, fdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.structSizeVarName(sdata), " + i * elemSize")
	if fdata.BasicKind == "generic" {
		g.P(typeName, "  return blockgen.NumberAt[", fieldTypeName, "](v.BlockBytes(), offset)")
	} else {
		g.P(typeName, "  return ", fieldTypeName, "(v.BlockBytes().", fmethods[1], "(offset))")
	}
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Set", fdata.FieldName, "ItemAt(i int, x ", fieldTypeName, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [:%d:%d]", i, v.`, fdata.FieldName, `Len(), v.`, fdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.structSizeVarName(sdata), " + i * elemSize")
	if fdata.BasicKind == "generic" {
		g.P(typeName, "  blockgen.SetNumberAt[", fieldTypeName, "](v.BlockBytes(), offset, x)")
	} else {
		g.P(typeName, "  v.BlockBytes().", fmethods[2], "(offset, ", fmethods[0], "(x))")
	}
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Append", fdata.FieldName, "Item(x ", fieldTypeName, ") {")
	g.P(typeName, "  n := v.", fdata.FieldName, "Len()")
	g.P(typeName, "  if n == v.", fdata.FieldName, "Cap() {")
	g.P(typeName, `    panic(fmt.Sprintf("append to slice overflows the maximum capacity [::%d]", v.`, fdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.internalSet", fdata.FieldName, "Len(n+1)")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.structSizeVarName(sdata), " + n * elemSize")
	if fdata.BasicKind == "generic" {
		g.P(typeName, "  blockgen.SetNumberAt[", fieldTypeName, "](v.BlockBytes(), offset, x)")
	} else {
		g.P(typeName, "  v.BlockBytes().", fmethods[2], "(offset, ", fmethods[0], "(x))")
	}
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}

func (g *Generator) generateSliceOfStructsFieldMethods(sdata *StructData, fdata *typecheck.FieldData) error {
	if fdata.Kind != "slice" || fdata.SliceKind != "struct" {
		return os.ErrInvalid
	}

	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)
	fieldTypeName := g.prepFieldTypeName(sdata, fdata, "", "", &fieldTypeNameOptions{PkgName: true, TypeArgs: true, TypeArgPkgNames: true})
	fieldReaderTypeName := g.prepFieldTypeName(sdata, fdata, "", "Reader", &fieldTypeNameOptions{TypeArgs: true, TypeArgPkgNames: true, IncludeReaderWriterTypeArgs: true})
	if g.IsGenericTypeParamField(sdata, fdata) {
		fieldReaderTypeName = fdata.TypeName + "Reader"
	}
	// fieldWriterTypeName := g.prepFieldTypeName(sdata, fdata, "", "Writer", &fieldTypeNameOptions{TypeArgs: true, TypeArgPkgNames: true, IncludeReaderWriterTypeArgs: true})
	// if g.IsGenericTypeParamField(sdata, fdata) {
	// 	fieldReaderTypeName = fdata.TypeName + "Reader"
	// }

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") ", fdata.FieldName, "ItemAt(i int) ", fieldReaderTypeName, " {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [:%d:%d]", i, v.`, fdata.FieldName, `Len(), v.`, fdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.structSizeVarName(sdata), " + i * elemSize")
	g.P(typeName, "  return ", fieldReaderTypeName, "(v.BlockBytes()[offset:])")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Set", fdata.FieldName, "ItemAt(i int, x ", fieldReaderTypeName, ") {")
	g.P(typeName, "  if i < 0 || i >= v.", fdata.FieldName, "Len() {")
	g.P(typeName, `    panic(fmt.Sprintf("slice index %d is out of range [:%d:%d]", i, v.`, fdata.FieldName, `Len(), v.`, fdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.structSizeVarName(sdata), " + i * elemSize")
	g.P(typeName, "  copy(v.BlockBytes()[offset:offset+elemSize], x.BlockBytes()[:elemSize])")
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") Append", fdata.FieldName, "Item(x ", fieldReaderTypeName, ") {")
	g.P(typeName, "  n := v.", fdata.FieldName, "Len()")
	g.P(typeName, "  if n == v.", fdata.FieldName, "Cap() {")
	g.P(typeName, `    panic(fmt.Sprintf("append to slice overflows the maximum capacity [::%d]", v.`, fdata.FieldName, `Cap()))`)
	g.P(typeName, "  }")
	g.P(typeName, "  v.internalSet", fdata.FieldName, "Len(n+1)")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  var elemSize = blockgen.ElemSizeFor[[]", fieldTypeName, "]()")
	g.P(typeName, "  var offset = ", g.structSizeVarName(sdata), " + n * elemSize")
	g.P(typeName, "  if x == nil {")
	g.P(typeName, "    blockgen.SetZero(v.BlockBytes()[offset:offset+elemSize])")
	g.P(typeName, "  } else {")
	g.P(typeName, "    copy(v.BlockBytes()[offset:offset+elemSize], x.BlockBytes()[:elemSize])")
	g.P(typeName, "  }")
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}

func (g *Generator) generateNewAndOpenMethods(typeName string) error {
	sdata, ok := g.structDataMap[typeName]
	if !ok {
		return os.ErrNotExist
	}

	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerName := g.prepStructTypeName(sdata, "", "Reader", nil)
	inputDefinition := g.prepStructTypeName(sdata, "", "", &structTypeNameOptions{TypeConstraints: true, TypeConstraintPkgNames: true, IncludeReaderWriterTypeParams: true})
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	// writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)

	capFuncCallName := g.prepStructTypeName(sdata, "", "SliceFieldCap", &structTypeNameOptions{TypeParams: true})

	g.P(typeName)
	g.P(typeName, "// New", readerName, " creates a zero-initialized ", typeName, ". Returns nil if input block size is too small.")
	g.P(typeName, "func New", inputDefinition, "(block []byte) ", readerReceiver, " {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  size := len(block)")
	g.P(typeName, "  if size < ", g.structSizeVarName(sdata), " {")
	g.P(typeName, "    return nil")
	g.P(typeName, "  }")
	g.P(typeName, "  blockgen.SetZero(block)")
	g.P(typeName, "  v := ", readerReceiver, "(block)")

	if sfdata := g.sliceField(sdata); sfdata != nil {
		g.P(typeName, "  // ", typeName, " type has a slice field; we must set a cap on it.")
		g.P(typeName, "  n := ", capFuncCallName, "(size)")
		g.P(typeName, "  v.Writer().internalSet", sfdata.FieldName, "Cap(n)")
	}
	g.P(typeName, "  return v")
	g.P(typeName, "}")
	g.P(typeName)

	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "// Open", readerName, " parses and prepares an existing ", typeName, " for read/write access.")
	g.P(typeName, "func Open", inputDefinition, "(block []byte) (", readerReceiver, ", error) {")
	if g.IsGenericStruct(sdata) {
		g.P(typeName, "  var ", g.structSizeVarName(sdata), " = blockgen.SizeFor[", inputStructName, "]()")
	}
	g.P(typeName, "  size := len(block)")
	g.P(typeName, "  if size < ", g.structSizeVarName(sdata), " {")
	g.P(typeName, `    return nil, fmt.Errorf("input size is too small")`)
	g.P(typeName, "  }")
	g.P(typeName, "  v := ", readerReceiver, "(block)")
	if sfdata := g.sliceField(sdata); sfdata != nil {
		g.P(typeName, "  // ", typeName, " type has a slice field; validate it's len and cap.")
		g.P(typeName, "  n := ", capFuncCallName, "(size)")
		g.P(typeName, "  if x := v.", sfdata.FieldName, "Cap(); x != n {")
		g.P(typeName, `    return nil, fmt.Errorf("slice field cap must be %d, found %d", n, x)`)
		g.P(typeName, "  }")
		g.P(typeName, "  if x := v.", sfdata.FieldName, "Len(); x < 0 || x > n {")
		g.P(typeName, `    return nil, fmt.Errorf("slice field len is %d, must be between [%d-%d)", x, 0, n)`)
		g.P(typeName, "  }")
	}
	g.P(typeName, "  return v, nil")
	g.P(typeName, "}")
	g.P(typeName)

	return nil
}

func (g *Generator) generatePrintMethods(sdata *StructData) error {
	typeName := sdata.StructName
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)

	if err := g.addImport(typeName, "", "strings"); err != nil {
		return err
	}
	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") String() string {")
	g.P(typeName, "  var sb strings.Builder")
	for i, fdata := range sdata.Fields {
		if i != 0 {
			g.P(typeName, `  fmt.Fprintf(&sb, " ")`)
		}

		switch fdata.Kind {
		case "basic":
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.FieldName, `=%d", v.`, fdata.FieldName, `())`)
		case "struct":
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.FieldName, `={%v}", v.`, fdata.FieldName, `())`)
		case "array":
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.FieldName, `=[%d]{", v.`, fdata.FieldName, `Len())`)
			g.P(typeName, `  for i := 0; i < v.`, fdata.FieldName, `Len(); i++ {`)
			g.P(typeName, `    if i == 0 {`)
			if fdata.ArrayKind == "basic" {
				g.P(typeName, `      fmt.Fprintf(&sb, "%d", v.`, fdata.FieldName, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, "{%v}", v.`, fdata.FieldName, `ItemAt(i))`)
			}
			g.P(typeName, `    } else {`)
			if fdata.ArrayKind == "basic" {
				g.P(typeName, `      fmt.Fprintf(&sb, " %d", v.`, fdata.FieldName, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, " {%v}", v.`, fdata.FieldName, `ItemAt(i))`)
			}
			g.P(typeName, `    }`)
			g.P(typeName, `  }`)
			g.P(typeName, `  fmt.Fprintf(&sb, "}")`)
		case "slice":
			g.P(typeName, `  fmt.Fprintf(&sb, "`, fdata.FieldName, `=[:%d:%d]{", v.`, fdata.FieldName, `Len(), v.`, fdata.FieldName, `Cap())`)
			g.P(typeName, `  for i := 0; i < v.`, fdata.FieldName, `Len(); i++ {`)
			g.P(typeName, `    if i == 0 {`)
			if fdata.SliceKind == "basic" {
				g.P(typeName, `      fmt.Fprintf(&sb, "%d", v.`, fdata.FieldName, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, "{%v}", v.`, fdata.FieldName, `ItemAt(i))`)
			}
			g.P(typeName, `    } else {`)
			if fdata.SliceKind == "basic" {
				g.P(typeName, `      fmt.Fprintf(&sb, " %d", v.`, fdata.FieldName, `ItemAt(i))`)
			} else {
				g.P(typeName, `      fmt.Fprintf(&sb, " {%v}", v.`, fdata.FieldName, `ItemAt(i))`)
			}
			g.P(typeName, `    }`)
			g.P(typeName, `  }`)
			g.P(typeName, `  fmt.Fprintf(&sb, "}")`)
		}
	}
	g.P(typeName, "  return sb.String()")
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}

func (g *Generator) generateCopyMethods(sdata *StructData) error {
	typeName := sdata.StructName
	inputStructName := g.prepStructTypeName(sdata, "", "", &inputStructNameOpts)
	readerReceiver := g.prepStructTypeName(sdata, "", "Reader", &receiverNameOptions)
	writerReceiver := g.prepStructTypeName(sdata, "", "Writer", &receiverNameOptions)

	if err := g.addImport(typeName, "", "strings"); err != nil {
		return err
	}
	if err := g.addImport(typeName, "", "fmt"); err != nil {
		return err
	}

	g.P(typeName)
	g.P(typeName, "func (v ", readerReceiver, ") CopyTo(x *", inputStructName, ") {")
	for _, fdata := range sdata.Fields {
		switch fdata.Kind {
		case "basic":
			g.P(typeName, "  x.", fdata.FieldName, " = v.", fdata.FieldName, "()")
		case "struct":
			g.P(typeName, "  v.", fdata.FieldName, "().CopyTo(&x.", fdata.FieldName, ")")
		case "array":
			g.P(typeName, `  for i := 0; i < v.`, fdata.FieldName, `Len(); i++ {`)
			if fdata.ArrayKind == "basic" {
				g.P(typeName, `      x.`, fdata.FieldName, `[i] = v.`, fdata.FieldName, `ItemAt(i)`)
			} else {
				g.P(typeName, `      v.`, fdata.FieldName, `ItemAt(i).CopyTo(&x.`, fdata.FieldName, `[i])`)
			}
			g.P(typeName, "  }")
		case "slice":
			fieldTypeName := g.prepFieldTypeName(sdata, fdata, "", "", &fieldTypeNameOptions{PkgName: true, TypeArgs: true, TypeArgPkgNames: true})
			g.P(typeName, `  x.`, fdata.FieldName, ` = make([]`, fieldTypeName, `, v.`, fdata.FieldName, `Len())`)
			g.P(typeName, `  for i := 0; i < v.`, fdata.FieldName, `Len(); i++ {`)
			if fdata.SliceKind == "basic" {
				g.P(typeName, `      x.`, fdata.FieldName, `[i] = v.`, fdata.FieldName, `ItemAt(i)`)
			} else {
				g.P(typeName, `      v.`, fdata.FieldName, `ItemAt(i).CopyTo(&x.`, fdata.FieldName, `[i])`)
			}
			g.P(typeName, `  }`)
		}
	}
	g.P(typeName, "}")
	g.P(typeName)

	g.P(typeName)
	g.P(typeName, "func (v ", writerReceiver, ") CopyFrom(x *", inputStructName, ") {")
	for _, fdata := range sdata.Fields {
		switch fdata.Kind {
		case "basic":
			g.P(typeName, "  v.Set", fdata.FieldName, "(x.", fdata.FieldName, ")")
		case "struct":
			g.P(typeName, "  v.", fdata.FieldName, "().Writer().CopyFrom(&x.", fdata.FieldName, ")")
		case "array":
			g.P(typeName, `  for i := 0; i < v.`, fdata.FieldName, `Len(); i++ {`)
			if fdata.ArrayKind == "basic" {
				g.P(typeName, "  v.Set", fdata.FieldName, "ItemAt(i, x.", fdata.FieldName, "[i])")
			} else {
				g.P(typeName, "  v.", fdata.FieldName, "ItemAt(i).Writer().CopyFrom(&x.", fdata.FieldName, "[i])")
			}
			g.P(typeName, `  }`)
		case "slice":
			g.P(typeName, `  v.Resize`, fdata.FieldName, `(len(x.`, fdata.FieldName, `))`)
			g.P(typeName, `  for i := 0; i < len(x.`, fdata.FieldName, `); i++ {`)
			if fdata.SliceKind == "basic" {
				g.P(typeName, "  v.Set", fdata.FieldName, "ItemAt(i, x.", fdata.FieldName, "[i])")
			} else {
				g.P(typeName, "  v.", fdata.FieldName, "ItemAt(i).Writer().CopyFrom(&x.", fdata.FieldName, "[i])")
			}
			g.P(typeName, `  }`)
		}
	}
	g.P(typeName, "}")
	g.P(typeName)
	return nil
}
