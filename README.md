# TinyHiera

TinyHiera is a small configuration resolver inspired by Hiera. It evaluates a YAML document alongside a set of facts to produce a final configuration map. The resolver supports `first` and `deep` merge strategies and relies on simple string interpolation for hierarchy entries.

It is optimized for single files that hold the hierarchy and configuration data rather than the multi file approach common in Hiera.

> [!NOTE]
> OpenAI Codex almost entirely created this project

## Installation

```
go get github.com/choria-io/tinyhiera
```

## Usage

Supply a YAML document and a map of facts. The resolver will parse the hierarchy, replace `%{fact}` placeholders, and merge the matching sections. Use `literal()` to emit templating characters verbatim (for example `%{literal('%')}{SERVER_NAME}` becomes `%{SERVER_NAME}`).

Here the `hierarchy` key defines the lookup strategies and the `configuration` key defines what will be returned.

The rest is the hierarchy data.

```go
package main

import (
        "fmt"

        "github.com/choria-io/tinyhiera"
)

func main() {
        yamlDoc := []byte(`
 hierarchy:
   order:
     - global
     - env:%{env}
     - role:%{role}
     - host:%{hostname}
   merge: deep

 configuration:
   log_level: INFO
   packages:
     - ca-certificates
   web:
     listen_port: 80
     tls: false

 env:prod:
   log_level: WARN

 role:web:
   packages:
     - nginx
   web:
     tls: true

 host:web01:
   log_level: TRACE
`)

        facts := map[string]any{
                "env":      "prod",
                "role":     "web",
                "hostname": "web01",
        }

        resolved, err := tinyhiera.ResolveYaml(yamlDoc, facts)
        if err != nil {
                panic(err)
        }

        fmt.Println(resolved)
}
```

Running the example yields the following configuration map:

```
map[log_level:TRACE packages:[ca-certificates nginx] web:map[listen_port:80 tls:true]]
```

## Merge strategies

- `first` (default): Applies the first matching overlay from the hierarchy order and returns the merged configuration.
- `deep`: Recursively merges all matching overlays. Maps are merged, slices are concatenated, and scalar values override earlier values.

## Parsed input usage

If you already have parsed YAML data available, call `Resolve` directly:

```go
config := map[string]any{
        "hierarchy": map[string]any{
                "order": []any{"configuration", "role:%{role}"},
                "merge": "deep",
        },
        "configuration": map[string]any{
                "value": 1,
        },
        "role:web": map[string]any{
                "value": 2,
        },
}

resolved, err := tinyhiera.Resolve(config, map[string]any{"role": "web"})
if err != nil {
        panic(err)
}

fmt.Println(resolved)
// Output: map[value:2]
```

## Testing

The project uses Ginkgo and Gomega for testing. Run the suite with:

```
go test ./...
```
