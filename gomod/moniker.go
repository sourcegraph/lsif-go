package gomod

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/sourcegraph/lsif-go/protocol"
)

type decorator struct {
	encoder              *json.Encoder
	packageName          string
	packageVersion       string
	dependencies         map[string]string
	packageInformationID string
}

type moniker struct {
	ID         string               `json:"id"`
	Type       protocol.ElementType `json:"type"`
	Label      protocol.VertexLabel `json:"label"`
	Kind       string               `json:"kind"`
	Scheme     string               `json:"scheme"`
	Identifier string               `json:"identifier"`
}

func newDecorator(out io.Writer, packageName, packageVersion string, dependencies map[string]string) *decorator {
	return &decorator{
		packageName:    packageName,
		packageVersion: packageVersion,
		dependencies:   dependencies,
		encoder:        json.NewEncoder(out),
	}
}

func (d *decorator) decorate(line string) error {
	moniker := &moniker{}
	if err := json.Unmarshal([]byte(line), &moniker); err != nil {
		return fmt.Errorf("unmarshal line: %v", err)
	}

	if moniker.Type == protocol.ElementVertex && moniker.Label == protocol.VertexMoniker {
		switch moniker.Kind {
		case "import":
			if err := d.addImportMoniker(moniker); err != nil {
				return fmt.Errorf("encode json: %v", err)
			}

		case "export":
			if err := d.addExportMoniker(moniker); err != nil {
				return fmt.Errorf("encode json: %v", err)
			}
		}
	}

	return nil
}

func (d *decorator) addImportMoniker(moniker *moniker) error {
	version, ok := d.dependencies[moniker.Identifier]
	if !ok {
		return nil
	}

	packageInformationID := uuid.New().String()
	vertex := protocol.NewPackageInformation(packageInformationID, moniker.Identifier, "gomod", version)
	if err := d.encoder.Encode(vertex); err != nil {
		return err
	}

	return d.addMonikers("import", moniker.Identifier, moniker.ID, packageInformationID)
}

func (d *decorator) addExportMoniker(moniker *moniker) error {
	// If we haven't exported our own package information, do so now.
	// If the vertex is needed again later, we can use the same identifier.
	if d.packageInformationID == "" {
		d.packageInformationID = uuid.New().String()
		vertex := protocol.NewPackageInformation(d.packageInformationID, d.packageName, "gomod", d.packageVersion)
		if err := d.encoder.Encode(vertex); err != nil {
			return err
		}
	}

	return d.addMonikers("export", moniker.Identifier, moniker.ID, d.packageInformationID)
}

// addMonikers outputs a "gomod" moniker vertex, attaches the given package vertex
// identifier to it, and attaches the new moniker to the source moniker vertex.
func (d *decorator) addMonikers(kind string, identifier string, sourceID, packageID string) error {
	monikerID := uuid.New().String()
	vertex := protocol.NewMoniker(monikerID, kind, "gomod", identifier)
	if err := d.encoder.Encode(vertex); err != nil {
		return err
	}

	packageInformationEdge := protocol.NewPackageInformationEdge(uuid.New().String(), monikerID, packageID)
	if err := d.encoder.Encode(packageInformationEdge); err != nil {
		return err
	}

	nextMonikerEdge := protocol.NewNextMonikerEdge(uuid.New().String(), sourceID, monikerID)
	if err := d.encoder.Encode(nextMonikerEdge); err != nil {
		return err
	}

	return nil
}
