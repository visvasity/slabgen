// Copyright (c) 2025 Visvasity LLC

package internal

import (
	"fmt"
	"go/types"
	"iter"
	"log"
	"strings"

	"golang.org/x/tools/go/types/typeutil"
)

type NameData struct {
	Name string

	Kind string // One of ["basic", "struct", "array", "slice", "interface"]

	TypeParams      []string
	TypeConstraints []string

	InputPkgPath string
	InputPkgName string
}

type StructData struct {
	StructName string

	InputPkgPath    string
	InputPkgName    string
	InputStructName string

	TypeParams      []string
	TypeConstraints []string

	ReaderName string
	WriterName string

	// FieldList is NOT ordered as per the struct type field order.
	FieldList []*FieldData
}

func (s *StructData) IsGeneric() bool {
	return len(s.TypeParams) != 0
}

func (s *StructData) SliceField() *FieldData {
	for _, f := range s.FieldList {
		if f.Kind == "slice" {
			return f
		}
	}
	return nil
}

func (s *StructData) TypeParamsStr() string {
	if s.IsGeneric() {
		return "[" + strings.Join(s.TypeParams, ",") + "]"
	}
	return ""
}

func (s *StructData) TypeConstraintsStr() string {
	if s.IsGeneric() {
		str := ""
		for i := range s.TypeParams {
			if i == 0 {
				str += s.TypeParams[i] + " " + s.TypeConstraints[i]
			} else {
				str += "," + s.TypeParams[i] + " " + s.TypeConstraints[i]
			}
		}
		return "[" + str + "]"
	}
	return ""
}

func (s *StructData) InputTypeName(tparams bool) string {
	if !s.IsGeneric() {
		return s.InputStructName
	}
	if tparams {
		return s.InputStructName + s.TypeParamsStr()
	}
	return s.InputStructName
}

/////////////////////////////////////////////////////////////////////////////

type FieldData struct {
	Index int
	Name  string
	Kind  string // One of ["number","struct","array", "slice"]

	// Type holds the Go type name for NUMBER and STRUCT kinds and element type
	// for ARRAY and SLICE kinds. For fields based on generic type parameter, it
	// will be the generic type name.
	Type string

	// Kind == "number" || ArrayKind == "number" || SliceKind == "number"
	NumberKind string // One of ["generic","int8","int16","int32","int64","uint8","uint16","uint32","uint64","float32","float64"]

	// Kind == "struct" || ArrayKind == "struct" || SliceKind == "struct"
	StructKind string // One of ["generic", "struct"]
	StructName string

	// Kind == "array"
	ArrayLen  int64
	ArrayKind string // One of ["number","struct"]

	// Kind == "slice"
	SliceKind string // One of ["number","struct"]

	// GenericKind string // One of ["parameterized", "instantiated"]

	// Cross package import required for the type name used by the field.
	ImportTypeName    string
	ImportPackagePath string
	ImportPackageName string
}

func (f *FieldData) IsGeneric() bool {
	switch {
	case f.Kind == "number" && f.NumberKind == "generic":
		return true
	case f.Kind == "struct" && f.StructKind == "generic":
		return true
	case f.Kind == "array" && f.ArrayKind == "number" && f.NumberKind == "generic":
		return true
	case f.Kind == "array" && f.ArrayKind == "struct" && f.StructKind == "generic":
		return true
	case f.Kind == "slice" && f.SliceKind == "number" && f.NumberKind == "generic":
		return true
	case f.Kind == "slice" && f.SliceKind == "struct" && f.StructKind == "generic":
		return true
	}
	return false
}

/////////////////////////////////////////////////////////////////////////////

type TypeChecker struct {
	nameDataMap map[string]*NameData

	structDataMap map[string]*StructData

	structsWithSlice map[string]bool

	failedTypes   typeutil.Map // map[*types.Type]error
	checkedTypes  typeutil.Map // map[*types.Type]*StructData
	checkingTypes typeutil.Map // map[*types.Type]bool
}

func NewTypeChecker() *TypeChecker {
	return &TypeChecker{
		nameDataMap:      make(map[string]*NameData),
		structDataMap:    make(map[string]*StructData),
		structsWithSlice: make(map[string]bool),
	}
}

func (tc *TypeChecker) GetStruct(typeName string) (*StructData, error) {
	for _, v := range tc.structDataMap {
		if v.StructName == typeName {
			return v, nil
		}
	}
	return nil, fmt.Errorf("typename %q doesn't exist", typeName)
}

func (tc *TypeChecker) NameDataMap() map[string]*NameData {
	return tc.nameDataMap
}

func (tc *TypeChecker) AllStructData() iter.Seq2[string, *StructData] {
	return func(yield func(string, *StructData) bool) {
		for k, v := range tc.structDataMap {
			if !yield(k, v) {
				return
			}
		}
	}
}

func nameKey(tn *types.TypeName) string {
	pkg := tn.Pkg()
	if pkg == nil {
		return tn.Name()
	}
	return pkg.Path() + "." + tn.Name()
}

func getKind(v types.Type) (string, error) {
	switch v.Underlying().(type) {
	case *types.Basic:
		return "basic", nil
	case *types.Struct:
		return "struct", nil
	case *types.Array:
		return "array", nil
	case *types.Slice:
		return "slice", nil
	case *types.Interface, *types.Union:
		return "interface", nil
	}
	return "", fmt.Errorf("unsupported type %T (%v) with underlying type %T", v, v, v.Underlying())
}

// Collect prepares nameDataMap with all supported types.
func (tc *TypeChecker) Collect(datatype types.Type) error {
	kind, err := getKind(datatype)
	if err != nil {
		return err
	}

	switch x := datatype.(type) {
	default:
		return fmt.Errorf("unhandled input type %T", datatype)
	case *types.Chan, *types.Tuple, *types.Map, *types.Pointer, *types.Signature:
		return fmt.Errorf("unsupported input type %T", datatype)

	case *types.Basic:
		key := x.Name()
		if _, ok := tc.nameDataMap[key]; !ok {
			tc.nameDataMap[key] = &NameData{
				Kind: "basic",
				Name: key,
			}
		}
		return nil

	case *types.Union:
		for i := range x.Len() {
			if err := tc.Collect(x.Term(i).Type()); err != nil {
				return err
			}
		}
		return nil

	case *types.Interface:
		for i := range x.NumEmbeddeds() {
			if err := tc.Collect(x.EmbeddedType(i)); err != nil {
				return err
			}
		}
		return nil

	case *types.TypeParam:
		return tc.Collect(x.Constraint())

	case *types.Struct:
		for i := range x.NumFields() {
			ftype := x.Field(i).Type()
			if err := tc.Collect(ftype); err != nil {
				log.Printf("Collect: could not collect from field %d type in (%v): %v", i, x, err)
				return err
			}
		}
		return nil

	case *types.Array:
		etype := x.Elem()
		if err := tc.Collect(etype); err != nil {
			log.Printf("Collect: could not collect from array element type in (%v): %v", x, err)
			return err
		}
		return nil
	case *types.Slice:
		etype := x.Elem()
		if err := tc.Collect(etype); err != nil {
			log.Printf("Collect: could not collect from slice element type in (%v): %v", x, err)
			return err
		}
		return nil

	case *types.Alias:
		tn := x.Origin().Obj()
		key := nameKey(tn)
		if _, ok := tc.nameDataMap[key]; ok {
			return nil
		}
		data := &NameData{
			Kind: kind,
			Name: tn.Name(),
		}
		if pkg := tn.Pkg(); pkg != nil {
			data.InputPkgPath = pkg.Path()
			data.InputPkgName = pkg.Name()
		}
		if tps := x.TypeParams(); tps != nil {
			for i := range tps.Len() {
				tp := tps.At(i)
				if err := tc.Collect(tp); err != nil {
					return err
				}
				data.TypeParams = append(data.TypeParams, tp.Obj().Name())
				if named, ok := tp.Constraint().(interface{ Obj() *types.TypeName }); ok {
					data.TypeConstraints = append(data.TypeConstraints, nameKey(named.Obj()))
				} else {
					data.TypeConstraints = append(data.TypeConstraints, "")
				}
			}
		}
		if tas := x.TypeArgs(); tas != nil {
			for i := range tas.Len() {
				targ := tas.At(i)
				if err := tc.Collect(targ); err != nil {
					log.Printf("Collect: could not collect from type arg %d in (%v): %v", i, x, err)
					return err
				}
			}
		}
		tc.nameDataMap[key] = data
		if err := tc.Collect(x.Underlying()); err != nil {
			return err
		}
		return nil
	case *types.Named:
		tn := x.Origin().Obj()
		key := nameKey(tn)
		if _, ok := tc.nameDataMap[key]; ok {
			return nil
		}
		data := &NameData{
			Kind: kind,
			Name: tn.Name(),
		}
		if pkg := tn.Pkg(); pkg != nil {
			data.InputPkgPath = pkg.Path()
			data.InputPkgName = pkg.Name()
		}
		if tps := x.TypeParams(); tps != nil {
			for i := range tps.Len() {
				tp := tps.At(i)
				if err := tc.Collect(tp); err != nil {
					return err
				}
				data.TypeParams = append(data.TypeParams, tp.Obj().Name())
				if named, ok := tp.Constraint().(interface{ Obj() *types.TypeName }); ok {
					data.TypeConstraints = append(data.TypeConstraints, nameKey(named.Obj()))
				} else {
					data.TypeConstraints = append(data.TypeConstraints, "")
				}
			}
		}
		if tas := x.TypeArgs(); tas != nil {
			for i := range tas.Len() {
				targ := tas.At(i)
				if err := tc.Collect(targ); err != nil {
					log.Printf("Collect: could not collect from type arg %d in (%v): %v", i, x, err)
					return err
				}
			}
		}
		if err := tc.Collect(x.Underlying()); err != nil {
			return err
		}
		tc.nameDataMap[key] = data
		return nil
	}
}

func (tc *TypeChecker) Check(localName string, named *types.Named) (sdata *StructData, status error) {
	if localName == "" {
		localName = named.Obj().Name()
	}
	if v, ok := tc.structDataMap[localName]; ok {
		return v, nil
	}

	sdata = &StructData{
		StructName: localName,
		ReaderName: localName,
		WriterName: localName + "Writer",

		InputPkgName:    named.Obj().Pkg().Name(),
		InputPkgPath:    named.Obj().Pkg().Path(),
		InputStructName: named.Obj().Name(),
	}
	defer func() {
		if status == nil {
			tc.structDataMap[localName] = sdata
		}
	}()

	// Input type MUST be resolved to a struct type.
	s, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("checkType: %v: input type is not a named struct type", named)
	}

	if v := tc.failedTypes.At(s); v != nil {
		return nil, fmt.Errorf("checkType: %v: struct type is not supported: %w", s, v.(error))
	}
	if v := tc.checkingTypes.At(s); v != nil {
		return nil, fmt.Errorf("checkType: %v: struct type with recursive references is not supported", s)
	}

	tc.checkingTypes.Set(s, true)
	defer func() {
		if status == nil {
			tc.checkedTypes.Set(s, sdata)
		} else {
			tc.failedTypes.Set(s, status)
		}
		tc.checkingTypes.Delete(s)
	}()

	// Verify that if the input type is a generic type, type constraints are all
	// from blockgen package.
	if tps := named.TypeParams(); tps.Len() != 0 {
		for i := range tps.Len() {
			tp := tps.At(i)
			named, ok := tp.Constraint().(*types.Named)
			if !ok {
				return nil, fmt.Errorf("Check: %v: generic types type parameters must all be named", tp)
			}
			sdata.TypeParams = append(sdata.TypeParams, tp.Obj().Name())
			sdata.TypeConstraints = append(sdata.TypeConstraints, "blockgen."+named.Obj().Name())
		}
	}

	log.Printf("Check: Named=%v Named.Obj=%v Named.Origin=%v Named.Underlying=%v Named.TypeParams=%v Named.TypeArgs=%v", named, named.Obj(), named.Origin(), named.Underlying(), named.TypeParams().Len(), named.TypeArgs().Len())

	for i := 0; i < s.NumFields(); i++ {
		fdata := &FieldData{Index: i /*Size: -1, Align: -1*/}
		if err := tc.checkFieldType(named, s, s.Field(i), fdata); err != nil {
			return nil, err
		}
		sdata.FieldList = append(sdata.FieldList, fdata)
	}

	return sdata, nil
}

func (tc *TypeChecker) checkFieldType(named *types.Named, s *types.Struct, f *types.Var, fdata *FieldData) error {
	ftype := f.Type()
	futype := ftype.Underlying()
	// log.Printf("checkFieldType: f(%T)=%v; ftype(%T)=%v; f.Underlying(%T)=%v;", f, f, ftype, ftype, futype, futype)
	// log.Printf("checkFieldType: isFixedType(ftype)=%t isInstantiatedType(ftype)=%t :: %v", isFixedSizeType(ftype), isInstantiatedType(ftype), f)
	qname, err := getQualifiedTypeName(ftype)
	log.Printf("checkFieldType: QualifiedTypeName=%q err=%v :: %v", qname, err, f)

	fdata.Name = f.Name()

	switch {
	case !f.IsField():
		return fmt.Errorf("checkFieldType: %v: is not a struct field", f)

	case f.Embedded():
		return fmt.Errorf("checkFieldType: %v: anonymous/embedded fields are not supported", f)

	case !f.Exported():
		return fmt.Errorf("checkFieldType: %v: field is not exported", f)

	case isBasicType(ftype):
		if err := tc.checkBasicField(named, s, f, fdata); err != nil {
			return err
		}

	case isStructType(ftype):
		if err := tc.checkStructField(named, s, f, fdata); err != nil {
			return err
		}

	case isArrayType(ftype):
		if err := tc.checkArrayField(named, s, f, fdata); err != nil {
			return err
		}

	case isSliceType(ftype):
		if err := tc.checkSliceField(named, s, f, fdata); err != nil {
			return err
		}

	default:
		return fmt.Errorf("checkFieldType: field (%v) of type %T (Underlying: %T) is not supported", f, ftype, futype)
	}
	return nil
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

func getQualifiedTypeName(t types.Type) (string, error) {
	switch x := t.(type) {
	case *types.Basic:
		return x.Name(), nil
	case *types.Slice:
		name, err := getQualifiedTypeName(x.Elem())
		if err != nil {
			return "", err
		}
		return "[]" + name, nil
	case *types.Array:
		name, err := getQualifiedTypeName(x.Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[%d]", x.Len()) + name, nil
	case *types.TypeParam:
		return x.Obj().Name(), nil
	case *types.Named:
		if vs := typeArgs(x); len(vs) != 0 {
			return x.Obj().Name() + "[" + strings.Join(vs, ",") + "]", nil
		}
		if vs := typeParams(x); len(vs) != 0 {
			return x.Obj().Name() + "[" + strings.Join(vs, ",") + "]", nil
		}
		return x.Obj().Name(), nil
	case *types.Alias:
		if vs := typeArgs(x); len(vs) != 0 {
			return x.Obj().Name() + "[" + strings.Join(vs, ",") + "]", nil
		}
		if vs := typeParams(x); len(vs) != 0 {
			return x.Obj().Name() + "[" + strings.Join(vs, ",") + "]", nil
		}
		return x.Obj().Name(), nil
	}
	return "", fmt.Errorf("getQualifiedTypeName: %v: could not prepare type name", t)
}

func getStructKindAndType(ftype types.Type) (skind, gtype string, ok bool) {
	if _, ok := ftype.Underlying().(*types.Struct); ok {
		if named, ok := ftype.(*types.Named); ok {
			// FIXME: Type Arguments.
			return "struct", named.Obj().Name(), true
		}
		if alias, ok := ftype.(*types.Alias); ok {
			// FIXME: Type Arguments.
			return "struct", alias.Obj().Name(), true
		}
		return "", "", false
	}

	if tp, ok := ftype.(*types.TypeParam); ok {
		if named, ok := tp.Constraint().(*types.Named); ok {
			tn := named.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Struct" {
				return "generic", tp.Obj().Name(), true
			}
		}
	}
	return "", "", false
}

func getNumberKindAndType(ftype types.Type) (nkind, gtype string, ok bool) {
	if basic, ok := ftype.(*types.Basic); ok {
		if _, ok := fixedBasicTypeMap[basic.Kind()]; ok {
			return basic.Name(), basic.Name(), true
		}
		return "", "", false
	}

	if basic, ok := ftype.Underlying().(*types.Basic); ok {
		if named, ok := ftype.(*types.Named); ok {
			return basic.Name(), named.Obj().Name(), true
		}
		if alias, ok := ftype.(*types.Alias); ok {
			return basic.Name(), alias.Obj().Name(), true
		}
		return "", "", false
	}

	if tp, ok := ftype.(*types.TypeParam); ok {
		if named, ok := tp.Constraint().(*types.Named); ok {
			tn := named.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Number" {
				return "generic", tp.Obj().Name(), true
			}
		}
	}
	return "", "", false
}

func (tc *TypeChecker) checkBasicField(top *types.Named, s *types.Struct, f *types.Var, fdata *FieldData) error {
	ftype := f.Type()
	if basic, ok := ftype.(*types.Basic); ok {
		if _, ok := fixedBasicTypeMap[basic.Kind()]; ok {
			fdata.Kind = "number"
			fdata.Type = basic.Name()
			fdata.NumberKind = basic.Name()
			return nil
		}
		return fmt.Errorf("checkBasicField: %v: basic type is not a fixed size type", f)
	}

	if basic, ok := ftype.Underlying().(*types.Basic); ok {
		if _, ok := fixedBasicTypeMap[basic.Kind()]; !ok {
			return fmt.Errorf("checkBasicField: %v: named basic type is not fixed size type", f)
		}
		if named, ok := ftype.(*types.Named); ok {
			fdata.Kind = "number"
			fdata.NumberKind = basic.Name()
			fdata.Type = named.Obj().Name()
			fdata.ImportTypeName = named.Obj().Name()
			fdata.ImportPackagePath = named.Obj().Pkg().Path()
			fdata.ImportPackageName = named.Obj().Pkg().Name()
			return nil
		}
		if alias, ok := ftype.(*types.Alias); ok {
			fdata.Kind = "number"
			fdata.NumberKind = basic.Name()
			fdata.Type = alias.Obj().Name()
			fdata.ImportTypeName = alias.Obj().Name()
			fdata.ImportPackagePath = alias.Obj().Pkg().Path()
			fdata.ImportPackageName = alias.Obj().Pkg().Name()
			return nil
		}
		log.Printf("unhandled/unsupported basic underlying field (%v): type=%T utype=%T", f, ftype, ftype.Underlying())
		return fmt.Errorf("checkBasicField: %v: basic type field is unhandled or unsupported", f)
	}

	if tp, ok := ftype.(*types.TypeParam); ok {
		if named, ok := tp.Constraint().(*types.Named); ok {
			tn := named.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Number" {
				fdata.Kind = "number"
				fdata.NumberKind = "generic"
				fdata.Type = tp.Obj().Name()
				return nil
			}
		}
	}
	return fmt.Errorf("checkBasicField: %v: not a basic field", f)
}

func (tc *TypeChecker) checkStructField(top *types.Named, s *types.Struct, f *types.Var, fdata *FieldData) error {
	ftype := f.Type()
	if _, ok := ftype.(*types.Struct); ok {
		return fmt.Errorf("checkStructField: %v: anonymous struct field types are not supported", f)
	}

	if x, ok := ftype.Underlying().(*types.Struct); ok {
		if tc.failedTypes.At(x) != nil {
			return fmt.Errorf("checkStructField: %v: field's struct type is invalid/unsupported", f)
		}
		// FIXME: What about aliases to named struct types?
		named, ok := ftype.(*types.Named)
		if !ok {
			return fmt.Errorf("checkStructField: %v: field's struct type *must* be a named type", f)
		}
		if _, err := tc.Check("", named.Origin()); err != nil {
			return err
		}
		if _, ok := tc.structsWithSlice[named.Obj().Id()]; ok {
			return fmt.Errorf("checkStructField: %v: field's struct type has dynamic size due to slice members", f)
		}
		gtype, err := getQualifiedTypeName(named)
		if err != nil {
			return err
		}
		fdata.Kind = "struct"
		fdata.StructKind = "struct"
		if len(typeArgs(named)) != 0 {
			fdata.StructKind = "generic"
		}
		fdata.Type = gtype
		fdata.ImportTypeName = named.Obj().Name()
		fdata.ImportPackagePath = named.Obj().Pkg().Path()
		fdata.ImportPackageName = named.Obj().Pkg().Name()
		return nil
	}

	if tp, ok := ftype.(*types.TypeParam); ok {
		if named, ok := tp.Constraint().(*types.Named); ok {
			tn := named.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Struct" {
				fdata.Kind = "struct"
				fdata.StructKind = "generic"
				fdata.Type = tp.Obj().Name()
				return nil
			}
		}
	}
	return fmt.Errorf("checkStructField: %v: not a struct field type", f)
}

func (tc *TypeChecker) checkArrayField(top *types.Named, s *types.Struct, f *types.Var, fdata *FieldData) error {
	ftype := f.Type()
	array, ok := ftype.(*types.Array)
	if !ok {
		return fmt.Errorf("checkArrayField: %v: array field types must have direct size", f)
	}
	if array.Len() == 0 {
		return fmt.Errorf("checkArrayField: %v: zero length array fields are not supported", f)
	}

	elem := array.Elem()
	if !isBasicType(elem) && !isStructType(elem) {
		return fmt.Errorf("checkArrayField: %v: element type is not supported", f)
	}

	if isBasicType(elem) {
		if !isFixedSizeBasicType(elem) {
			return fmt.Errorf("checkArrayField: %v: element type is not a fixed size basic type", f)
		}
		nkind, gtype, ok := getNumberKindAndType(elem)
		if !ok {
			return fmt.Errorf("checkArrayField: %v: unexpected: could not determine number element details", f)
		}
		fdata.Kind = "array"
		fdata.ArrayLen = array.Len()
		fdata.ArrayKind = "number"
		fdata.Type = gtype
		fdata.NumberKind = nkind
		return nil
	}

	if _, ok := elem.(*types.Struct); ok {
		return fmt.Errorf("checkArrayField: %v: anonymous struct element type is not supported", f)
	}
	skind, gtype, ok := getStructKindAndType(elem)
	if !ok {
		return fmt.Errorf("checkArrayField: %v: could not determine type name for the struct element", f)
	}

	fdata.Kind = "array"
	fdata.ArrayLen = array.Len()
	fdata.ArrayKind = "struct"
	fdata.Type = gtype
	fdata.StructKind = skind
	return nil
}

func (tc *TypeChecker) checkSliceField(top *types.Named, s *types.Struct, f *types.Var, fdata *FieldData) error {
	ftype := f.Type()
	if fdata.Index != s.NumFields()-1 {
		return fmt.Errorf("checkFieldType: %v: slice field must be the (only) last field", f)
	}
	tc.structsWithSlice[top.Obj().Id()] = true

	slice, ok := ftype.(*types.Slice)
	if !ok {
		return fmt.Errorf("checkSliceField: %v: slice field types must be direct slices", f)
	}
	elem := slice.Elem()
	if !isBasicType(elem) && !isStructType(elem) {
		return fmt.Errorf("checkSliceField: %v: element type is not supported", f)
	}

	if isBasicType(elem) {
		if !isFixedSizeBasicType(elem) {
			return fmt.Errorf("checkArrayField: %v: element type is not a fixed size basic type", f)
		}
		nkind, gtype, ok := getNumberKindAndType(elem)
		if !ok {
			return fmt.Errorf("checkArrayField: %v: unexpected: could not determine number element details", f)
		}
		fdata.Kind = "slice"
		fdata.SliceKind = "number"
		fdata.Type = gtype
		fdata.NumberKind = nkind
		return nil
	}

	if _, ok := elem.(*types.Struct); ok {
		return fmt.Errorf("checkArrayField: %v: anonymous struct element type is not supported", f)
	}
	skind, gtype, ok := getStructKindAndType(elem)
	if !ok {
		return fmt.Errorf("checkArrayField: %v: could not determine type name for the struct element", f)
	}

	fdata.Kind = "slice"
	fdata.SliceKind = "struct"
	fdata.Type = gtype
	fdata.StructKind = skind
	return nil
}

func typeParams[T interface{ TypeParams() *types.TypeParamList }](x T) []string {
	tps := x.TypeParams()
	if tps == nil {
		return nil
	}
	vs := make([]string, tps.Len())
	for i := range tps.Len() {
		vs[i] = tps.At(i).Obj().Name()
	}
	return vs
}

func typeArgs[T interface{ TypeArgs() *types.TypeList }](x T) []string {
	tas := x.TypeArgs()
	if tas == nil {
		return nil
	}
	vs := make([]string, tas.Len())
	for i := range tas.Len() {
		vs[i] = tas.At(i).String()
	}
	return vs
}

func typeConstraints[T interface{ TypeParms() *types.TypeParam }](x T) [][2]string {
	return nil
}

func isInstantiatedType(ftype types.Type) bool {
	switch x := ftype.(type) {
	case *types.Basic:
		return true

	case *types.Struct:
		for i := range x.NumFields() {
			if !isInstantiatedType(x.Field(i).Type()) {
				return false
			}
		}

	case *types.Array:
		return isInstantiatedType(x.Elem())
	case *types.Slice:
		return isInstantiatedType(x.Elem())

	case *types.Alias:
		tas := x.TypeArgs()
		for i := range tas.Len() {
			if _, ok := tas.At(i).(*types.TypeParam); ok {
				return false
			}
			if !isInstantiatedType(tas.At(i)) {
				return false
			}
		}
		return true

	case *types.Named:
		tas := x.TypeArgs()
		for i := range tas.Len() {
			if _, ok := tas.At(i).(*types.TypeParam); ok {
				return false
			}
			if !isInstantiatedType(tas.At(i)) {
				return false
			}
		}
		return true
	}
	return false
}

func isFixedSizeType(ftype types.Type) bool {
	switch x := ftype.(type) {
	case *types.Basic:
		return true

	case *types.Struct:
		for i := range x.NumFields() {
			if !isFixedSizeType(x.Field(i).Type()) {
				return false
			}
		}

	case *types.Array:
		return isFixedSizeType(x.Elem())

	case *types.Slice:
		return false

	case *types.Alias:
		tas := x.TypeArgs()
		for i := range tas.Len() {
			if _, ok := tas.At(i).(*types.TypeParam); ok {
				return false
			}
			if !isFixedSizeType(tas.At(i)) {
				return false
			}
		}
		return true

	case *types.Named:
		tas := x.TypeArgs()
		for i := range tas.Len() {
			if _, ok := tas.At(i).(*types.TypeParam); ok {
				return false
			}
			if !isFixedSizeType(tas.At(i)) {
				return false
			}
		}
		return true
	}
	return false
}

func isValidNamedType(ftype types.Type) bool {
	if _, ok := ftype.(*types.Basic); ok {
		return true
	}
	if named, ok := ftype.(*types.Named); ok {
		if named.TypeArgs() == nil {
			return true
		}
		return false
	}
	if alias, ok := ftype.(*types.Alias); ok {
		if alias.TypeArgs() == nil {
			return true
		}
		return false
	}
	if tp, ok := ftype.(*types.TypeParam); ok {
		if named, ok := tp.Constraint().(*types.Named); ok {
			tn := named.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" {
				return true
			}
			return false
		}

		if alias, ok := tp.Constraint().(*types.Alias); ok {
			tn := alias.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" {
				return true
			}
			return false
		}
	}
	return false
}

func isBasicType(ftype types.Type) bool {
	if _, ok := ftype.Underlying().(*types.Basic); ok {
		return true
	}
	if tp, ok := ftype.(*types.TypeParam); ok {
		if named, ok := tp.Constraint().(*types.Named); ok {
			tn := named.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Number" {
				return true
			}
		}
	}
	return false
}

func isFixedSizeBasicType(ftype types.Type) bool {
	if !isBasicType(ftype) {
		return false
	}
	if basic, ok := ftype.Underlying().(*types.Basic); ok {
		_, ok := fixedBasicTypeMap[basic.Kind()]
		return ok
	}
	if _, ok := ftype.(*types.TypeParam); ok {
		return true
	}
	return false
}

func isStructType(ftype types.Type) bool {
	if _, ok := ftype.(*types.Struct); ok {
		return true
	}
	if _, ok := ftype.Underlying().(*types.Struct); ok {
		return true
	}
	if tp, ok := ftype.(*types.TypeParam); ok {
		if named, ok := tp.Constraint().(*types.Named); ok {
			tn := named.Obj()
			if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Struct" {
				return true
			}
		}
	}
	return false
}

func isArrayType(ftype types.Type) bool {
	_, ok := ftype.Underlying().(*types.Array)
	return ok
}

func isSliceType(ftype types.Type) bool {
	_, ok := ftype.Underlying().(*types.Slice)
	return ok
}
