# TinyHiera

TinyHiera is a small data resolver inspired by Hiera. It evaluates a YAML document alongside a set of facts to produce a final data map. The resolver supports `first` and `deep` merge strategies and relies on simple string interpolation for hierarchy entries.

It is optimized for single files that hold the hierarchy and data rather than the multi file approach common in Hiera.

This is an experiment at the moment to see how I can solve a specific need, my goal is to create a small-scale configuration management system suitable for running in a Choria Autonomous Agent.

Autonomous Agents focus on managing a single thing, like an application, and owns the entire lifecycle of that application including monitoring, remediation, upgrades and everything.  In that context the data needs are simple - essentially those of a single Puppet module.

So this Hiera, while being Hiera inspired, will be quite different.  It will not be orientated around single-key lookup but rather in resolving the entire data structure in one go and handing it back fully resolved.

The end goal is to have some CLI tooling that allows for this:

```nohighlight
# Like the Puppet RAL but for CLI, supports multiple package systems etc
$ marionette package ensure zsh --version 1.2.3
$ marionette service ensure ssh-server --enable --running
$ marionette service info httpd

# Can be driven by JSON in and returns JSON out instead
$ echo '{"name":"zsh", "version":"1.2.3"}'|marionette package apply
{
 ....
} 
```

The aim above is to make a scriptable RAL, you get multi OS support but your scripts do not change to support different OSes and the individual RAL calls remain idempotent - making it much easier to create re-runnable scripts.  We will focus on just package, `service`, `file`, `exec` and `user` resources to keep things focussed.

We could though create manifest that encapsulates one service - the package, config, service trio - into one JSON file and apply that:

```nohighlight
# We create a manifest installing a package with a customizable version
cat <<EOF>manifest.json
{
  "hierarchy": {
    # note we have access to other functions here in the query like lowecasing stuff, easy to extend
    "order": [ "fqdn:{{ lookup('facts.networking.fqdn') | lower() }}" ]
  }
  "data": {
    "package": "zsh",
    "version": "present"
  }
  "resources": [
    {"package": {"name": "{{ lookup('data.package') }}", "version": "{{ lookup('data.version') }}"}}   
  ]
  "overrides": {
    "fqdn:my.example.net": {
        "version": "1.2.3",
        "package": "zsh-shell"
    }
  }
}
EOF

# we apply the manifest using node facts in facts.json
$ marionette apply manifest.json --facts facts.json
```

And we could even compile this manifest to a executable binary that is statically compiled and have no dependencies - imagine `./setup --facts facts.json` and your node is configured.

## Status

Given this focus, the needs will be a bit different and those are still being discovered. You're welcome to share ideas and feedback but I'd hold off on using this just yet.

TODO list:

 * [ ] Move away from `${...}` to `{{ ... }}` this feels a bit more modern and aligns more with Choria
 * [ ] Support interpolating data in values using [expr](https://expr-lang.org) 
 * [ ] Once `expr` support lands support data types for interpolated values
 * [x] Add a `--query` flag to the CLI to dig into the resulting data
 * [x] Rename `configuration` to more generic `data`
 * [ ] Move the overriding data from top level to `overrides`
 * [ ] Move to a dependency for deep merges, the implementation here is a bit meh
 
## Installation

```
go get github.com/choria-io/tinyhiera
```

## Usage

### Hierarchy file format

Here is an annotated example of a hierarchy file:

```yaml
hierarchy:
    # this is the lookup and override order, facts will be resolved here
    #
    # if your fact is nested, you can use gjson format queries like %{networking.fqdn}
    order:
     - env:%{env}
     - role:%{role}
     - host:%{hostname}
    merge: deep # or first

# This is the resulting output and must be present, the hierarchy results will be merged in
data:
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
```

See [GJSON Path Syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) for help in accessing nested facts.

### CLI example

A small utility is provided to resolve a hierarchy file and a set of facts:

Given the input file `data.json`:

```json{
{
    "hierarchy": {
        "order": [
            "fqdn:%{fqdn}"
        ]
    },
    "data": {
        "test": "value"
    },
    "fqdn:my.fqdn.com": {
        "test": "override"
    }
}
```

We can run the utility like this:

```
$ tinyhiera data.json fqdn=my.fqdn.com
{
  "test": "override"
}
$ tinyhiera data.json fqdn=other.fqdn.com
{
  "test": "value"
}
```

It can also produce YAML output:

```
$ tinyhiera test.json fqdn=other.fqdn.com --yaml
test: value
```

### Go example

Supply a YAML document and a map of facts. The resolver will parse the hierarchy, replace `%{fact}` placeholders, and merge the matching sections. Use `literal()` to emit templating characters verbatim (for example `%{literal('%')}{SERVER_NAME}` becomes `%{SERVER_NAME}`).

Here the `hierarchy` key defines the lookup strategies and the `data` key defines what will be returned.

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
     - env:%{env}
     - role:%{role}
     - host:%{hostname}
   merge: deep

 data:
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

Running the example yields the following data map:

```
map[log_level:TRACE packages:[ca-certificates nginx] web:map[listen_port:80 tls:true]]
```

## Merge strategies

- `first` (default): Applies the first matching overlay from the hierarchy order and returns the merged data.
- `deep`: Recursively merges all matching overlays. Maps are merged, slices are concatenated, and scalar values override earlier values.

## Parsed input usage

If you already have parsed YAML data available, call `Resolve` directly:

```go
config := map[string]any{
        "hierarchy": map[string]any{
                "order": []any{"role:%{role}"},
                "merge": "deep",
        },
        "data": map[string]any{
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
