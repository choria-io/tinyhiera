package tinyhiera

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/tidwall/gjson"
)

// Hierarchy describes how data sections should be resolved.
type Hierarchy struct {
	// Order defines the lookup sequence for data sections.
	Order []string `yaml:"order"`
	// Merge selects the merge strategy ("first" or "deep").
	Merge string `yaml:"merge"`
}

var (
	// maxInt represents the largest int value for the current architecture and is used to safely normalize numeric types.
	maxInt = int(^uint(0) >> 1)
	// minInt represents the smallest int value for the current architecture and is used to safely normalize numeric types.
	minInt = -maxInt - 1
)

// Resolve consumes a parsed data document and a map of facts to produce a final data map.
// The data map is expected to contain a hierarchy section, a base data section, and any number of overlays.
// Placeholders in the hierarchy order (e.g. env:%{env}) are replaced with values from the provided facts map.
func Resolve(root map[string]any, facts map[string]any) (map[string]any, error) {
	normalizedRoot, ok := normalizeNumericValues(root).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("root document must be a map")
	}

	root = normalizedRoot

	hierarchy, err := parseHierarchy(root)
	if err != nil {
		return nil, err
	}

	base := map[string]any{}
	data, hasData := root["data"].(map[string]any)
	if hasData {
		base = cloneMap(data)
	}

	mergeMode := strings.ToLower(hierarchy.Merge)
	if mergeMode == "" {
		mergeMode = "first"
	}

	for _, entry := range hierarchy.Order {
		resolvedKey := applyFacts(entry, facts)
		candidateKey := resolvedKey
		if candidateKey == "global" {
			if _, ok := root[candidateKey]; !ok {
				candidateKey = "data"
			}
		}
		if candidateKey == "data" && hasData {
			continue
		}
		candidate, ok := root[candidateKey].(map[string]any)
		if !ok {
			continue
		}

		switch mergeMode {
		case "deep":
			base = deepMerge(base, candidate)
		case "first":
			base = shallowMerge(base, candidate)
			return base, nil
		default:
			return nil, fmt.Errorf("unsupported merge mode: %s", mergeMode)
		}
	}

	return base, nil
}

// ResolveYaml consumes raw YAML bytes and a map of facts to produce a final data map.
// The function decodes the YAML document and delegates processing to Resolve to perform merges and fact substitution.
func ResolveYaml(data []byte, facts map[string]any) (map[string]any, error) {
	root := map[string]any{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return Resolve(root, facts)
}

// ResolveJson consumes raw JSON bytes and a map of facts to produce a final data map.
// The function decodes the JSON document and delegates processing to Resolve to perform merges and fact substitution.
func ResolveJson(data []byte, facts map[string]any) (map[string]any, error) {
	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return Resolve(root, facts)
}

// parseHierarchy extracts the hierarchy definition from the raw YAML map.
func parseHierarchy(root map[string]any) (Hierarchy, error) {
	raw, ok := root["hierarchy"].(map[string]any)
	if !ok {
		return Hierarchy{}, fmt.Errorf("hierarchy section is required")
	}

	orderSlice, ok := raw["order"].([]any)
	if !ok {
		return Hierarchy{}, fmt.Errorf("hierarchy.order must be a list")
	}

	order := make([]string, 0, len(orderSlice))
	for _, item := range orderSlice {
		text, ok := item.(string)
		if !ok {
			return Hierarchy{}, fmt.Errorf("hierarchy.order must contain only strings")
		}
		order = append(order, text)
	}

	mergeMode, _ := raw["merge"].(string)

	return Hierarchy{Order: order, Merge: mergeMode}, nil
}

// applyFacts replaces %{fact} placeholders with concrete values from the provided facts map.
// A special literal function allows emitting templated delimiters without triggering further substitution,
// for example %{literal('%')}{SERVER_NAME} becomes %{SERVER_NAME}.
func applyFacts(template string, facts map[string]any) string {
	re := regexp.MustCompile(`%\{([^}]+)\}`)

	parseLiteral := func(key string) (string, bool) {
		if !strings.HasPrefix(key, "literal(") || !strings.HasSuffix(key, ")") {
			return "", false
		}

		inner := strings.TrimSuffix(strings.TrimPrefix(key, "literal("), ")")
		if len(inner) < 2 {
			return "", false
		}

		quote := inner[0]
		if (quote != '\'' && quote != '"') || inner[len(inner)-1] != quote {
			return "", false
		}

		return inner[1 : len(inner)-1], true
	}

	factJson, err := json.Marshal(facts)
	if err != nil {
		return template
	}

	return re.ReplaceAllStringFunc(template, func(match string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(match, "%{"), "}")
		if literalValue, ok := parseLiteral(key); ok {
			return literalValue
		}

		factsValue := gjson.GetBytes(factJson, key)
		if factsValue.Exists() {
			return factsValue.String()
		}
		return ""
	})
}

// normalizeNumericValues walks a decoded YAML structure and converts numeric values into int when they safely fit the
// platform size. This keeps numeric handling consistent with the expectations of the resolver and its tests.
func normalizeNumericValues(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			result[key] = normalizeNumericValues(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, val := range typed {
			result[i] = normalizeNumericValues(val)
		}
		return result
	case int64:
		if typed >= int64(minInt) && typed <= int64(maxInt) {
			return int(typed)
		}
		return typed
	case uint64:
		if typed <= uint64(maxInt) {
			return int(typed)
		}
		return typed
	case float64:
		if typed == math.Trunc(typed) && typed >= float64(minInt) && typed <= float64(maxInt) {
			return int(typed)
		}
		return typed
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case uint:
		if typed <= uint(maxInt) {
			return int(typed)
		}
		return typed
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		if typed <= uint32(maxInt) {
			return int(typed)
		}
		return typed
	default:
		return typed
	}
}

// shallowMerge merges source keys into target without recursion.
func shallowMerge(target, source map[string]any) map[string]any {
	result := cloneMap(target)
	for key, value := range source {
		result[key] = cloneValue(value)
	}
	return result
}

// deepMerge merges source maps into target recursively. Map values are merged, slices are concatenated, and other values override.
func deepMerge(target, source map[string]any) map[string]any {
	result := cloneMap(target)
	for key, value := range source {
		if existing, ok := result[key]; ok {
			switch existingTyped := existing.(type) {
			case map[string]any:
				if incomingMap, ok := value.(map[string]any); ok {
					result[key] = deepMerge(existingTyped, incomingMap)
					continue
				}
			case []any:
				if incomingSlice, ok := value.([]any); ok {
					combined := append(cloneSlice(existingTyped), incomingSlice...)
					result[key] = combined
					continue
				}
			}
		}
		result[key] = cloneValue(value)
	}
	return result
}

// cloneMap creates a shallow copy of the provided map with cloned values.
func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneValue(value)
	}
	return result
}

// cloneSlice returns a shallow copy of a slice with cloned elements.
func cloneSlice(source []any) []any {
	result := make([]any, len(source))
	for i, value := range source {
		result[i] = cloneValue(value)
	}
	return result
}

// cloneValue duplicates maps and slices to avoid mutating caller state.
func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	default:
		return typed
	}
}
