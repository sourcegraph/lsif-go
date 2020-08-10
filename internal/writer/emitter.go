package writer

import (
	"sync/atomic"

	"github.com/sourcegraph/lsif-go/protocol"
)

// Emitter creates vertex and edge values and passes them to the underlying
// JSONWriter instance. Use of this struct guarantees that unique identifiers
// are generated for each constructed element.
type Emitter struct {
	writer JSONWriter
	id     uint64
}

func NewEmitter(writer JSONWriter) *Emitter {
	return &Emitter{
		writer: writer,
	}
}

func (e *Emitter) NumElements() uint64 {
	return atomic.LoadUint64(&e.id)
}

func (e *Emitter) NextID() uint64 {
	return atomic.AddUint64(&e.id, 1)
}

func (e *Emitter) Flush() error {
	return e.writer.Flush()
}

func (e *Emitter) EmitMetaData(root string, info protocol.ToolInfo) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewMetaData(id, root, info))
	return id, nil
}

func (e *Emitter) EmitProject(languageID string) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewProject(id, languageID))
	return id, nil
}

func (e *Emitter) EmitDocument(languageID, path string) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewDocument(id, languageID, "file://"+path, nil))
	return id, nil
}

func (e *Emitter) EmitRange(start, end protocol.Pos) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewRange(id, start, end))
	return id, nil
}

func (e *Emitter) EmitResultSet() (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewResultSet(id))
	return id, nil
}

func (e *Emitter) EmitHoverResult(contents []protocol.MarkedString) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewHoverResult(id, contents))
	return id, nil
}

func (e *Emitter) EmitTextDocumentHover(outV, inV uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewTextDocumentHover(id, outV, inV))
	return id, nil
}

func (e *Emitter) EmitDefinitionResult() (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewDefinitionResult(id))
	return id, nil
}

func (e *Emitter) EmitTextDocumentDefinition(outV, inV uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewTextDocumentDefinition(id, outV, inV))
	return id, nil
}

func (e *Emitter) EmitReferenceResult() (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewReferenceResult(id))
	return id, nil
}

func (e *Emitter) EmitTextDocumentReferences(outV, inV uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewTextDocumentReferences(id, outV, inV))
	return id, nil
}

func (e *Emitter) EmitItem(outV uint64, inVs []uint64, docID uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewItem(id, outV, inVs, docID))
	return id, nil
}

func (e *Emitter) EmitItemOfDefinitions(outV uint64, inVs []uint64, docID uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewItemOfDefinitions(id, outV, inVs, docID))
	return id, nil
}

func (e *Emitter) EmitItemOfReferences(outV uint64, inVs []uint64, docID uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewItemOfReferences(id, outV, inVs, docID))
	return id, nil
}

func (e *Emitter) EmitMoniker(kind, scheme, identifier string) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewMoniker(id, kind, scheme, identifier))
	return id, nil
}

func (e *Emitter) EmitMonikerEdge(outV, inV uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewMonikerEdge(id, outV, inV))
	return id, nil
}

func (e *Emitter) EmitPackageInformation(packageName, scheme, version string) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewPackageInformation(id, packageName, scheme, version))
	return id, nil
}

func (e *Emitter) EmitPackageInformationEdge(outV, inV uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewPackageInformationEdge(id, outV, inV))
	return id, nil
}

func (e *Emitter) EmitContains(outV uint64, inVs []uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewContains(id, outV, inVs))
	return id, nil
}

func (e *Emitter) EmitNext(outV, inV uint64) (uint64, error) {
	id := e.NextID()
	e.writer.Write(protocol.NewNext(id, outV, inV))
	return id, nil
}
