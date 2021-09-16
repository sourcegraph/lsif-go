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
