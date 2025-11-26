# Choria Hierarchical Data Resolver

Choria Hierarchical Data Resolver (aka `tinyhiera`) is a small data resolver inspired by Hiera. It evaluates a YAML or JSON document alongside a set of facts to produce a final data map. The resolver supports `first` and `deep` merge strategies and relies on simple string interpolation for hierarchy entries.

It is optimized for single files that hold the hierarchy and data rather than the multi-file approach common in Hiera.

Major features:

 * Lookup expressions based on a full language
 * Types are supported, and lookups can return typed data
 * Command line tool that includes built-in system facts
 * Go library

## Background

My goal is to create a small-scale Configuration Management system suitable for running in shell scripts, standalone or in a Choria Autonomous Agent.

Autonomous Agents focus on managing a single thing, like an application, and owns the entire lifecycle of that application including monitoring, remediation, upgrades and everything.  In that context the data needs are simple - essentially those of a single Puppet module.

So the focus here is how we would create standalone management systems that, essentially, owns what one single Puppet module would own. More and more we are moving to single purpose nodes, containers, pods etc and I want to make something for that world.

So this Hiera, while being Hiera inspired, is quite different.  It is not orientated around single-key lookup but rather in resolving the entire data structure in one go and handing it back fully resolved.

The end goal is to have a CLI Configuration Management tool that allows for this:

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

From above we can see the end goal is a single, self-container, configuration management manifest that includes data and management.

## Status

This is now quite usable and full-featured, we might make some changes in future - like the name might change - but I welcome early adopter feedback.

TODO list:

 * [x] Move away from `${...}` to `{{ ... }}` this feels a bit more modern and aligns more with Choria
 * [x] Support [expr](https://expr-lang.org) to create hierarchy order
 * [x] Support interpolating data in values using [expr](https://expr-lang.org) 
 * [x] Once `expr` support lands support data types for interpolated values
 * [x] Add a `--query` flag to the CLI to dig into the resulting data
 * [x] Rename `configuration` to more generic `data`
 * [x] Move the overriding data from top level to `overrides`
 * [x] Support emitting environment variables as output format in the CLI
 * [x] CLI supports built-in system facts that can be optionally enabled
 * [x] CLI can use the environment as facts
 * [ ] Move to a dependency for deep merges, the implementation here is a bit meh
 
## Installation

Download the binaries from the release page, on MacOS you can use homebrew:

```
brew tap choria-io/tap
brew install choria-io/tap/tinyhiera
```

## Usage

### Hierarchy file format

Here is an annotated example of a hierarchy file:

```yaml
hierarchy:
    # this is the lookup and override order, facts will be resolved here
    #
    # if your fact is nested, you can use gjson format queries like via the lookup function {{ lookup('networking.fqdn') }}
    order:
     - env:{{ lookup('env') }}
     - role:{{ lookup('role') }}
     - host:{{ lookup('hostname') }}
    merge: deep # or first

# This is the resulting output and must be present, the hierarchy results will be merged in
data:
   log_level: INFO
   packages:
     - ca-certificates
   web:
     # we look up the number and convert its type to a int if the facts was not already an int
     listen_port: 80
     tls: false

overrides:
    env:prod:
      log_level: WARN

    role:web:
      packages:
        - nginx
      web:
        listen_port: 443
        tls: true

    host:web01:
      log_level: TRACE
```

See [GJSON Path Syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) for help in accessing nested facts. See [Expr Language Definition](https://expr-lang.org/docs/language-definition) for the query language

### CLI example

A small utility is provided to resolve a hierarchy file and a set of facts:

Given the input file `data.json`:

```json{
{
    "hierarchy": {
        "order": [
            "fqdn:{{ lookup('fqdn'}} }}"
        ]
    },
    "data": {
        "test": "value"
    },
    "overrides": {
        "fqdn:my.fqdn.com": {
            "test": "override"
        }
    }
}
```

We can run the utility like this:

```
$ tinyhiera parse data.json fqdn=my.fqdn.com
{
  "test": "override"
}
$ tinyhiera parse data.json fqdn=other.fqdn.com
{
  "test": "value"
}
```

It can also produce YAML output:

```
$ tinyhiera parse test.json fqdn=other.fqdn.com --yaml
test: value
```

It can also produce Environment Variable output:

```
$ tinyhiera parse test.json fqdn=other.fqdn.com --env
HIERA_TEST=value
```

In these examples we provided facts from a file or on the CLI, we can also populate the facts from an internal fact provider, first we view the internal facts:

```
$ tinyhiera facts --system-facts
{
  ....
  "host": {
      "info": {
          "hostname": "example.net",
          "uptime": 3725832,
          "bootTime": 1760351572,
          "procs": 625,
          "os": "darwin",
          "platform": "darwin",
          "platformFamily": "Standalone Workstation",
          "platformVersion": "15.7.1",
          "kernelVersion": "24.6.0",
          "kernelArch": "arm64",
          "virtualizationSystem": "",
          "virtualizationRole": ""
      }
  }
....
}
```

Now we resolve the data using those facts:

```
$ tinyhiera parse test.json --system-facts
```

We can also populate the environment variables as facts, variables will be split on the `=` and the variable name becomes a fact name.

```
$ tinyhiera parse test.json --env-facts
```

These facts will be merged with ones from the command line and external files and all can be combined

### Go example

Supply a YAML document and a map of facts. The resolver will parse the hierarchy, replace `{{ lookup('fact') }}` placeholders, and merge the matching sections.

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
     - env:{{ lookup('env') }}
     - role:{{ lookup('role') }}
     - host:{{ lookup('hostname') }}
   merge: deep

 data:
   log_level: INFO
   packages:
     - ca-certificates
   web:
     listen_port: 80
     tls: false

 overrides:
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
                "order": []any{"role:{{ lookup('role') }}"},
                "merge": "deep",
        },
        "data": map[string]any{
                "value": 1,
        },
		"overrides": map[string]any{
            "role:web": map[string]any{
                "value": 2,
            },
        }
}

resolved, err := tinyhiera.Resolve(config, map[string]any{"role": "web"})
if err != nil {
        panic(err)
}

fmt.Println(resolved)
// Output: map[value:2]
```
