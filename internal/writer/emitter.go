package writer

import (
	"sync/atomic"

	"github.com/sourcegraph/lsif-go/protocol"
)

// Emitter creates vertex and edge values and passes them to the underlying
// JSONWriter instance. Use of this struct guarantees that unique identifiers
// are generated for each constructed element.
type Emitter struct {
	writer      JSONWriter
	id          uint64
	numElements uint64
}

func NewEmitter(writer JSONWriter) *Emitter {
	return &Emitter{
		writer: writer,
	}
}

func (e *Emitter) Flush() error {
	return e.writer.Flush()
}

func (e *Emitter) NumElements() uint64 {
	return atomic.LoadUint64(&e.numElements)
}

func (e *Emitter) EmitMetaData(root string, info protocol.ToolInfo) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewMetaData(id, root, info))
}

func (e *Emitter) EmitProject(languageID string) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewProject(id, languageID))
}

func (e *Emitter) EmitDocument(languageID, path string) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewDocument(id, languageID, "file://"+path, nil))
}

func (e *Emitter) EmitRange(start, end protocol.Pos) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewRange(id, start, end))
}

func (e *Emitter) EmitResultSet() (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewResultSet(id))
}

func (e *Emitter) EmitHoverResult(contents []protocol.MarkedString) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewHoverResult(id, contents))
}

func (e *Emitter) EmitTextDocumentHover(outV, inV uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentHover(id, outV, inV))
}

func (e *Emitter) EmitDefinitionResult() (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewDefinitionResult(id))
}

func (e *Emitter) EmitTextDocumentDefinition(outV, inV uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentDefinition(id, outV, inV))
}

func (e *Emitter) EmitReferenceResult() (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewReferenceResult(id))
}

func (e *Emitter) EmitTextDocumentReferences(outV, inV uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewTextDocumentReferences(id, outV, inV))
}

func (e *Emitter) EmitItem(outV uint64, inVs []uint64, docID uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItem(id, outV, inVs, docID))
}

func (e *Emitter) EmitItemOfDefinitions(outV uint64, inVs []uint64, docID uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItemOfDefinitions(id, outV, inVs, docID))
}

func (e *Emitter) EmitItemOfReferences(outV uint64, inVs []uint64, docID uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewItemOfReferences(id, outV, inVs, docID))
}

func (e *Emitter) EmitMoniker(kind, scheme, identifier string) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewMoniker(id, kind, scheme, identifier))
}

func (e *Emitter) EmitMonikerEdge(outV, inV uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewMonikerEdge(id, outV, inV))
}

func (e *Emitter) EmitPackageInformation(packageName, scheme, version string) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewPackageInformation(id, packageName, scheme, version))
}

func (e *Emitter) EmitPackageInformationEdge(outV, inV uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewPackageInformationEdge(id, outV, inV))
}

func (e *Emitter) EmitContains(outV uint64, inVs []uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewContains(id, outV, inVs))
}

func (e *Emitter) EmitNext(outV, inV uint64) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewNext(id, outV, inV))
}

func (e *Emitter) EmitBeginEvent(scope string, data string) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewEvent(id, "begin", scope, data))
}

func (e *Emitter) EmitEndEvent(scope string, data string) (uint64, error) {
	id := e.nextID()
	return id, e.emit(protocol.NewEvent(id, "end", scope, data))
}

func (e *Emitter) nextID() uint64 {
	return atomic.AddUint64(&e.id, 1)
}

func (e *Emitter) emit(v interface{}) error {
	atomic.AddUint64(&e.numElements, 1)
	return e.writer.Write(v)
}
