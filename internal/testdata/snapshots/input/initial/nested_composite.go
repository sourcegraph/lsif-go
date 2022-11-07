package initial

import "net/http"

type NestedHandler struct {
	http.Handler
	Other int
}

func NestedExample(n NestedHandler) {
	_ = n.Handler.ServeHTTP
	_ = n.ServeHTTP
	_ = n.Other
}
