package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/choria-io/fisk"
	"github.com/choria-io/tinyhiera"
	"github.com/choria-io/tinyhiera/internal"
	"github.com/goccy/go-yaml"
	"github.com/tidwall/gjson"
)

var (
	input      string
	factsInput map[string]string
	factsFile  string
	sysFacts   bool
	envFacts   bool
	yamlOutput bool
	envOutput  bool
	envPrefix  string
	version    string
	query      string
	debug      bool

	ctx context.Context
)

func main() {
	factsInput = make(map[string]string)

	app := fisk.New("cmd", "Choria Hierarchical Data resolver")
	app.Version(version)
	app.Author("R.I.Pienaar <rip@choria.io>")

	parse := app.Command("parse", "Parses a YAML or JSON file and prints the result as JSON").Action(runAction)
	parse.Arg("input", "Input JSON or YAML file to resolve").Envar("HIERA_INPUT").Required().ExistingFileVar(&input)
	parse.Arg("fact", "Facts about the node").StringMapVar(&factsInput)
	parse.Flag("facts", "JSON or YAML file containing facts").ExistingFileVar(&factsFile)
	parse.Flag("system-facts", "Provide facts from the internal facts provider").Short('S').UnNegatableBoolVar(&sysFacts)
	parse.Flag("env-facts", "Provide facts from the process environment").Short('E').UnNegatableBoolVar(&envFacts)
	parse.Flag("yaml", "Output YAML instead of JSON").UnNegatableBoolVar(&yamlOutput)
	parse.Flag("env", "Output environment variables").UnNegatableBoolVar(&envOutput)
	parse.Flag("env-prefix", "Prefix for environment variable names").Default("HIERA").StringVar(&envPrefix)
	parse.Flag("query", "Performs a gjson query on the result").StringVar(&query)
	parse.Flag("debug", "Enables debug output").UnNegatableBoolVar(&debug)

	facts := app.Command("facts", "Shows resolved facts").Action(showFactsAction)
	facts.Arg("fact", "Facts about the node").StringMapVar(&factsInput)
	facts.Flag("facts", "JSON or YAML file containing facts").ExistingFileVar(&factsFile)
	facts.Flag("system-facts", "Provide facts from the internal facts provider").Short('S').UnNegatableBoolVar(&sysFacts)
	facts.Flag("env-facts", "Provide facts from the process environment").Short('E').UnNegatableBoolVar(&envFacts)
	facts.Flag("query", "Performs a gjson query on the facts").StringVar(&query)

	app.PreAction(func(_ *fisk.ParseContext) error {
		ctx, _ = signal.NotifyContext(context.Background(), os.Interrupt)
		return nil
	})

	app.MustParseWithUsage(os.Args[1:])
}

func showFactsAction(_ *fisk.ParseContext) error {
	facts, err := resolveFacts()
	if err != nil {
		return err
	}

	j, err := json.MarshalIndent(facts, "", "  ")
	if err != nil {
		return err
	}

	if query != "" {
		val := gjson.GetBytes(j, query)
		fmt.Println(val.String())
		return nil
	}

	fmt.Println(string(j))

	return nil
}
func runAction(_ *fisk.ParseContext) error {
	facts, err := resolveFacts()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	var res map[string]any
	var logger tinyhiera.Logger

	if debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	if isJson(data) {
		res, err = tinyhiera.ResolveJson(data, facts, logger)
	} else {
		res, err = tinyhiera.ResolveYaml(data, facts, logger)
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

func resolveFacts() (map[string]any, error) {
	facts := make(map[string]any)

	if sysFacts {
		sf, err := internal.StandardFacts(ctx)
		if err != nil {
			return nil, err
		}
		for k, v := range sf {
			facts[k] = v
		}
	}

	if envFacts {
		for _, v := range os.Environ() {
			kv := strings.Split(v, "=")
			facts[kv[0]] = kv[1]
		}
	}

	if factsFile != "" {
		fc, err := os.ReadFile(factsFile)
		if err != nil {
			return nil, err
		}

		if isJson(fc) {
			err = json.Unmarshal(fc, &facts)
		} else {
			err = yaml.Unmarshal(fc, &facts)
		}
		if err != nil {
			return nil, err
		}
	}

	for k, v := range factsInput {
		facts[k] = v
	}

	return facts, nil
}
