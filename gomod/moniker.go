package gomod

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/sourcegraph/lsif-go/protocol"
)

type decorator struct {
	encoder               *json.Encoder
	moduleName            string
	moduleVersion         string
	dependencies          map[string]string
	packageInformationIDs map[string]string
}

type moniker struct {
	ID         string               `json:"id"`
	Type       protocol.ElementType `json:"type"`
	Label      protocol.VertexLabel `json:"label"`
	Kind       string               `json:"kind"`
	Scheme     string               `json:"scheme"`
	Identifier string               `json:"identifier"`
}

func newDecorator(out io.Writer, moduleName, moduleVersion string, dependencies map[string]string) *decorator {
	return &decorator{
		moduleName:            moduleName,
		moduleVersion:         moduleVersion,
		dependencies:          dependencies,
		packageInformationIDs: map[string]string{},
		encoder:               json.NewEncoder(out),
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
	for _, moduleName := range packagePrefixes(strings.Split(moniker.Identifier, ":")[0]) {
		moduleVersion, ok := d.dependencies[moduleName]
		if !ok {
			continue
		}

		packageInformationID, err := d.ensurePackageInformation(moduleName, moduleVersion)
		if err != nil {
			return err
		}

		return d.addMonikers("import", moniker.Identifier, moniker.ID, packageInformationID)
	}

	return nil
}

func (d *decorator) addExportMoniker(moniker *moniker) error {
	packageInformationID, err := d.ensurePackageInformation(d.moduleName, d.moduleVersion)
	if err != nil {
		return err
	}

	return d.addMonikers("export", moniker.Identifier, moniker.ID, packageInformationID)
}

func (d *decorator) ensurePackageInformation(packageName, version string) (string, error) {
	packageInformationID, ok := d.packageInformationIDs[packageName]
	if !ok {
		packageInformationID = uuid.New().String()
		vertex := protocol.NewPackageInformation(packageInformationID, packageName, "gomod", version)
		if err := d.encoder.Encode(vertex); err != nil {
			return "", err
		}

		d.packageInformationIDs[packageName] = packageInformationID
	}

	return packageInformationID, nil
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

func packagePrefixes(packageName string) []string {
	parts := strings.Split(packageName, "/")
	prefixes := make([]string, len(parts))

	for i := 1; i <= len(parts); i++ {
		prefixes[len(parts)-i] = strings.Join(parts[:i], "/")
	}

	return prefixes
}
