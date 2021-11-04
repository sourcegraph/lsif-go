package testdata

type I0 interface{}

type I1 interface {
	F1()
}

type I2 interface {
	F2()
}

type T1 int

func (r T1) F1() {}

type T2 int

func (r T2) F1() {}
func (r T2) F2() {}

type A1 = T1
type A12 = A1

type InterfaceWithNonExportedMethod interface {
	nonExportedMethod()
}

type InterfaceWithExportedMethod interface {
	ExportedMethod()
}

type Foo int

func (r Foo) nonExportedMethod() {}
func (r Foo) ExportedMethod()    {}
func (r Foo) Close() error       { return nil }

type SharedOne interface {
	Shared()
	Distinct()
}

type SharedTwo interface {
	Shared()
	Unique()
}

type Between struct{}

func (Between) Shared()   {}
func (Between) Distinct() {}
func (Between) Unique()   {}

func shouldShow(shared SharedOne) {
	shared.Shared()
}
