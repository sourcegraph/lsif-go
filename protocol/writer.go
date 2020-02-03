package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
)

type Writer struct {
	w              io.Writer
	excludeContent bool
	id             int
	numElements    int
}

func NewWriter(w io.Writer, excludeContent bool) *Writer {
	return &Writer{
		w:              w,
		excludeContent: excludeContent,
	}
}

func (w *Writer) NumElements() int {
	return w.numElements
}

func (w *Writer) NextID() string {
	w.id++
	return strconv.Itoa(w.id)
}

func (w *Writer) emit(v interface{}) error {
	w.numElements++
	return json.NewEncoder(w.w).Encode(v)
}

func (w *Writer) EmitMetaData(root string, info ToolInfo) (string, error) {
	id := w.NextID()
	return id, w.emit(NewMetaData(id, root, info))
}

func (w *Writer) EmitBeginEvent(scope string, data string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewEvent(id, "begin", scope, data))
}

func (w *Writer) EmitEndEvent(scope string, data string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewEvent(id, "end", scope, data))
}

func (w *Writer) EmitProject(languageID string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewProject(id, languageID))
}

func (w *Writer) EmitDocument(languageID, path string) (string, error) {
	var contents []byte
	if !w.excludeContent {
		var err error
		contents, err = ioutil.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read file: %v", err)
		}
	}

	id := w.NextID()
	return id, w.emit(NewDocument(id, languageID, "file://"+path, contents))
}

func (w *Writer) EmitContains(outV string, inVs []string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewContains(id, outV, inVs))
}

func (w *Writer) EmitResultSet() (string, error) {
	id := w.NextID()
	return id, w.emit(NewResultSet(id))
}

func (w *Writer) EmitRange(start, end Pos) (string, error) {
	id := w.NextID()
	return id, w.emit(NewRange(id, start, end))
}

func (w *Writer) EmitNext(outV, inV string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewNext(id, outV, inV))
}

func (w *Writer) EmitDefinitionResult() (string, error) {
	id := w.NextID()
	return id, w.emit(NewDefinitionResult(id))
}

func (w *Writer) EmitTextDocumentDefinition(outV, inV string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewTextDocumentDefinition(id, outV, inV))
}

func (w *Writer) EmitHoverResult(contents []MarkedString) (string, error) {
	id := w.NextID()
	return id, w.emit(NewHoverResult(id, contents))
}

func (w *Writer) EmitTextDocumentHover(outV, inV string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewTextDocumentHover(id, outV, inV))
}

func (w *Writer) EmitReferenceResult() (string, error) {
	id := w.NextID()
	return id, w.emit(NewReferenceResult(id))
}

func (w *Writer) EmitTextDocumentReferences(outV, inV string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewTextDocumentReferences(id, outV, inV))
}

func (w *Writer) EmitItem(outV string, inVs []string, docID string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewItem(id, outV, inVs, docID))
}

func (w *Writer) EmitItemOfDefinitions(outV string, inVs []string, docID string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewItemOfDefinitions(id, outV, inVs, docID))
}

func (w *Writer) EmitItemOfReferences(outV string, inVs []string, docID string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewItemOfReferences(id, outV, inVs, docID))
}

func (w *Writer) EmitPackageInformation(packageName, scheme, version string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewPackageInformation(id, packageName, scheme, version))
}

func (w *Writer) EmitMoniker(kind, scheme, identifier string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewMoniker(id, kind, scheme, identifier))
}

func (w *Writer) EmitPackageInformationEdge(outV, inV string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewPackageInformationEdge(id, outV, inV))
}

func (w *Writer) EmitMonikerEdge(outV, inV string) (string, error) {
	id := w.NextID()
	return id, w.emit(NewMonikerEdge(id, outV, inV))
}
