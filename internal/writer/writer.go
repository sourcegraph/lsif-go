package writer

import (
	"encoding/json"
	"io"
)

// JSONWriter serializes vertexes and edges into JSON and writes them to an
// underlying writer as newline-delimited JSON.
type JSONWriter interface {
	// Write emits a single vertex or edge value.
	Write(v interface{}) error
}

type jsonWriter struct {
	w io.Writer
}

// NewJSONWriter creates a new JSONWriter wrapping the given writer.
func NewJSONWriter(w io.Writer) JSONWriter {
	return &jsonWriter{w}
}

// Write emits a single vertex or edge value.
func (w *jsonWriter) Write(v interface{}) error {
	return json.NewEncoder(w.w).Encode(v)
}
