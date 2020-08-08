package writer

import (
	"strconv"

	"github.com/sourcegraph/lsif-go/protocol"
)

// Emitter creates vertex and edge values and passes them to the underlying
// JSONWriter instance. Use of this struct guarantees that unique identifiers
// are generated for each constructed element.
type Emitter struct {
	writer      JSONWriter
	id          int
	numElements int
}

func NewEmitter(writer JSONWriter) *Emitter {
	return &Emitter{
		writer: writer,
	}
}

func (e *Emitter) Flush() error {
	return e.writer.Flush()
}

func (e *Emitter) NumElements() int {
	return e.numElements
}

func (e *Emitter) NextID() string {
	e.id++
	return strconv.Itoa(e.id)
}

func (e *Emitter) emit(v interface{}) error {
	e.numElements++
	return e.writer.Write(v)
}

func (e *Emitter) EmitMetaData(root string, info protocol.ToolInfo) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewMetaData(id, root, info))
}

func (e *Emitter) EmitProject(languageID string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewProject(id, languageID))
}

func (e *Emitter) EmitDocument(languageID, path string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewDocument(id, languageID, "file://"+path, nil))
}

func (e *Emitter) EmitRange(start, end protocol.Pos) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewRange(id, start, end))
}

func (e *Emitter) EmitResultSet() (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewResultSet(id))
}

func (e *Emitter) EmitHoverResult(contents []protocol.MarkedString) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewHoverResult(id, contents))
}

func (e *Emitter) EmitTextDocumentHover(outV, inV string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewTextDocumentHover(id, outV, inV))
}

func (e *Emitter) EmitDefinitionResult() (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewDefinitionResult(id))
}

func (e *Emitter) EmitTextDocumentDefinition(outV, inV string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewTextDocumentDefinition(id, outV, inV))
}

func (e *Emitter) EmitReferenceResult() (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewReferenceResult(id))
}

func (e *Emitter) EmitTextDocumentReferences(outV, inV string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewTextDocumentReferences(id, outV, inV))
}

func (e *Emitter) EmitItem(outV string, inVs []string, docID string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewItem(id, outV, inVs, docID))
}

func (e *Emitter) EmitItemOfDefinitions(outV string, inVs []string, docID string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewItemOfDefinitions(id, outV, inVs, docID))
}

func (e *Emitter) EmitItemOfReferences(outV string, inVs []string, docID string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewItemOfReferences(id, outV, inVs, docID))
}

func (e *Emitter) EmitMoniker(kind, scheme, identifier string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewMoniker(id, kind, scheme, identifier))
}

func (e *Emitter) EmitMonikerEdge(outV, inV string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewMonikerEdge(id, outV, inV))
}

func (e *Emitter) EmitPackageInformation(packageName, scheme, version string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewPackageInformation(id, packageName, scheme, version))
}

func (e *Emitter) EmitPackageInformationEdge(outV, inV string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewPackageInformationEdge(id, outV, inV))
}

func (e *Emitter) EmitContains(outV string, inVs []string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewContains(id, outV, inVs))
}

func (e *Emitter) EmitNext(outV, inV string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewNext(id, outV, inV))
}

func (e *Emitter) EmitBeginEvent(scope string, data string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewEvent(id, "begin", scope, data))
}

func (e *Emitter) EmitEndEvent(scope string, data string) (string, error) {
	id := e.NextID()
	return id, e.emit(protocol.NewEvent(id, "end", scope, data))
}
