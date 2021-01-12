package testdata

type Struct struct {
	Field string
}

func (s *Struct) StructMethod() {}

type Interface interface {
	InterfaceMethod() string
}
