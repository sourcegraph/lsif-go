package writer

import (
	"bufio"
	"io"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/sourcegraph/lsif-go/protocol"
)

var marshaller = jsoniter.ConfigFastest

type jsonWriter struct {
	wg             sync.WaitGroup
	ch             chan (interface{})
	bufferedWriter *bufio.Writer
	err            error
}

var _ protocol.JSONWriter = &jsonWriter{}

// channelBufferSize is the number of elements that can be queued to be written.
const channelBufferSize = 512

// writerBufferSize is the size of the buffered writer wrapping output to the target file.
const writerBufferSize = 4096

// NewJSONWriter creates a new JSONWriter wrapping the given writer.
func NewJSONWriter(w io.Writer) protocol.JSONWriter {
	ch := make(chan interface{}, channelBufferSize)
	bufferedWriter := bufio.NewWriterSize(w, writerBufferSize)
	jw := &jsonWriter{ch: ch, bufferedWriter: bufferedWriter}
	encoder := marshaller.NewEncoder(bufferedWriter)

	jw.wg.Add(1)
	go func() {
		defer jw.wg.Done()

		for v := range ch {
			if err := encoder.Encode(v); err != nil {
				jw.err = err
				break
			}
		}

		for range ch {
		}
	}()

	return jw
}

// Write emits a single vertex or edge value.
func (jw *jsonWriter) Write(v interface{}) {
	jw.ch <- v
}

// Flush ensures that all elements have been written to the underlying writer.
func (jw *jsonWriter) Flush() error {
	close(jw.ch)
	jw.wg.Wait()

	if jw.err != nil {
		return jw.err
	}

	if err := jw.bufferedWriter.Flush(); err != nil {
		return err
	}

	return nil
}
