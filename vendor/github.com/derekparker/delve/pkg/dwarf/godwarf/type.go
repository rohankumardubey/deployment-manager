// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// DWARF type information structures.
// The format is heavily biased toward C, but for simplicity
// the String methods use a pseudo-Go syntax.

// Borrowed from golang.org/x/debug/dwarf/type.go

package godwarf

import (
	"debug/dwarf"
	"fmt"
	"reflect"
	"strconv"

	"github.com/derekparker/delve/pkg/dwarf/op"
	"github.com/derekparker/delve/pkg/dwarf/util"
)

const (
	AttrGoKind          dwarf.Attr = 0x2900
	AttrGoKey           dwarf.Attr = 0x2901
	AttrGoElem          dwarf.Attr = 0x2902
	AttrGoEmbeddedField dwarf.Attr = 0x2903
	AttrGoRuntimeType   dwarf.Attr = 0x2904
)

// Basic type encodings -- the value for AttrEncoding in a TagBaseType Entry.
const (
	encAddress        = 0x01
	encBoolean        = 0x02
	encComplexFloat   = 0x03
	encFloat          = 0x04
	encSigned         = 0x05
	encSignedChar     = 0x06
	encUnsigned       = 0x07
	encUnsignedChar   = 0x08
	encImaginaryFloat = 0x09
)

// A Type conventionally represents a pointer to any of the
// specific Type structures (CharType, StructType, etc.).
//TODO: remove this use dwarf.Type
type Type interface {
	Common() *CommonType
	String() string
	Size() int64
}

// A CommonType holds fields common to multiple types.
// If a field is not known or not applicable for a given type,
// the zero value is used.
type CommonType struct {
	ByteSize    int64        // size of value of this type, in bytes
	Name        string       // name that can be used to refer to type
	ReflectKind reflect.Kind // the reflect kind of the type.
	Offset      dwarf.Offset // the offset at which this type was read
}

func (c *CommonType) Common() *CommonType { return c }

func (c *CommonType) Size() int64 { return c.ByteSize }

// Basic types

// A BasicType holds fields common to all basic types.
type BasicType struct {
	CommonType
	BitSize   int64
	BitOffset int64
}

func (b *BasicType) Basic() *BasicType { return b }

func (t *BasicType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "?"
}

// A CharType represents a signed character type.
type CharType struct {
	BasicType
}

// A UcharType represents an unsigned character type.
type UcharType struct {
	BasicType
}

// An IntType represents a signed integer type.
type IntType struct {
	BasicType
}

// A UintType represents an unsigned integer type.
type UintType struct {
	BasicType
}

// A FloatType represents a floating point type.
type FloatType struct {
	BasicType
}

// A ComplexType represents a complex floating point type.
type ComplexType struct {
	BasicType
}

// A BoolType represents a boolean type.
type BoolType struct {
	BasicType
}

// An AddrType represents a machine address type.
type AddrType struct {
	BasicType
}

// An UnspecifiedType represents an implicit, unknown, ambiguous or nonexistent type.
type UnspecifiedType struct {
	BasicType
}

// qualifiers

// A QualType represents a type that has the C/C++ "const", "restrict", or "volatile" qualifier.
type QualType struct {
	CommonType
	Qual string
	Type Type
}

func (t *QualType) String() string { return t.Qual + " " + t.Type.String() }

func (t *QualType) Size() int64 { return t.Type.Size() }

// An ArrayType represents a fixed size array type.
type ArrayType struct {
	CommonType
	Type          Type
	StrideBitSize int64 // if > 0, number of bits to hold each element
	Count         int64 // if == -1, an incomplete array, like char x[].
}

func (t *ArrayType) String() string {
	return "[" + strconv.FormatInt(t.Count, 10) + "]" + t.Type.String()
}

func (t *ArrayType) Size() int64 { return t.Count * t.Type.Size() }

// A VoidType represents the C void type.
type VoidType struct {
	CommonType
}

func (t *VoidType) String() string { return "void" }

// A PtrType represents a pointer type.
type PtrType struct {
	CommonType
	Type Type
}

func (t *PtrType) String() string { return "*" + t.Type.String() }

// A StructType represents a struct, union, or C++ class type.
type StructType struct {
	CommonType
	StructName string
	Kind       string // "struct", "union", or "class".
	Field      []*StructField
	Incomplete bool // if true, struct, union, class is declared but not defined
}

// A StructField represents a field in a struct, union, or C++ class type.
type StructField struct {
	Name       string
	Type       Type
	ByteOffset int64
	ByteSize   int64
	BitOffset  int64 // within the ByteSize bytes at ByteOffset
	BitSize    int64 // zero if not a bit field
	Embedded   bool
}

func (t *StructType) String() string {
	if t.StructName != "" {
		return t.Kind + " " + t.StructName
	}
	return t.Defn()
}

func (t *StructType) Defn() string {
	s := t.Kind
	if t.StructName != "" {
		s += " " + t.StructName
	}
	if t.Incomplete {
		s += " /*incomplete*/"
		return s
	}
	s += " {"
	for i, f := range t.Field {
		if i > 0 {
			s += "; "
		}
		s += f.Name + " " + f.Type.String()
		s += "@" + strconv.FormatInt(f.ByteOffset, 10)
		if f.BitSize > 0 {
			s += " : " + strconv.FormatInt(f.BitSize, 10)
			s += "@" + strconv.FormatInt(f.BitOffset, 10)
		}
	}
	s += "}"
	return s
}

// A SliceType represents a Go slice type. It looks like a StructType, describing
// the runtime-internal structure, with extra fields.
type SliceType struct {
	StructType
	ElemType Type
}

func (t *SliceType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "[]" + t.ElemType.String()
}

// A StringType represents a Go string type. It looks like a StructType, describing
// the runtime-internal structure, but we wrap it for neatness.
type StringType struct {
	StructType
}

func (t *StringType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "string"
}

// An InterfaceType represents a Go interface.
type InterfaceType struct {
	TypedefType
}

func (t *InterfaceType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "Interface"
}

// An EnumType represents an enumerated type.
// The only indication of its native integer type is its ByteSize
// (inside CommonType).
type EnumType struct {
	CommonType
	EnumName string
	Val      []*EnumValue
}

// An EnumValue represents a single enumeration value.
type EnumValue struct {
	Name string
	Val  int64
}

func (t *EnumType) String() string {
	s := "enum"
	if t.EnumName != "" {
		s += " " + t.EnumName
	}
	s += " {"
	for i, v := range t.Val {
		if i > 0 {
			s += "; "
		}
		s += v.Name + "=" + strconv.FormatInt(v.Val, 10)
	}
	s += "}"
	return s
}

// A FuncType represents a function type.
type FuncType struct {
	CommonType
	ReturnType Type
	ParamType  []Type
}

func (t *FuncType) String() string {
	s := "func("
	for i, t := range t.ParamType {
		if i > 0 {
			s += ", "
		}
		s += t.String()
	}
	s += ")"
	if t.ReturnType != nil {
		s += " " + t.ReturnType.String()
	}
	return s
}

// A DotDotDotType represents the variadic ... function parameter.
type DotDotDotType struct {
	CommonType
}

func (t *DotDotDotType) String() string { return "..." }

// A TypedefType represents a named type.
type TypedefType struct {
	CommonType
	Type Type
}

func (t *TypedefType) String() string { return t.Name }

func (t *TypedefType) Size() int64 { return t.Type.Size() }

// A MapType represents a Go map type. It looks like a TypedefType, describing
// the runtime-internal structure, with extra fields.
type MapType struct {
	TypedefType
	KeyType  Type
	ElemType Type
}

func (t *MapType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "map[" + t.KeyType.String() + "]" + t.ElemType.String()
}

// A ChanType represents a Go channel type.
type ChanType struct {
	TypedefType
	ElemType Type
}

func (t *ChanType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "chan " + t.ElemType.String()
}

// Type reads the type at off in the DWARF ``info'' section.
func ReadType(d *dwarf.Data, off dwarf.Offset, typeCache map[dwarf.Offset]Type) (Type, error) {
	return readType(d, "info", d.Reader(), off, typeCache)
}

func getKind(e *dwarf.Entry) reflect.Kind {
	integer, _ := e.Val(AttrGoKind).(int64)
	return reflect.Kind(integer)
}

// readType reads a type from r at off of name using and updating a
// type cache.
func readType(d *dwarf.Data, name string, r *dwarf.Reader, off dwarf.Offset, typeCache map[dwarf.Offset]Type) (Type, error) {
	if t, ok := typeCache[off]; ok {
		return t, nil
	}
	r.Seek(off)
	e, err := r.Next()
	if err != nil {
		return nil, err
	}
	addressSize := r.AddressSize()
	if e == nil || e.Offset != off {
		return nil, dwarf.DecodeError{name, off, "no type at offset"}
	}

	// Parse type from dwarf.Entry.
	// Must always set typeCache[off] before calling
	// d.Type recursively, to handle circular types correctly.
	var typ Type

	nextDepth := 0

	// Get next child; set err if error happens.
	next := func() *dwarf.Entry {
		if !e.Children {
			return nil
		}
		// Only return direct children.
		// Skip over composite entries that happen to be nested
		// inside this one. Most DWARF generators wouldn't generate
		// such a thing, but clang does.
		// See golang.org/issue/6472.
		for {
			kid, err1 := r.Next()
			if err1 != nil {
				err = err1
				return nil
			}
			if kid.Tag == 0 {
				if nextDepth > 0 {
					nextDepth--
					continue
				}
				return nil
			}
			if kid.Children {
				nextDepth++
			}
			if nextDepth > 0 {
				continue
			}
			return kid
		}
	}

	// Get Type referred to by dwarf.Entry's attr.
	// Set err if error happens.  Not having a type is an error.
	typeOf := func(e *dwarf.Entry, attr dwarf.Attr) Type {
		tval := e.Val(attr)
		var t Type
		switch toff := tval.(type) {
		case dwarf.Offset:
			if t, err = readType(d, name, d.Reader(), toff, typeCache); err != nil {
				return nil
			}
		case uint64:
			err = dwarf.DecodeError{name, e.Offset, "DWARFv4 section debug_types unsupported"}
			return nil
		default:
			// It appears that no Type means "void".
			return new(VoidType)
		}
		return t
	}

	switch e.Tag {
	case dwarf.TagArrayType:
		// Multi-dimensional array.  (DWARF v2 ??5.4)
		// Attributes:
		//	AttrType:subtype [required]
		//	AttrStrideSize: distance in bits between each element of the array
		//	AttrStride: distance in bytes between each element of the array
		//	AttrByteSize: size of entire array
		// Children:
		//	TagSubrangeType or TagEnumerationType giving one dimension.
		//	dimensions are in left to right order.
		t := new(ArrayType)
		t.Name, _ = e.Val(dwarf.AttrName).(string)
		t.ReflectKind = getKind(e)
		typ = t
		typeCache[off] = t
		if t.Type = typeOf(e, dwarf.AttrType); err != nil {
			goto Error
		}
		if bytes, ok := e.Val(dwarf.AttrStride).(int64); ok {
			t.StrideBitSize = 8 * bytes
		} else if bits, ok := e.Val(dwarf.AttrStrideSize).(int64); ok {
			t.StrideBitSize = bits
		} else {
			// If there's no stride specified, assume it's the size of the
			// array's element type.
			t.StrideBitSize = 8 * t.Type.Size()
		}

		// Accumulate dimensions,
		ndim := 0
		for kid := next(); kid != nil; kid = next() {
			// TODO(rsc): Can also be TagEnumerationType
			// but haven't seen that in the wild yet.
			switch kid.Tag {
			case dwarf.TagSubrangeType:
				count, ok := kid.Val(dwarf.AttrCount).(int64)
				if !ok {
					// Old binaries may have an upper bound instead.
					count, ok = kid.Val(dwarf.AttrUpperBound).(int64)
					if ok {
						count++ // Length is one more than upper bound.
					} else {
						count = -1 // As in x[].
					}
				}
				if ndim == 0 {
					t.Count = count
				} else {
					// Multidimensional array.
					// Create new array type underneath this one.
					t.Type = &ArrayType{Type: t.Type, Count: count}
				}
				ndim++
			case dwarf.TagEnumerationType:
				err = dwarf.DecodeError{name, kid.Offset, "cannot handle enumeration type as array bound"}
				goto Error
			}
		}
		if ndim == 0 {
			// LLVM generates this for x[].
			t.Count = -1
		}

	case dwarf.TagBaseType:
		// Basic type.  (DWARF v2 ??5.1)
		// Attributes:
		//	AttrName: name of base type in programming language of the compilation unit [required]
		//	AttrEncoding: encoding value for type (encFloat etc) [required]
		//	AttrByteSize: size of type in bytes [required]
		//	AttrBitOffset: for sub-byte types, size in bits
		//	AttrBitSize: for sub-byte types, bit offset of high order bit in the AttrByteSize bytes
		name, _ := e.Val(dwarf.AttrName).(string)
		enc, ok := e.Val(dwarf.AttrEncoding).(int64)
		if !ok {
			err = dwarf.DecodeError{name, e.Offset, "missing encoding attribute for " + name}
			goto Error
		}
		switch enc {
		default:
			err = dwarf.DecodeError{name, e.Offset, "unrecognized encoding attribute value"}
			goto Error

		case encAddress:
			typ = new(AddrType)
		case encBoolean:
			typ = new(BoolType)
		case encComplexFloat:
			typ = new(ComplexType)
			if name == "complex" {
				// clang writes out 'complex' instead of 'complex float' or 'complex double'.
				// clang also writes out a byte size that we can use to distinguish.
				// See issue 8694.
				switch byteSize, _ := e.Val(dwarf.AttrByteSize).(int64); byteSize {
				case 8:
					name = "complex float"
				case 16:
					name = "complex double"
				}
			}
		case encFloat:
			typ = new(FloatType)
		case encSigned:
			typ = new(IntType)
		case encUnsigned:
			typ = new(UintType)
		case encSignedChar:
			typ = new(CharType)
		case encUnsignedChar:
			typ = new(UcharType)
		}
		typeCache[off] = typ
		t := typ.(interface {
			Basic() *BasicType
		}).Basic()
		t.Name = name
		t.BitSize, _ = e.Val(dwarf.AttrBitSize).(int64)
		t.BitOffset, _ = e.Val(dwarf.AttrBitOffset).(int64)
		t.ReflectKind = getKind(e)

	case dwarf.TagClassType, dwarf.TagStructType, dwarf.TagUnionType:
		// Structure, union, or class type.  (DWARF v2 ??5.5)
		// Also Slices and Strings (Go-specific).
		// Attributes:
		//	AttrName: name of struct, union, or class
		//	AttrByteSize: byte size [required]
		//	AttrDeclaration: if true, struct/union/class is incomplete
		// 	AttrGoElem: present for slices only.
		// Children:
		//	TagMember to describe one member.
		//		AttrName: name of member [required]
		//		AttrType: type of member [required]
		//		AttrByteSize: size in bytes
		//		AttrBitOffset: bit offset within bytes for bit fields
		//		AttrBitSize: bit size for bit fields
		//		AttrDataMemberLoc: location within struct [required for struct, class]
		// There is much more to handle C++, all ignored for now.
		t := new(StructType)
		t.ReflectKind = getKind(e)
		switch t.ReflectKind {
		case reflect.Slice:
			slice := new(SliceType)
			slice.ElemType = typeOf(e, AttrGoElem)
			t = &slice.StructType
			typ = slice
		case reflect.String:
			str := new(StringType)
			t = &str.StructType
			typ = str
		default:
			typ = t
		}
		typeCache[off] = typ
		switch e.Tag {
		case dwarf.TagClassType:
			t.Kind = "class"
		case dwarf.TagStructType:
			t.Kind = "struct"
		case dwarf.TagUnionType:
			t.Kind = "union"
		}
		t.Name, _ = e.Val(dwarf.AttrName).(string)
		t.StructName, _ = e.Val(dwarf.AttrName).(string)
		t.Incomplete = e.Val(dwarf.AttrDeclaration) != nil
		t.Field = make([]*StructField, 0, 8)
		var lastFieldType Type
		var lastFieldBitOffset int64
		for kid := next(); kid != nil; kid = next() {
			if kid.Tag == dwarf.TagMember {
				f := new(StructField)
				if f.Type = typeOf(kid, dwarf.AttrType); err != nil {
					goto Error
				}
				switch loc := kid.Val(dwarf.AttrDataMemberLoc).(type) {
				case []byte:
					// TODO: Should have original compilation
					// unit here, not unknownFormat.
					if len(loc) == 0 {
						// Empty exprloc. f.ByteOffset=0.
						break
					}
					b := util.MakeBuf(d, util.UnknownFormat{}, "location", 0, loc)
					op_ := op.Opcode(b.Uint8())
					switch op_ {
					case op.DW_OP_plus_uconst:
						// Handle opcode sequence [DW_OP_plus_uconst <uleb128>]
						f.ByteOffset = int64(b.Uint())
						b.AssertEmpty()
					case op.DW_OP_consts:
						// Handle opcode sequence [DW_OP_consts <sleb128> DW_OP_plus]
						f.ByteOffset = b.Int()
						op_ = op.Opcode(b.Uint8())
						if op_ != op.DW_OP_plus {
							err = dwarf.DecodeError{name, kid.Offset, fmt.Sprintf("unexpected opcode 0x%x", op_)}
							goto Error
						}
						b.AssertEmpty()
					default:
						err = dwarf.DecodeError{name, kid.Offset, fmt.Sprintf("unexpected opcode 0x%x", op_)}
						goto Error
					}
					if b.Err != nil {
						err = b.Err
						goto Error
					}
				case int64:
					f.ByteOffset = loc
				}

				haveBitOffset := false
				f.Name, _ = kid.Val(dwarf.AttrName).(string)
				f.ByteSize, _ = kid.Val(dwarf.AttrByteSize).(int64)
				f.BitOffset, haveBitOffset = kid.Val(dwarf.AttrBitOffset).(int64)
				f.BitSize, _ = kid.Val(dwarf.AttrBitSize).(int64)
				f.Embedded, _ = kid.Val(AttrGoEmbeddedField).(bool)
				t.Field = append(t.Field, f)

				bito := f.BitOffset
				if !haveBitOffset {
					bito = f.ByteOffset * 8
				}
				if bito == lastFieldBitOffset && t.Kind != "union" {
					// Last field was zero width.  Fix array length.
					// (DWARF writes out 0-length arrays as if they were 1-length arrays.)
					zeroArray(lastFieldType)
				}
				lastFieldType = f.Type
				lastFieldBitOffset = bito
			}
		}
		if t.Kind != "union" {
			b, ok := e.Val(dwarf.AttrByteSize).(int64)
			if ok && b*8 == lastFieldBitOffset {
				// Final field must be zero width.  Fix array length.
				zeroArray(lastFieldType)
			}
		}

	case dwarf.TagConstType, dwarf.TagVolatileType, dwarf.TagRestrictType:
		// Type modifier (DWARF v2 ??5.2)
		// Attributes:
		//	AttrType: subtype
		t := new(QualType)
		t.Name, _ = e.Val(dwarf.AttrName).(string)
		t.ReflectKind = getKind(e)
		typ = t
		typeCache[off] = t
		if t.Type = typeOf(e, dwarf.AttrType); err != nil {
			goto Error
		}
		switch e.Tag {
		case dwarf.TagConstType:
			t.Qual = "const"
		case dwarf.TagRestrictType:
			t.Qual = "restrict"
		case dwarf.TagVolatileType:
			t.Qual = "volatile"
		}

	case dwarf.TagEnumerationType:
		// Enumeration type (DWARF v2 ??5.6)
		// Attributes:
		//	AttrName: enum name if any
		//	AttrByteSize: bytes required to represent largest value
		// Children:
		//	TagEnumerator:
		//		AttrName: name of constant
		//		AttrConstValue: value of constant
		t := new(EnumType)
		t.ReflectKind = getKind(e)
		typ = t
		typeCache[off] = t
		t.Name, _ = e.Val(dwarf.AttrName).(string)
		t.EnumName, _ = e.Val(dwarf.AttrName).(string)
		t.Val = make([]*EnumValue, 0, 8)
		for kid := next(); kid != nil; kid = next() {
			if kid.Tag == dwarf.TagEnumerator {
				f := new(EnumValue)
				f.Name, _ = kid.Val(dwarf.AttrName).(string)
				f.Val, _ = kid.Val(dwarf.AttrConstValue).(int64)
				n := len(t.Val)
				if n >= cap(t.Val) {
					val := make([]*EnumValue, n, n*2)
					copy(val, t.Val)
					t.Val = val
				}
				t.Val = t.Val[0 : n+1]
				t.Val[n] = f
			}
		}

	case dwarf.TagPointerType:
		// Type modifier (DWARF v2 ??5.2)
		// Attributes:
		//	AttrType: subtype [not required!  void* has no AttrType]
		//	AttrAddrClass: address class [ignored]
		t := new(PtrType)
		t.Name, _ = e.Val(dwarf.AttrName).(string)
		t.ReflectKind = getKind(e)
		typ = t
		typeCache[off] = t
		if e.Val(dwarf.AttrType) == nil {
			t.Type = &VoidType{}
			break
		}
		t.Type = typeOf(e, dwarf.AttrType)

	case dwarf.TagSubroutineType:
		// Subroutine type.  (DWARF v2 ??5.7)
		// Attributes:
		//	AttrType: type of return value if any
		//	AttrName: possible name of type [ignored]
		//	AttrPrototyped: whether used ANSI C prototype [ignored]
		// Children:
		//	TagFormalParameter: typed parameter
		//		AttrType: type of parameter
		//	TagUnspecifiedParameter: final ...
		t := new(FuncType)
		t.Name, _ = e.Val(dwarf.AttrName).(string)
		t.ReflectKind = getKind(e)
		typ = t
		typeCache[off] = t
		if t.ReturnType = typeOf(e, dwarf.AttrType); err != nil {
			goto Error
		}
		t.ParamType = make([]Type, 0, 8)
		for kid := next(); kid != nil; kid = next() {
			var tkid Type
			switch kid.Tag {
			default:
				continue
			case dwarf.TagFormalParameter:
				if tkid = typeOf(kid, dwarf.AttrType); err != nil {
					goto Error
				}
			case dwarf.TagUnspecifiedParameters:
				tkid = &DotDotDotType{}
			}
			t.ParamType = append(t.ParamType, tkid)
		}

	case dwarf.TagTypedef:
		// Typedef (DWARF v2 ??5.3)
		// Also maps and channels (Go-specific).
		// Attributes:
		//	AttrName: name [required]
		//	AttrType: type definition [required]
		//	AttrGoKey: present for maps.
		//	AttrGoElem: present for maps and channels.
		t := new(TypedefType)
		t.ReflectKind = getKind(e)
		switch t.ReflectKind {
		case reflect.Map:
			m := new(MapType)
			m.KeyType = typeOf(e, AttrGoKey)
			m.ElemType = typeOf(e, AttrGoElem)
			t = &m.TypedefType
			typ = m
		case reflect.Chan:
			c := new(ChanType)
			c.ElemType = typeOf(e, AttrGoElem)
			t = &c.TypedefType
			typ = c
		case reflect.Interface:
			it := new(InterfaceType)
			t = &it.TypedefType
			typ = it
		default:
			typ = t
		}
		typeCache[off] = typ
		t.Name, _ = e.Val(dwarf.AttrName).(string)
		t.Type = typeOf(e, dwarf.AttrType)

	case dwarf.TagUnspecifiedType:
		// Unspecified type (DWARF v3 ??5.2)
		// Attributes:
		//      AttrName: name
		t := new(UnspecifiedType)
		typ = t
		typeCache[off] = t
		t.Name, _ = e.Val(dwarf.AttrName).(string)
	}

	if err != nil {
		goto Error
	}

	typ.Common().Offset = off

	{
		b, ok := e.Val(dwarf.AttrByteSize).(int64)
		if !ok {
			b = -1
			switch t := typ.(type) {
			case *TypedefType:
				b = t.Type.Size()
			case *MapType:
				b = t.Type.Size()
			case *ChanType:
				b = t.Type.Size()
			case *InterfaceType:
				b = t.Type.Size()
			case *PtrType:
				b = int64(addressSize)
			case *FuncType:
				// on Go < 1.10 function types do not have a DW_AT_byte_size attribute.
				b = int64(addressSize)
			}
		}
		typ.Common().ByteSize = b
	}
	return typ, nil

Error:
	// If the parse fails, take the type out of the cache
	// so that the next call with this offset doesn't hit
	// the cache and return success.
	delete(typeCache, off)
	return nil, err
}

func zeroArray(t Type) {
	for {
		at, ok := t.(*ArrayType)
		if !ok {
			break
		}
		at.Count = 0
		t = at.Type
	}
}

func resolveTypedef(typ Type) Type {
	for {
		if tt, ok := typ.(*TypedefType); ok {
			typ = tt.Type
		} else {
			return typ
		}
	}
}
