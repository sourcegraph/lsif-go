package initial

type LiteralType int

type FuncType func(LiteralType, int) bool

type IfaceType interface {
	Method() LiteralType
}

type StructType struct {
	m IfaceType
	f LiteralType

	// anonymous struct
	anon struct {
		sub int
	}

	// interface within struct
	i interface {
		AnonMethod() bool
	}
}

type DeclaredBefore struct{ DeclaredAfter }
type DeclaredAfter struct{}
