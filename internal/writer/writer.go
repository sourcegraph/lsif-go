package writer

import (
	"io"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var marshaller = jsoniter.ConfigFastest

// JSONWriter serializes vertexes and edges into JSON and writes them to an
// underlying writer as newline-delimited JSON.
type JSONWriter interface {
	// Write emits a single vertex or edge value.
	Write(v interface{}) error

	// Flush ensures that all elements have been written to the underlying writer.
	Flush() error
}

type jsonWriter struct {
	wg   sync.WaitGroup
	ch   chan (interface{})
	err  error
	once sync.Once
}

// TODO - document
const channelBufferSize = 512

// NewJSONWriter creates a new JSONWriter wrapping the given writer.
func NewJSONWriter(w io.Writer) JSONWriter {
	ch := make(chan interface{}, channelBufferSize)
	jw := &jsonWriter{ch: ch}
	encoder := marshaller.NewEncoder(w)

	jw.wg.Add(1)
	go func() {
		defer jw.wg.Done()

		for v := range ch {
			if err := encoder.Encode(v); err != nil {
				jw.once.Do(func() { jw.err = err })
			}
		}
	}()

	return jw
}

// Write emits a single vertex or edge value.
func (jw *jsonWriter) Write(v interface{}) error {
	jw.ch <- v
	return nil
}

// Flush ensures that all elements have been written to the underlying writer.
func (jw *jsonWriter) Flush() error {
	close(jw.ch)
	jw.wg.Wait()
	return jw.err
}
