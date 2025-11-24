package tinyhiera

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolveYaml", func() {
	It("merges data deeply following hierarchy order", func() {
		yamlData := []byte(`
hierarchy:
  order:
    - global
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

		result, err := ResolveYaml(yamlData, facts)
		Expect(err).NotTo(HaveOccurred())

		Expect(result).To(Equal(map[string]any{
			"log_level": "TRACE",
			"packages":  []any{"ca-certificates", "nginx"},
			"web": map[string]any{
				"listen_port": 80,
				"tls":         true,
			},
		}))
	})

	It("returns the first matching overlay when using first merge mode", func() {
		yamlData := []byte(`
hierarchy:
  order:
    - env:{{ lookup('env') }}
    - role:{ lookup('role') }}
  merge: first

data:
  log_level: INFO

env:stage:
  log_level: DEBUG

role:web:
  log_level: WARN
`)

		facts := map[string]any{
			"env":  "stage",
			"role": "web",
		}

		result, err := ResolveYaml(yamlData, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"log_level": "DEBUG",
		}))
	})
})

var _ = Describe("Resolve", func() {
	It("processes an already parsed map without mutating input", func() {
		data := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"data", "role:{{ lookup('role') | lower() }}"},
				"merge": "deep",
			},
			"data": map[string]any{
				"value": 1,
			},
			"role:web": map[string]any{
				"list":  []any{float64(2)},
				"value": 2,
			},
		}

		facts := map[string]any{"role": "WEB"}

		result, err := Resolve(data, facts)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"value": 2,
			"list":  []any{2},
		}))

		Expect(data).To(Equal(map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"data", "role:{{ lookup('role') | lower() }}"},
				"merge": "deep",
			},
			"data": map[string]any{
				"value": 1,
			},
			"role:web": map[string]any{
				"list":  []any{float64(2)},
				"value": 2,
			},
		}))
	})
})

var _ = Describe("parseHierarchy", func() {
	It("extracts order and merge data", func() {
		// Ensures hierarchy parsing returns expected values when the structure is correct.
		root := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"global", "env:%{env}"},
				"merge": "deep",
			},
		}

		hierarchy, err := parseHierarchy(root)
		Expect(err).NotTo(HaveOccurred())
		Expect(hierarchy.Order).To(Equal([]string{"global", "env:%{env}"}))
		Expect(hierarchy.Merge).To(Equal("deep"))
	})

	It("returns an error when the hierarchy is malformed", func() {
		// Validates that bad hierarchy data is rejected early.
		root := map[string]any{
			"hierarchy": map[string]any{
				"order": []any{"global", 2},
			},
		}

		_, err := parseHierarchy(root)
		Expect(err).To(MatchError("hierarchy.order must contain only strings"))
	})
})

var _ = Describe("applyFacts", func() {
	It("replaces placeholders with fact values", func() {
		// Verifies templated segments are substituted when facts are available.
		result, err := applyFacts("role:{{ lookup('role') }}", map[string]any{"role": "web"})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("role:web"))
	})

	It("drops placeholders when facts are missing", func() {
		// Confirms missing fact keys result in empty substitutions.
		result, err := applyFacts("env:{{ lookup('unknown') }}", map[string]any{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("env:"))
	})

	It("Should support gjson lookups", func() {
		result, err := applyFacts("{{ lookup('node.fqdn') }}", map[string]any{"node": map[string]any{"fqdn": "example.com"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("example.com"))

		result, err = applyFacts("{{ lookup('node.foo') }}", map[string]any{"node": map[string]any{"fqdn": "example.com"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(""))
	})
})

var _ = Describe("normalizeNumericValues", func() {
	It("coerces compatible numeric types into int", func() {
		// Ensures whole numbers within int bounds are normalized to int for consistent merging.
		normalized := normalizeNumericValues(map[string]any{
			"float":    10.0,
			"int64":    int64(20),
			"uint64":   uint64(30),
			"inBounds": []any{float64(5)},
		})

		Expect(normalized).To(Equal(map[string]any{
			"float":    10,
			"int64":    20,
			"uint64":   30,
			"inBounds": []any{5},
		}))
	})
})

var _ = Describe("clone helpers", func() {
	It("clones maps deeply so mutations do not leak", func() {
		// Checks that modifying a cloned map leaves the original untouched.
		source := map[string]any{
			"nested": map[string]any{"value": 1},
			"list":   []any{1, 2},
		}

		cloned := cloneMap(source)
		cloned["nested"].(map[string]any)["value"] = 2
		cloned["list"].([]any)[0] = 99

		Expect(source).To(Equal(map[string]any{
			"nested": map[string]any{"value": 1},
			"list":   []any{1, 2},
		}))
	})

	It("deep merges maps without reusing source slices", func() {
		// Ensures deepMerge concatenates slices while maintaining isolation from inputs.
		target := map[string]any{
			"list": []any{1},
		}
		source := map[string]any{
			"list": []any{2},
		}

		merged := deepMerge(target, source)
		merged["list"].([]any)[0] = 42

		Expect(target["list"].([]any)).To(Equal([]any{1}))
		Expect(source["list"].([]any)).To(Equal([]any{2}))
		Expect(merged["list"].([]any)).To(Equal([]any{42, 2}))
	})
})
