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
	encoder              *json.Encoder
	packageName          string
	packageVersion       string
	dependencies         map[string]string
	packageInformationID string
}

type moniker struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Scheme     string `json:"scheme"`
	Identifier string `json:"identifier"`
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
		return fmt.Errorf("failed to unmarshal line: %v", err)
	}

	if moniker.Type == "vertex" && moniker.Label == "moniker" {
		if moniker.Kind == "import" {
			return d.addImportMoniker(moniker)
		}

		if moniker.Kind == "export" {
			return d.addExportMoniker(moniker)
		}
	}

	return nil
}

func (d *decorator) addImportMoniker(moniker *moniker) error {
	// TODO - don't emit these in lsif-go in the first place
	name := strings.Trim(moniker.Identifier, `"`)
	version, ok := d.dependencies[name]
	if !ok {
		return nil
	}

	packageInformationID := uuid.New().String()
	if err := d.encoder.Encode(protocol.NewPackageInformation(packageInformationID, name, "gomod", version)); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}

	return d.addMonikers("import", moniker.Identifier, moniker.ID, packageInformationID)
}

func (d *decorator) addExportMoniker(moniker *moniker) error {
	if d.packageInformationID == "" {
		d.packageInformationID = uuid.New().String()
		if err := d.encoder.Encode(protocol.NewPackageInformation(d.packageInformationID, d.packageName, "gomod", d.packageVersion)); err != nil {
			return fmt.Errorf("failed to write: %v", err)
		}
	}

	return d.addMonikers("export", moniker.Identifier, moniker.ID, d.packageInformationID)
}

func (d *decorator) addMonikers(kind string, identifier string, sourceID, packageID string) error {
	monikerID := uuid.New().String()
	if err := d.encoder.Encode(protocol.NewMoniker(monikerID, kind, "gomod", identifier)); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}

	if err := d.encoder.Encode(protocol.NewPackageInformationEdge(uuid.New().String(), monikerID, packageID)); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}

	if err := d.encoder.Encode(protocol.NewNextMonikerEdge(uuid.New().String(), sourceID, monikerID)); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}

	return nil
}
