package testdata

import "net/http"

type NestedHandler struct {
	http.Handler
	Other int
}
