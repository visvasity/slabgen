// Copyright (c) 2025 Visvasity LLC

package typecheck

import (
	"fmt"
	"go/types"
)

type TypeArgData struct {
	Name    string
	PkgPath string
	PkgName string
}

type TypeParamData struct {
	Name       string
	Constraint string
	PkgPath    string
	PkgName    string
}

type StructData struct {
	StructName string

	PkgPath string
	PkgName string

	TypeParams []*TypeParamData

	Fields []*FieldData
}

type FieldData struct {
	Index     int
	FieldName string

	TypePkgPath string
	TypePkgName string

	Kind string // One of [basic|struct|array|slice]

	TypeName string
	TypeArgs []*TypeArgData

	TypeParams []*TypeParamData

	BasicKind string // One of [generic|int|uint|int8|uint8|int16|uint16|...]

	StructKind string // One of [generic|struct]

	ArrayLens []int64
	ArrayKind string // One of [basic|struct]

	SliceKind string // One of [basic|struct]
}

type Checker struct {
	structDataMap map[string]*StructData
}

func New() *Checker {
	return &Checker{
		structDataMap: make(map[string]*StructData),
	}
}

func (c *Checker) StructDataMap() map[string]*StructData {
	return c.structDataMap
}

func typenameKey(tn *types.TypeName) string {
	pkg := tn.Pkg()
	if pkg == nil {
		return tn.Name()
	}
	return pkg.Path() + "." + tn.Name()
}

func underlyingKind(v types.Type) (string, error) {
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

// Check scans the input data type and prepares structDataMap with the
// metadata.
func (c *Checker) Check(typename *types.TypeName) error {
	if err := c.collectFields(typename); err != nil {
		return err
	}
	return nil
}

func (c *Checker) collectFields(typename *types.TypeName) error {
	stype, ok := typename.Type().Underlying().(*types.Struct)
	if !ok {
		return fmt.Errorf("underlying type is not a struct")
	}

	sdata := &StructData{
		StructName: typename.Name(),
		PkgName:    typename.Pkg().Name(),
		PkgPath:    typename.Pkg().Path(),
	}
	if snamed, ok := typename.Type().(nameOrAlias); ok {
		if tps := snamed.TypeParams(); tps != nil {
			for i := range tps.Len() {
				tpdata, err := getTypeParamData(tps.At(i))
				if err != nil {
					return err
				}
				sdata.TypeParams = append(sdata.TypeParams, tpdata)
			}
		}
	}

	for i := 0; i < stype.NumFields(); i++ {
		fdata := &FieldData{Index: i}
		if err := c.collectField(stype, stype.Field(i), fdata); err != nil {
			return err
		}
		sdata.Fields = append(sdata.Fields, fdata)
	}

	c.structDataMap[typename.Name()] = sdata
	return nil
}

func (c *Checker) collectField(stype *types.Struct, field *types.Var, fdata *FieldData) error {
	ftype := field.Type()
	futype := ftype.Underlying()

	fdata.FieldName = field.Name()

	switch {
	case isBasicType(ftype):
		fdata.Kind = "basic"
		return c.collectBasicField(field, fdata, ftype)
	case isStructType(ftype):
		fdata.Kind = "struct"
		return c.collectStructField(field, fdata, ftype)
	case isArrayType(ftype):
		fdata.Kind = "array"
		return c.collectArrayField(field, fdata, ftype)
	case isSliceType(ftype):
		fdata.Kind = "slice"
		return c.collectSliceField(field, fdata, ftype)
	}

	return fmt.Errorf("collectField: field (%v) of type %T (underlying=%T) is not supported", field, ftype, futype)
}

func (c *Checker) collectBasicField(field *types.Var, fdata *FieldData, xtype types.Type) error {
	switch x := xtype.(type) {
	case *types.Basic:
		fdata.TypeName = x.Name()
		fdata.BasicKind = x.Name()
		return nil

	case *types.TypeParam:
		fdata.TypeName = x.Obj().Name()
		fdata.BasicKind = "generic"
		tpdata, err := getTypeParamData(x)
		if err != nil {
			return err
		}
		fdata.TypeParams = append(fdata.TypeParams, tpdata)
		return nil

	case *types.Named:
		xutype, ok := x.Underlying().(*types.Basic)
		if !ok {
			return fmt.Errorf("collectBasicField: field=(%v): underlying type=%T is not a basic", field, xutype)
		}
		if err := collectTypeArgsConstraints(x, fdata); err != nil {
			return err
		}
		fdata.TypeName = x.Obj().Name()
		if pkg := x.Obj().Pkg(); pkg != nil {
			fdata.TypePkgPath = pkg.Path()
			fdata.TypePkgName = pkg.Name()
		}
		fdata.BasicKind = xutype.Name()
		if len(fdata.TypeArgs) != 0 {
			fdata.BasicKind = "generic"
		}
		return nil

	case *types.Alias:
		xutype, ok := x.Underlying().(*types.Basic)
		if !ok {
			return fmt.Errorf("collectBasicField: field=(%v): underlying type=%T is not a basic", field, xutype)
		}
		if err := collectTypeArgsConstraints(x, fdata); err != nil {
			return err
		}
		fdata.TypeName = x.Obj().Name()
		if pkg := x.Obj().Pkg(); pkg != nil {
			fdata.TypePkgPath = pkg.Path()
			fdata.TypePkgName = pkg.Name()
		}
		fdata.BasicKind = xutype.Name()
		if len(fdata.TypeArgs) != 0 {
			fdata.BasicKind = "generic"
		}
		return nil
	}
	return fmt.Errorf("collectBasicField: field=(%v): type %v (underlying=%T) is not supported", field, xtype, xtype.Underlying())
}

func (c *Checker) collectStructField(field *types.Var, fdata *FieldData, xtype types.Type) error {
	switch x := xtype.(type) {
	case *types.TypeParam:
		fdata.StructKind = "generic"
		fdata.TypeName = x.Obj().Name()
		tpdata, err := getTypeParamData(x)
		if err != nil {
			return err
		}
		fdata.TypeParams = append(fdata.TypeParams, tpdata)
		return nil

	case *types.Named:
		if err := c.Check(x.Obj()); err != nil {
			return err
		}
		if err := collectTypeArgsConstraints(x, fdata); err != nil {
			return err
		}
		fdata.TypeName = x.Obj().Name()
		if pkg := x.Obj().Pkg(); pkg != nil {
			fdata.TypePkgPath = pkg.Path()
			fdata.TypePkgName = pkg.Name()
		}
		fdata.StructKind = "struct"
		if len(fdata.TypeArgs) != 0 {
			fdata.StructKind = "generic"
		}
		return nil

	case *types.Alias:
		if err := c.Check(x.Obj()); err != nil {
			return err
		}
		if err := collectTypeArgsConstraints(x, fdata); err != nil {
			return err
		}
		fdata.TypeName = x.Obj().Name()
		if pkg := x.Obj().Pkg(); pkg != nil {
			fdata.TypePkgPath = pkg.Path()
			fdata.TypePkgName = pkg.Name()
		}
		fdata.StructKind = "struct"
		if len(fdata.TypeArgs) != 0 {
			fdata.StructKind = "generic"
		}
		return nil
	}
	return fmt.Errorf("collectStructField: field=(%v): type %v (underlying=%T) is not supported", field, xtype, xtype.Underlying())
}

func (c *Checker) collectArrayField(field *types.Var, fdata *FieldData, xtype types.Type) error {
	array, ok := xtype.Underlying().(*types.Array)
	if !ok {
		return fmt.Errorf("collectArrayField: field (%v) is not a array type", field)
	}

	etype := array.Elem()
	switch {
	case isBasicType(etype):
		fdata.ArrayKind = "basic"
		fdata.ArrayLens = append(fdata.ArrayLens, array.Len())
		return c.collectBasicField(field, fdata, etype)
	case isStructType(etype):
		fdata.ArrayKind = "struct"
		fdata.ArrayLens = append(fdata.ArrayLens, array.Len())
		return c.collectStructField(field, fdata, etype)
	case isArrayType(etype):
		fdata.ArrayLens = append(fdata.ArrayLens, array.Len())
		return c.collectArrayField(field, fdata, etype)
	}

	return fmt.Errorf("collectArrayField: field=(%v): type %v (underlying=%T) is not supported", field, xtype, xtype.Underlying())
}

func (c *Checker) collectSliceField(field *types.Var, fdata *FieldData, xtype types.Type) error {
	slice, ok := xtype.Underlying().(*types.Slice)
	if !ok {
		return fmt.Errorf("collectSliceField: field (%v) is not a slice type", field)
	}

	etype := slice.Elem()
	switch {
	case isBasicType(etype):
		fdata.SliceKind = "basic"
		return c.collectBasicField(field, fdata, etype)
	case isStructType(etype):
		fdata.SliceKind = "struct"
		return c.collectStructField(field, fdata, etype)
	}

	return fmt.Errorf("collectSliceField: field=(%v): type %v (underlying=%T) is not supported", field, xtype, xtype.Underlying())
}

type nameOrAlias interface {
	TypeArgs() *types.TypeList
	TypeParams() *types.TypeParamList
}

func getTypeNameNames(tn *types.TypeName) (name, pkgPath, pkgName string) {
	vs := [3]string{tn.Name()}
	if pkg := tn.Pkg(); pkg != nil {
		vs[1] = pkg.Path()
		vs[2] = pkg.Name()
	}
	return vs[0], vs[1], vs[2]
}

func getTypeParamData(tp *types.TypeParam) (*TypeParamData, error) {
	if x, ok := tp.Constraint().(*types.Named); ok {
		a, b, c := getTypeNameNames(x.Obj())
		return &TypeParamData{
			Name:       tp.Obj().Name(),
			Constraint: a,
			PkgPath:    b,
			PkgName:    c,
		}, nil
	}
	if x, ok := tp.Constraint().(*types.Alias); ok {
		a, b, c := getTypeNameNames(x.Obj())
		return &TypeParamData{
			Name:       tp.Obj().Name(),
			Constraint: a,
			PkgPath:    b,
			PkgName:    c,
		}, nil
	}
	panic(fmt.Errorf("getTypeParamData: type param (%v) with constraint of type %T is not supported", tp, tp.Constraint()))
	return nil, fmt.Errorf("getTypeParamData: type param (%v) with constraint of type %T is not supported", tp, tp.Constraint())
}

func getTypeArgName(ftype types.Type) (*TypeArgData, error) {
	switch x := ftype.(type) {
	case *types.Basic:
		return &TypeArgData{Name: x.Name()}, nil
	case *types.TypeParam:
		return &TypeArgData{Name: x.Obj().Name()}, nil
	case *types.Named:
		v := &TypeArgData{Name: x.Obj().Name()}
		if pkg := x.Obj().Pkg(); pkg != nil {
			v.PkgPath = pkg.Path()
			v.PkgName = pkg.Name()
		}
		return v, nil
	case *types.Alias:
		v := &TypeArgData{Name: x.Obj().Name()}
		if pkg := x.Obj().Pkg(); pkg != nil {
			v.PkgPath = pkg.Path()
			v.PkgName = pkg.Name()
		}
		return v, nil
	}
	return nil, fmt.Errorf("getTypeArgName: type argument (%v) type is not supported", ftype)
}

func collectTypeArgsConstraints(ftype nameOrAlias, fdata *FieldData) error {
	if tas := ftype.TypeArgs(); tas != nil {
		for i := range tas.Len() {
			targ := tas.At(i)
			if x, ok := targ.(nameOrAlias); ok {
				if x.TypeArgs() != nil {
					return fmt.Errorf("collectTypeArgsConstraints: type=%v: nested type args are not supported", targ)
				}
			}
			tadata, err := getTypeArgName(targ)
			if err != nil {
				return err
			}
			fdata.TypeArgs = append(fdata.TypeArgs, tadata)
		}
	}
	if tps := ftype.TypeParams(); tps != nil {
		for i := range tps.Len() {
			tpdata, err := getTypeParamData(tps.At(i))
			if err != nil {
				return err
			}
			fdata.TypeParams = append(fdata.TypeParams, tpdata)
		}
	}
	return nil
}

func isBasicType(ftype types.Type) bool {
	if _, ok := ftype.Underlying().(*types.Basic); ok {
		return true
	}
	if tp, ok := ftype.(*types.TypeParam); ok {
		return isNumberTypeParam(tp)
	}
	return false
}

func isStructType(ftype types.Type) bool {
	if _, ok := ftype.Underlying().(*types.Struct); ok {
		return true
	}
	if tp, ok := ftype.(*types.TypeParam); ok {
		return isStructTypeParam(tp)
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

func isStructTypeParam(tparam *types.TypeParam) bool {
	if named, ok := tparam.Constraint().(*types.Named); ok {
		tn := named.Obj()
		if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Struct" {
			return true
		}
	}
	return false
}

func isNumberTypeParam(tparam *types.TypeParam) bool {
	if named, ok := tparam.Constraint().(*types.Named); ok {
		tn := named.Obj()
		if tn.Pkg() != nil && tn.Pkg().Path() == "github.com/visvasity/blockgen/blockgen" && tn.Name() == "Number" {
			return true
		}
	}
	return false
}
