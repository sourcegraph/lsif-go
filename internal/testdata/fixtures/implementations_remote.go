package testdata

import "net/http"

type implementsWriter struct{}

func (implementsWriter) Header() http.Header        { panic("Just for how") }
func (implementsWriter) Write([]byte) (int, error)  { panic("Just for show") }
func (implementsWriter) WriteHeader(statusCode int) {}

func ShowsInSignature(respWriter http.ResponseWriter) {
	respWriter.WriteHeader(1)
}
