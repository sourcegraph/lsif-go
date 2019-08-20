package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/sourcegraph/lsif-go/protocol"
)

func main() {
	if err := doMain(); err != nil {
		panic(err.Error())
	}
}

func doMain() error {
	packageName, packageVersion, err := readModFile("go.mod")
	if err != nil {
		return err
	}

	dependencies, err := readSumFile("go.sum")
	if err != nil {
		return err
	}

	in, err := os.Open(os.Args[1])
	if err != nil {
		return fmt.Errorf("open dump file: %v", err)
	}
	defer in.Close()

	out, err := os.Create("data.lsif")
	if err != nil {
		return fmt.Errorf("create dump file: %v", err)
	}
	defer out.Close()

	encoder := json.NewEncoder(out)

	lines := 0
	maxToken := 1024 * 1024 * 1024
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, maxToken), maxToken)

	for scanner.Scan() {
		lines++
		_, err := out.WriteString(scanner.Text() + "\n")
		if err != nil {
			return fmt.Errorf("failed to write: %v", err)
		}

		if err := handleLine(scanner.Text(), encoder, packageName, packageVersion, dependencies); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return out.Sync()
}

type moniker struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Scheme     string `json:"scheme"`
	Identifier string `json:"identifier"`
}

var exportPackageID string

func handleLine(line string, encoder *json.Encoder, packageName, packageVersion string, dependencies map[string]string) error {
	moniker := &moniker{}
	if err := json.Unmarshal([]byte(line), &moniker); err != nil {
		return err
	}

	if moniker.Type != "vertex" || moniker.Label != "moniker" || moniker.Kind == "local" {
		return nil
	}

	if moniker.Kind == "import" {
		name := strings.Trim(moniker.Identifier, `"`)
		version, ok := dependencies[name]
		if ok {
			id0 := uuid.New().String()
			v1 := protocol.NewPackageInformation(id0, name, "gomod", version)
			if err := encoder.Encode(v1); err != nil {
				return err
			}

			if err := addMonikers("import", moniker.Identifier, moniker.ID, id0, encoder); err != nil {
				return err
			}
		}
	}

	if moniker.Kind == "export" {
		if exportPackageID == "" {
			exportPackageID = uuid.New().String()
			v1 := protocol.NewPackageInformation(exportPackageID, packageName, "gomod", packageVersion)
			if err := encoder.Encode(v1); err != nil {
				return err
			}
		}

		if err := addMonikers("export", moniker.Identifier, moniker.ID, exportPackageID, encoder); err != nil {
			return err
		}
	}

	return nil
}

func addMonikers(kind string, identifier string, sourceID, packageID string, encoder *json.Encoder) error {
	id1 := uuid.New().String()
	v2 := protocol.NewMoniker(id1, kind, "gomod", identifier)
	if err := encoder.Encode(v2); err != nil {
		return err
	}

	id2 := uuid.New().String()
	e1 := protocol.NewPackageInformationEdge(id2, id1, packageID)
	if err := encoder.Encode(e1); err != nil {
		return err
	}

	id3 := uuid.New().String()
	e2 := protocol.NewNextMonikerEdge(id3, sourceID, id1)
	if err := encoder.Encode(e2); err != nil {
		return err
	}

	return nil
}

var (
	// TODO - also get version
	modPattern = regexp.MustCompile("^module (.*)$")

	// TODO - read docs to get actual pattern
	sumPattern = regexp.MustCompile("^([^ ]+) v([^/]+)/go.mod")
)

func readModFile(filepath string) (string, string, error) {
	reader, err := os.Open(filepath)
	if err != nil {
		return "", "", fmt.Errorf("open dump file: %v", err)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if matches := modPattern.FindStringSubmatch(scanner.Text()); len(matches) > 0 {
			return matches[1], "0.1.0", nil
		}
	}

	return "", "", nil
}

func readSumFile(filepath string) (map[string]string, error) {
	reader, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("open dump file: %v", err)
	}
	defer reader.Close()

	dependencies := map[string]string{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if matches := sumPattern.FindStringSubmatch(scanner.Text()); len(matches) > 0 {
			dependencies[matches[1]] = matches[2]
		}
	}

	return dependencies, nil
}
