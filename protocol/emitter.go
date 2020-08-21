package protocol

import "sync/atomic"

// Emitter creates vertex and edge values and passes them to the underlying
// JSONWriter instance. Use of this struct guarantees that unique identifiers
// are generated for each constructed element.
type Emitter struct {
	writer JSONWriter
	id     uint64
}

// JSONWriter serializes vertexes and edges into JSON and writes them to an
// underlying writer as newline-delimited JSON.
type JSONWriter interface {
	// Write emits a single vertex or edge value.
	Write(v interface{})

	// Flush ensures that all elements have been written to the underlying writer.
	Flush() error
}

func NewEmitter(writer JSONWriter) *Emitter {
	return &Emitter{
		writer: writer,
	}
}

func (e *Emitter) EmitMetaData(root string, info ToolInfo) uint64 {
	id := e.nextID()
	e.writer.Write(NewMetaData(id, root, info))
	return id
}

func (e *Emitter) EmitProject(languageID string) uint64 {
	id := e.nextID()
	e.writer.Write(NewProject(id, languageID))
	return id
}

func (e *Emitter) EmitDocument(languageID, path string) uint64 {
	id := e.nextID()
	e.writer.Write(NewDocument(id, languageID, "file://"+path))
	return id
}

func (e *Emitter) EmitRange(start, end Pos) uint64 {
	id := e.nextID()
	e.writer.Write(NewRange(id, start, end))
	return id
}

func (e *Emitter) EmitResultSet() uint64 {
	id := e.nextID()
	e.writer.Write(NewResultSet(id))
	return id
}

func (e *Emitter) EmitHoverResult(contents []MarkedString) uint64 {
	id := e.nextID()
	e.writer.Write(NewHoverResult(id, contents))
	return id
}

func (e *Emitter) EmitTextDocumentHover(outV, inV uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewTextDocumentHover(id, outV, inV))
	return id
}

func (e *Emitter) EmitDefinitionResult() uint64 {
	id := e.nextID()
	e.writer.Write(NewDefinitionResult(id))
	return id
}

func (e *Emitter) EmitTextDocumentDefinition(outV, inV uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewTextDocumentDefinition(id, outV, inV))
	return id
}

func (e *Emitter) EmitReferenceResult() uint64 {
	id := e.nextID()
	e.writer.Write(NewReferenceResult(id))
	return id
}

func (e *Emitter) EmitTextDocumentReferences(outV, inV uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewTextDocumentReferences(id, outV, inV))
	return id
}

func (e *Emitter) EmitItem(outV uint64, inVs []uint64, docID uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewItem(id, outV, inVs, docID))
	return id
}

func (e *Emitter) EmitItemOfDefinitions(outV uint64, inVs []uint64, docID uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewItemOfDefinitions(id, outV, inVs, docID))
	return id
}

func (e *Emitter) EmitItemOfReferences(outV uint64, inVs []uint64, docID uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewItemOfReferences(id, outV, inVs, docID))
	return id
}

func (e *Emitter) EmitMoniker(kind, scheme, identifier string) uint64 {
	id := e.nextID()
	e.writer.Write(NewMoniker(id, kind, scheme, identifier))
	return id
}

func (e *Emitter) EmitMonikerEdge(outV, inV uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewMonikerEdge(id, outV, inV))
	return id
}

func (e *Emitter) EmitPackageInformation(packageName, scheme, version string) uint64 {
	id := e.nextID()
	e.writer.Write(NewPackageInformation(id, packageName, scheme, version))
	return id
}

func (e *Emitter) EmitPackageInformationEdge(outV, inV uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewPackageInformationEdge(id, outV, inV))
	return id
}

func (e *Emitter) EmitContains(outV uint64, inVs []uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewContains(id, outV, inVs))
	return id
}

func (e *Emitter) EmitNext(outV, inV uint64) uint64 {
	id := e.nextID()
	e.writer.Write(NewNext(id, outV, inV))
	return id
}

func (e *Emitter) NumElements() uint64 {
	return atomic.LoadUint64(&e.id)
}

func (e *Emitter) Flush() error {
	return e.writer.Flush()
}

func (e *Emitter) nextID() uint64 {
	return atomic.AddUint64(&e.id, 1)
}
