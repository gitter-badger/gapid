// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"errors"
	"fmt"
	"unicode"

	"reflect"

	"strings"

	"github.com/google/gapid/core/gapil/semantic"
)

var (
	// NodeTypeList exposes all the types valid as Is*name* tests
	NodeTypeList = []interface{}{
		// Primitive types
		"",    // string
		false, // bool
		// primitive value node types
		semantic.BoolValue(true),
		semantic.StringValue(""),
		semantic.Int8Value(0),
		semantic.Uint8Value(0),
		semantic.Int16Value(0),
		semantic.Uint16Value(0),
		semantic.Int32Value(0),
		semantic.Uint32Value(0),
		semantic.Int64Value(0),
		semantic.Uint64Value(0),
		semantic.Float32Value(0),
		semantic.Float64Value(0),
		// semantic node types
		semantic.Abort{},
		semantic.API{},
		semantic.ArrayAssign{},
		semantic.ArrayIndex{},
		semantic.ArrayInitializer{},
		semantic.Assert{},
		semantic.Assign{},
		semantic.BinaryOp{},
		semantic.BitTest{},
		semantic.Block{},
		semantic.Branch{},
		semantic.Builtin{},
		semantic.Call{},
		semantic.Callable{},
		semantic.Cast{},
		semantic.Choice{},
		semantic.ClassInitializer{},
		semantic.Class{},
		semantic.Clone{},
		semantic.Copy{},
		semantic.Create{},
		semantic.DeclareLocal{},
		semantic.Definition{},
		semantic.EnumEntry{},
		semantic.Enum{},
		semantic.Fence{},
		semantic.Field{},
		semantic.FieldInitializer{},
		semantic.Function{},
		semantic.Global{},
		semantic.Ignore{},
		semantic.Iteration{},
		semantic.MapIteration{},
		semantic.Label{},
		semantic.Length{},
		semantic.Local{},
		semantic.Make{},
		semantic.MapAssign{},
		semantic.MapContains{},
		semantic.MapIndex{},
		semantic.MapRemove{},
		semantic.Map{},
		semantic.Member{},
		semantic.MessageValue{},
		semantic.New{},
		semantic.Null{},
		semantic.Observed{},
		semantic.Parameter{},
		semantic.PointerRange{},
		semantic.Pointer{},
		semantic.Pseudonym{},
		semantic.Read{},
		semantic.Reference{},
		semantic.Return{},
		semantic.Select{},
		semantic.SliceAssign{},
		semantic.SliceContains{},
		semantic.SliceIndex{},
		semantic.SliceRange{},
		semantic.Slice{},
		semantic.Slice{},
		semantic.StaticArray{},
		semantic.Switch{},
		semantic.UnaryOp{},
		semantic.Unknown{},
		semantic.Write{},
		// node interface types
		(*semantic.Annotated)(nil),
		(*semantic.Expression)(nil),
		(*semantic.Type)(nil),
		(*semantic.Labeled)(nil),
		(*semantic.Owned)(nil),
	}

	nodeTypes = map[string]reflect.Type{}
)

func init() {
	for _, n := range NodeTypeList {
		nt := baseType(n)
		name := nt.Name()
		nodeTypes[name] = nt
	}
}

func initNodeTypes(f *Functions) {
	for _, b := range semantic.BuiltinTypes {
		b := b
		name := "Is" + strings.Trim(strings.Title(b.Name()), "<>")
		f.funcs[name] = func(t semantic.Type) bool {
			return t == b
		}
	}
	for name, t := range nodeTypes {
		for _, r := range name {
			if unicode.IsUpper(r) {
				f.funcs["Is"+name] = isTypeTest(t)
			}
			break
		}
	}
}

// Returns the resolved semantic type of an expression node.
func (*Functions) TypeOf(v interface{}) (semantic.Type, error) {
	if v == nil {
		return semantic.VoidType, nil
	}
	switch e := v.(type) {
	case semantic.Type:
		return e, nil
	case *semantic.Field:
		return e.Type, nil
	case semantic.Expression:
		return e.ExpressionType(), nil
	default:
		return nil, fmt.Errorf("Type \"%T\" is not an expression", v)
	}
}

// TrueTypeOf returns the resolved semantic type of an expression node, dropping all pseudonyms
func (f *Functions) TrueTypeOf(v interface{}) (semantic.Type, error) {
	t, err := f.TypeOf(v)
	if err != nil {
		return t, err
	}
	return f.Underlying(t), nil
}

// Returns true if v is one of the primitive numeric value types.
func (*Functions) IsNumericValue(v interface{}) bool {
	switch v.(type) {
	case semantic.Int8Value,
		semantic.Uint8Value,
		semantic.Int16Value,
		semantic.Uint16Value,
		semantic.Int32Value,
		semantic.Uint32Value,
		semantic.Int64Value,
		semantic.Uint64Value,
		semantic.Float32Value,
		semantic.Float64Value:
		return true
	default:
		return false
	}
}

// Returns true if t is one of the primitive numeric types.
func (*Functions) IsNumericType(t interface{}) bool {
	if _, builtin := t.(*semantic.Builtin); !builtin {
		return false
	}
	switch t {
	case semantic.Int8Type,
		semantic.Uint8Type,
		semantic.Int16Type,
		semantic.Uint16Type,
		semantic.Int32Type,
		semantic.Uint32Type,
		semantic.Int64Type,
		semantic.Uint64Type,
		semantic.Float32Type,
		semantic.Float64Type,
		semantic.SizeType:
		return true
	default:
		return false
	}
}

// Returns the base name of the type of v
func baseType(v interface{}) reflect.Type {
	ty := reflect.TypeOf(v)
	for ty != nil && ty.Kind() == reflect.Ptr {
		return ty.Elem()
	}
	return ty
}

func singleTypeTest(test reflect.Type, against reflect.Type) bool {
	if test == nil {
		if against == nil {
			return true
		}
		return false
	}
	if against == nil {
		return false
	}
	return test.AssignableTo(against)
}

func doTypeTest(v interface{}, against ...reflect.Type) bool {
	test := reflect.TypeOf(v)
	for {
		for _, t := range against {
			if singleTypeTest(test, t) {
				return true
			}
		}
		if test != nil && test.Kind() == reflect.Ptr {
			test = test.Elem()
		} else {
			return false
		}
	}
}

func isTypeTest(t reflect.Type) func(v interface{}) bool {
	return func(v interface{}) bool {
		return doTypeTest(v, t)
	}
}

// Asserts that the type of v is in the list of expected types
func (*Functions) AssertType(v interface{}, expected ...string) (string, error) {
	types := make([]reflect.Type, len(expected))
	for i, e := range expected {
		if e != "nil" {
			et, found := nodeTypes[e]
			if !found {
				return "", fmt.Errorf("%s is not a valid type", e)
			}
			types[i] = et
		}
	}
	if doTypeTest(v, types...) {
		return "", nil
	}

	msg := fmt.Sprintf("Type assertion. Got: %T, Expected: ", v)
	if c := len(expected); c > 1 {
		msg += strings.Join(expected[:c-1], ", ")
		msg += " or " + expected[c-1]
	} else {
		msg += expected[0]
	}
	return "", errors.New(msg)
}