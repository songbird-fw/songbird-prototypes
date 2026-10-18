package model

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// MarshalObject renders one object in the canonical serialisation of the
// schema (design decision 12): every string value double-quoted, numbers
// and booleans bare, keys unquoted, 2-space indent, block style with inline
// [] for lists of scalars and inline {} for leaf maps inside lists. This is
// what the Config Manager commits, so diffs are stable.
func MarshalObject(o any) ([]byte, error) {
	var n yaml.Node
	if err := n.Encode(o); err != nil {
		return nil, err
	}
	canonical(&n, false)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func canonical(n *yaml.Node, inSeq bool) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			canonical(c, false)
		}
	case yaml.MappingNode:
		leaf := true
		for i := 1; i < len(n.Content); i += 2 {
			v := n.Content[i]
			if v.Kind != yaml.ScalarNode {
				leaf = false
			}
			canonical(v, false)
		}
		if inSeq && leaf {
			n.Style = yaml.FlowStyle
		}
	case yaml.SequenceNode:
		scalars := true
		for _, c := range n.Content {
			if c.Kind != yaml.ScalarNode {
				scalars = false
			}
			canonical(c, true)
		}
		if scalars {
			n.Style = yaml.FlowStyle
		}
	case yaml.ScalarNode:
		if n.Tag == "!!str" {
			n.Style = yaml.DoubleQuotedStyle
		}
	}
}

// RawDoc is one YAML document of a file, before it is typed.
type RawDoc struct {
	Index int    // position in a multi-document file
	Line  int    // first line of the document
	Kind  string // as declared
	Name  string // metadata.name as declared
	Node  *yaml.Node
	Value any    // JSON-compatible value, for the JSON Schema validator
	Data  []byte // the document alone, re-encoded
}

// SplitDocs parses a (possibly multi-document) YAML file.
func SplitDocs(data []byte) ([]RawDoc, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var out []RawDoc
	for i := 0; ; i++ {
		var n yaml.Node
		err := dec.Decode(&n)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		if len(n.Content) == 0 || (n.Content[0].Kind == yaml.ScalarNode && n.Content[0].Tag == "!!null") {
			continue
		}
		var v any
		if err := n.Decode(&v); err != nil {
			return out, fmt.Errorf("document %d: %w", i, err)
		}
		d := RawDoc{Index: i, Line: n.Content[0].Line, Node: &n, Value: jsonCompatible(v)}
		if m, ok := d.Value.(map[string]any); ok {
			d.Kind, _ = m["kind"].(string)
			if md, ok := m["metadata"].(map[string]any); ok {
				d.Name, _ = md["name"].(string)
			}
		}
		d.Data, _ = yaml.Marshal(&n)
		out = append(out, d)
	}
}

func jsonCompatible(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = jsonCompatible(e)
		}
		return x
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[fmt.Sprint(k)] = jsonCompatible(e)
		}
		return m
	case []any:
		for i, e := range x {
			x[i] = jsonCompatible(e)
		}
		return x
	}
	return v
}

// SplitPath turns "spec.rules[0].nat" into ["spec", "rules", "0", "nat"].
func SplitPath(p string) []string {
	p = strings.ReplaceAll(p, "[", ".")
	p = strings.ReplaceAll(p, "]", "")
	var out []string
	for _, s := range strings.Split(p, ".") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// LineOf returns the line of the deepest node of path that exists in the
// document (R-020.3: errors name file, line and path).
func LineOf(doc *yaml.Node, path string) int {
	n := doc
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	line := n.Line
	for _, seg := range SplitPath(path) {
		var next *yaml.Node
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == seg {
					line = n.Content[i].Line
					next = n.Content[i+1]
					break
				}
			}
		case yaml.SequenceNode:
			if k, err := strconv.Atoi(seg); err == nil && k < len(n.Content) {
				next = n.Content[k]
			}
		}
		if next == nil {
			return line
		}
		n = next
		line = n.Line
	}
	return line
}

// LineOfObject finds a path in the canonical file of an object.
func LineOfObject(o any, path string) int {
	b, err := MarshalObject(o)
	if err != nil {
		return 0
	}
	var n yaml.Node
	if yaml.Unmarshal(b, &n) != nil {
		return 0
	}
	return LineOf(&n, path)
}
