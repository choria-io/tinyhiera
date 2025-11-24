package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/choria-io/fisk"
	"github.com/choria-io/tinyhiera"
	"github.com/goccy/go-yaml"
)

var (
	input      string
	factsInput map[string]string
	factsFile  string
	yamlOutput bool
	version    string
)

func main() {
	factsInput = make(map[string]string)

	app := fisk.New("tinyhiera", "Tiny Hierarchical data resolver").Action(runAction)
	app.Version(version)
	app.Author("R.I.Pienaar <rip@choria.io>")

	app.Arg("input", "Input JSON or YAML file to resolve").Required().ExistingFileVar(&input)
	app.Arg("fact", "Facts about the node").StringMapVar(&factsInput)
	app.Flag("facts", "JSON or YAML file containing facts").ExistingFileVar(&factsFile)
	app.Flag("yaml", "Output YAML instead of JSON").UnNegatableBoolVar(&yamlOutput)

	app.MustParseWithUsage(os.Args[1:])
}

func runAction(_ *fisk.ParseContext) error {
	facts := make(map[string]any)
	for k, v := range factsInput {
		facts[k] = v
	}

	if factsFile != "" {
		fc, err := os.ReadFile(factsFile)
		if err != nil {
			return err
		}

		if isJson(fc) {
			err = json.Unmarshal(fc, &facts)
		} else {
			err = yaml.Unmarshal(fc, &facts)
		}
		if err != nil {
			return err
		}
	}

	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	var res map[string]any
	if isJson(data) {
		res, err = tinyhiera.ResolveJson(data, facts)
	} else {
		res, err = tinyhiera.ResolveYaml(data, facts)
	}
	if err != nil {
		return err
	}

	var out []byte
	if yamlOutput {
		out, err = yaml.Marshal(res)
	} else {
		out, err = json.MarshalIndent(res, "", "  ")
	}
	if err != nil {
		return err
	}

	fmt.Println(strings.TrimSpace(string(out)))

	return nil
}

func isJson(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))

	return strings.HasPrefix(string(trimmed), "{") || strings.HasPrefix(string(trimmed), "[")
}
