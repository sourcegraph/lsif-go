package indexer

import (
	"bytes"
	"fmt"
	"go/types"
	"strings"
)

// indent is used to format struct fields.
const indent = "    "

// typeString returns the string representation of the given object's type.
func typeString(obj ObjectLike) (signature string, extra string) {
	switch v := obj.(type) {
	case *types.PkgName:
		return fmt.Sprintf("package %s", v.Name()), ""

	case *types.TypeName:
		return formatTypeSignature(v), formatTypeExtra(v)

	case *types.Var:
		if v.IsField() {
			// TODO(efritz) - make this be "(T).F" instead of "struct field F string"
			return fmt.Sprintf("struct %s", obj.String()), ""
		}

	case *types.Const:
		return fmt.Sprintf("%s = %s", types.ObjectString(v, packageQualifier), v.Val()), ""

	case *PkgDeclaration:
		return fmt.Sprintf("package %s", v.name), ""

	}

	// Fall back to types.Object
	//    All other cases of this should be this type. We only had to implement PkgDeclaration because
	//    some fields are not exported in types.Object.
	//
	//    We expect any new ObjectLike items to be `types.Object` values.
	v, _ := obj.(types.Object)
	return types.ObjectString(v, packageQualifier), ""
}

// packageQualifier returns an empty string in order to remove the leading package
// name from all identifiers in the return value of types.ObjectString.
func packageQualifier(*types.Package) string { return "" }

// formatTypeSignature returns a brief description of the given struct or interface type.
func formatTypeSignature(obj *types.TypeName) string {
	switch obj.Type().Underlying().(type) {
	case *types.Struct:
		if obj.IsAlias() {
			switch obj.Type().(type) {
			case *types.Named:
				original := obj.Type().(*types.Named).Obj()
				var pkg string
				if obj.Pkg().Name() != original.Pkg().Name() {
					pkg = original.Pkg().Name() + "."
				}
				return fmt.Sprintf("type %s = %s%s", obj.Name(), pkg, original.Name())

			case *types.Struct:
				return fmt.Sprintf("type %s = struct", obj.Name())
			}
		}

		return fmt.Sprintf("type %s struct", obj.Name())
	case *types.Interface:
		return fmt.Sprintf("type %s interface", obj.Name())
	}

	return ""
}

// formatTypeExtra returns the beautified fields of the given struct or interface type.
//
// The output of `types.TypeString` puts fields of structs and interfaces on a single
// line separated by a semicolon. This method simply expands the fields to reside on
// different lines with the appropriate indentation.
func formatTypeExtra(obj *types.TypeName) string {
	extra := types.TypeString(obj.Type().Underlying(), packageQualifier)

	depth := 0
	buf := bytes.NewBuffer(make([]byte, 0, len(extra)))

outer:
	for i := 0; i < len(extra); i++ {
		switch extra[i] {
		case '"':
			for j := i + 1; j < len(extra); j++ {
				if extra[j] == '\\' {
					// skip over escaped characters
					j++
					continue
				}

				if extra[j] == '"' {
					// found non-escaped ending quote
					// write entire string unchanged, then skip to this
					// character adn continue the outer loop, which will
					// start the next iteration on the following character
					buf.WriteString(extra[i : j+1])
					i = j
					continue outer
				}
			}

			// note: we should never get down here otherwise
			// there is some illegal output from types.TypeString.

		case ';':
			buf.WriteString("\n")
			buf.WriteString(strings.Repeat(indent, depth))
			i++ // Skip following ' '

		case '{':
			// Special case empty fields so we don't insert
			// an unnecessary newline.
			if i < len(extra)-1 && extra[i+1] == '}' {
				buf.WriteString("{}")
				i++ // Skip following '}'
			} else {
				depth++
				buf.WriteString(" {\n")
				buf.WriteString(strings.Repeat(indent, depth))
			}

		case '}':
			depth--
			buf.WriteString("\n")
			buf.WriteString(strings.Repeat(indent, depth))
			buf.WriteString("}")

		default:
			buf.WriteByte(extra[i])
		}
	}

	return buf.String()
}
