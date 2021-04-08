// Package testdata
package testdata

type Embedded struct {
	EmbeddedField string
	Field string // conflicts with parent "Field"
}

type Struct struct {
	*Embedded
	Field string
	Anonymous struct {
		FieldA int
		FieldB int
		FieldC int
	}
}}

func (s *Struct) StructMethod() {}

type Interface interface {
	InterfaceMethod() string
}
