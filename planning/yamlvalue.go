package planning

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter is decoded the way PyYAML's safe_load decodes it, because
// the docs were written against scripts/plan and scripts/asks: a later
// duplicate key wins instead of failing, yes/no/on/off are booleans, and
// dates stay strings that print as written. Values are nil, bool, int64,
// float64, string, []any or map[string]any.

// decodeYAML decodes one YAML document. An empty document is nil.
func decodeYAML(text string) (any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, nil
	}
	return nodeValue(doc.Content[0], 0)
}

var yaml11Bool = map[string]bool{
	"yes": true, "Yes": true, "YES": true, "on": true, "On": true, "ON": true,
	"no": false, "No": false, "NO": false, "off": false, "Off": false, "OFF": false,
	"y": true, "Y": true, "n": false, "N": false,
}

var pyFloat = regexp.MustCompile(`^[-+]?(?:[0-9][0-9_]*)\.[0-9_]*(?:[eE][-+][0-9]+)?$`)

func nodeValue(n *yaml.Node, depth int) (any, error) {
	if depth > 100 {
		return nil, fmt.Errorf("frontmatter nests too deeply")
	}
	switch n.Kind {
	case yaml.AliasNode:
		return nodeValue(n.Alias, depth+1)
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return nodeValue(n.Content[0], depth+1)
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := nodeValue(c, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.MappingNode:
		out := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, err := nodeValue(n.Content[i], depth+1)
			if err != nil {
				return nil, err
			}
			v, err := nodeValue(n.Content[i+1], depth+1)
			if err != nil {
				return nil, err
			}
			if n.Content[i].Tag == "!!merge" {
				if m, ok := v.(map[string]any); ok {
					for mk, mv := range m {
						if _, set := out[mk]; !set {
							out[mk] = mv
						}
					}
				}
				continue
			}
			out[pyStr(k)] = v
		}
		return out, nil
	case yaml.ScalarNode:
		return scalarValue(n), nil
	}
	return nil, nil
}

func scalarValue(n *yaml.Node) any {
	v := n.Value
	if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return v
	}
	switch n.Tag {
	case "!!null":
		return nil
	case "!!bool":
		return strings.EqualFold(v, "true")
	case "!!int":
		s := strings.ReplaceAll(v, "_", "")
		if i, err := strconv.ParseInt(s, 0, 64); err == nil {
			return i
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		return v
	case "!!float":
		if !pyFloat.MatchString(v) && !strings.Contains(strings.ToLower(v), "inf") && !strings.Contains(strings.ToLower(v), "nan") {
			return v // PyYAML wants a dot: 1e3 is a string there
		}
		s := strings.ToLower(strings.ReplaceAll(v, "_", ""))
		switch s {
		case ".inf", "+.inf":
			return math.Inf(1)
		case "-.inf":
			return math.Inf(-1)
		case ".nan":
			return math.NaN()
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return v
	case "!!str", "!!timestamp", "":
		if b, ok := yaml11Bool[v]; ok && n.Tag != "" {
			return b
		}
		return v
	}
	return v
}

// pyStr is Python's str() of a decoded value.
func pyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case float64:
		return pyFloatStr(x)
	case string:
		return x
	}
	return pyRepr(v)
}

func pyFloatStr(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

// pyRepr is Python's repr() of a decoded value.
func pyRepr(v any) string {
	switch x := v.(type) {
	case string:
		return pyStrRepr(x)
	case []string:
		parts := make([]string, len(x))
		for i, s := range x {
			parts[i] = pyStrRepr(s)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = pyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = pyStrRepr(k) + ": " + pyRepr(x[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return pyStr(v)
}

func pyStrRepr(s string) string {
	q := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		q = `"`
	}
	var b strings.Builder
	b.WriteString(q)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case string(r) == q:
			b.WriteString(`\` + q)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString(q)
	return b.String()
}

// truthy is Python's bool() of a decoded value.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int64:
		return x != 0
	case int:
		return x != 0
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case []string:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

// orDefault is Python's `v or d`.
func orDefault(v any, d any) any {
	if truthy(v) {
		return v
	}
	return d
}

// get is Python's dict.get(key) on a decoded mapping.
func get(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

// getOr is Python's dict.get(key, d): d only when the key is absent.
func getOr(m map[string]any, key string, d any) any {
	if v, ok := m[key]; ok {
		return v
	}
	return d
}

// pyEqual is Python's == between two decoded scalars.
func pyEqual(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case int64:
		switch y := b.(type) {
		case int64:
			return x == y
		case float64:
			return float64(x) == y
		}
	case float64:
		switch y := b.(type) {
		case float64:
			return x == y
		case int64:
			return x == float64(y)
		}
	}
	return false
}
