package testdata

import "io"

type I3 interface {
	Close() error
}

type TClose struct {
	io.Closer
}
