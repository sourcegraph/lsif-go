package testdata

type InterfaceWithSingleMethod interface {
	SingleMethod() float64
}

type StructWithMethods struct{}

func (StructWithMethods) SingleMethod() float64 { return 5.0 }

type InterfaceWithSingleMethodTwoImplementers interface {
	SingleMethodTwoImpl() float64
}

type TwoImplOne struct{}

func (TwoImplOne) SingleMethodTwoImpl() float64 { return 5.0 }

type TwoImplTwo struct{}

func (TwoImplTwo) SingleMethodTwoImpl() float64         { return 5.0 }
func (TwoImplTwo) RandomThingThatDoesntMatter() float64 { return 5.0 }
