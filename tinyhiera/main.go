package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/choria-io/fisk"
	"github.com/choria-io/tinyhiera"
	"github.com/goccy/go-yaml"
	"github.com/tidwall/gjson"
)

var (
	input      string
	factsInput map[string]string
	factsFile  string
	yamlOutput bool
	envOutput  bool
	envPrefix  string
	version    string
	query      string
)

func main() {
	factsInput = make(map[string]string)

	app := fisk.New("tinyhiera", "Tiny Hierarchical data resolver").Action(runAction)
	app.Version(version)
	app.Author("R.I.Pienaar <rip@choria.io>")

	app.Arg("input", "Input JSON or YAML file to resolve").Required().ExistingFileVar(&input)
	app.Arg("fact", "Facts about the node").StringMapVar(&factsInput)
	app.Flag("query", "Performs a gjson query on the result").StringVar(&query)
	app.Flag("facts", "JSON or YAML file containing facts").ExistingFileVar(&factsFile)
	app.Flag("yaml", "Output YAML instead of JSON").UnNegatableBoolVar(&yamlOutput)
	app.Flag("env", "Output environment variables").UnNegatableBoolVar(&envOutput)
	app.Flag("env-prefix", "Prefix for environment variable names").Default("HIERA").StringVar(&envPrefix)

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

	jout, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}

	if query != "" {
		val := gjson.GetBytes(jout, query)
		fmt.Println(val.String())
		return nil
	}

	var out []byte
	switch {
	case yamlOutput:
		out, err = yaml.Marshal(res)
	case envOutput:
		buff := bytes.NewBuffer([]byte{})
		err = renderEnvOutput(buff, res)
		if err != nil {
			return err
		}
		out = buff.Bytes()
	default:
		out = jout
	}
	if err != nil {
		return err
	}

	fmt.Println(strings.TrimSpace(string(out)))

	return nil
}

func renderEnvOutput(w io.Writer, res map[string]any) error {
	for k, v := range res {
		key := fmt.Sprintf("%s_%s", envPrefix, strings.ToUpper(k))

		switch typed := v.(type) {
		case string:
			fmt.Fprintf(w, "%s=%s\n", key, typed)
		case int8, int16, int32, int64, int:
			fmt.Fprintf(w, "%s=%d\n", key, typed)
		case float32, float64:
			fmt.Fprintf(w, "%s=%f\n", key, typed)
		default:
			j, err := json.Marshal(typed)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%s=%s\n", key, string(j))
		}
	}

	return nil
}

func isJson(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))

	return strings.HasPrefix(string(trimmed), "{") || strings.HasPrefix(string(trimmed), "[")
}
