package testdata

import (
	. "fmt"
	h "net/http"
)

func Example() {
	Println(h.CanonicalHeaderKey("accept-encoding"))
}
