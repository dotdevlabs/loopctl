package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const refPrefix = "#/"

var (
	specOnce sync.Once
	specDoc  map[string]any
	specErr  error
)

// specRoot parses the embedded spec copy once.
func specRoot() (map[string]any, error) {
	specOnce.Do(func() {
		var n yaml.Node
		if specErr = yaml.Unmarshal(fixtureData, &n); specErr != nil {
			return
		}
		// The spec copy repeats a few mapping keys, which yaml refuses to
		// decode into a map, so convert the node tree directly (first wins).
		m, ok := nodeValue(&n).(map[string]any)
		if !ok {
			specErr = fmt.Errorf("spec copy is not a mapping")
			return
		}
		specDoc = m
	})
	return specDoc, specErr
}

// SpecEnum returns the enum values at the JSON pointer ref (for example
// "#/components/schemas/TodoAttributes/properties/status") in the embedded
// spec copy. Where the pointer crosses a oneOf, the values of every
// alternative are joined in order of appearance.
func SpecEnum(ref string) ([]string, error) {
	root, err := specRoot()
	if err != nil {
		return nil, err
	}
	nodes, err := resolveAll(root, root, ref, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for _, n := range nodes {
		vals, ok := n["enum"].([]any)
		if !ok {
			return nil, fmt.Errorf("spec %s: no enum", ref)
		}
		for _, v := range vals {
			s := fmt.Sprint(v)
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// SpecSchema returns the JSON Schema at the JSON pointer ref of the embedded
// spec copy, with $ref inlined, and with descriptions and x-loopcontrol-*
// extensions removed. When array is true the schema is wrapped as an array of
// the referenced schema.
func SpecSchema(ref string, array bool) (string, error) {
	root, err := specRoot()
	if err != nil {
		return "", err
	}
	nodes, err := resolveAll(root, root, ref, 0)
	if err != nil {
		return "", err
	}
	if len(nodes) != 1 {
		return "", fmt.Errorf("spec %s: ambiguous schema", ref)
	}
	out, err := export(root, nodes[0], 0)
	if err != nil {
		return "", fmt.Errorf("spec %s: %w", ref, err)
	}
	if array {
		out = map[string]any{"type": "array", "items": out}
	}
	b, err := json.Marshal(out)
	return string(b), err
}

const maxDepth = 32

// resolveAll walks the pointer from node, following $ref and fanning out
// across oneOf alternatives when a segment is absent from the node itself.
func resolveAll(root map[string]any, node any, ref string, depth int) ([]map[string]any, error) {
	if !strings.HasPrefix(ref, refPrefix) {
		return nil, fmt.Errorf("spec ref %q must start with %q", ref, refPrefix)
	}
	cur := []any{root}
	for _, seg := range strings.Split(strings.TrimPrefix(ref, refPrefix), "/") {
		var next []any
		for _, c := range cur {
			found, err := step(root, c, seg, depth)
			if err != nil {
				return nil, err
			}
			next = append(next, found...)
		}
		if len(next) == 0 {
			return nil, fmt.Errorf("spec ref %q: %q not found", ref, seg)
		}
		cur = next
	}
	out := make([]map[string]any, 0, len(cur))
	for _, c := range cur {
		m, ok := c.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("spec ref %q is not a schema object", ref)
		}
		out = append(out, m)
	}
	return out, nil
}

func step(root map[string]any, c any, seg string, depth int) ([]any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("$ref nesting exceeds %d", maxDepth)
	}
	m, ok := c.(map[string]any)
	if !ok {
		return nil, nil
	}
	if v, ok := m[seg]; ok {
		return []any{v}, nil
	}
	if r, ok := m["$ref"].(string); ok {
		targets, err := resolveAll(root, root, r, depth+1)
		if err != nil {
			return nil, err
		}
		var out []any
		for _, t := range targets {
			found, err := step(root, t, seg, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, found...)
		}
		return out, nil
	}
	alts, _ := m["oneOf"].([]any)
	var out []any
	for _, a := range alts {
		found, err := step(root, a, seg, depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

// export converts a spec schema node to plain JSON Schema.
func export(root map[string]any, node map[string]any, depth int) (map[string]any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("$ref nesting exceeds %d", maxDepth)
	}
	if r, ok := node["$ref"].(string); ok {
		targets, err := resolveAll(root, root, r, depth+1)
		if err != nil {
			return nil, err
		}
		return export(root, targets[0], depth+1)
	}
	out := map[string]any{}
	for k, v := range node {
		if k == "description" || strings.HasPrefix(k, "x-") {
			continue
		}
		switch k {
		case "properties":
			props := map[string]any{}
			for name, p := range v.(map[string]any) {
				pm, ok := p.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("property %q is not a schema", name)
				}
				e, err := export(root, pm, depth+1)
				if err != nil {
					return nil, err
				}
				props[name] = e
			}
			out[k] = props
		case "items":
			im, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("items is not a schema")
			}
			e, err := export(root, im, depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = e
		case "additionalProperties":
			if am, ok := v.(map[string]any); ok {
				e, err := export(root, am, depth+1)
				if err != nil {
					return nil, err
				}
				v = e
			}
			out[k] = v
		default:
			out[k] = v
		}
	}
	return out, nil
}

// nodeValue converts a YAML node to generic maps, slices and scalars. The first
// occurrence of a repeated mapping key wins.
func nodeValue(n *yaml.Node) any {
	switch n.Kind {
	case yaml.DocumentNode:
		return nodeValue(n.Content[0])
	case yaml.AliasNode:
		return nodeValue(n.Alias)
	case yaml.MappingNode:
		m := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if _, dup := m[n.Content[i].Value]; !dup {
				m[n.Content[i].Value] = nodeValue(n.Content[i+1])
			}
		}
		return m
	case yaml.SequenceNode:
		s := make([]any, len(n.Content))
		for i, c := range n.Content {
			s[i] = nodeValue(c)
		}
		return s
	}
	var v any
	_ = n.Decode(&v)
	return v
}
