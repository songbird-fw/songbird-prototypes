// Package schema is layer 1 of validation: every document is checked
// against the v1alpha1 JSON Schema, the normative artifact of the
// configuration format (C-005, R-020.2). v1alpha1/ is a verbatim copy of
// songbird-fw/schema at commit 99bfafe; ext/ holds the PoC proposals for
// the kinds the schema does not cover yet (StaticRoute, User).
//
// The schema README names santhosh-tekuri/jsonschema as the Go library for
// this layer. The patterns use ECMAScript lookaheads, so the regexp engine
// is regexp2 in ECMAScript mode.
package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed v1alpha1/*.json ext/*.json
var files embed.FS

const base = "https://config.songbird-fw.com/v1alpha1/"

// Extensions are the kinds validated against a PoC proposal in ext/.
var Extensions = map[string]string{
	"StaticRoute": "staticroute.schema.json",
	"User":        "user.schema.json",
}

// Issue is one structural error: where (path in the document) and why.
type Issue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type re2 struct{ *regexp2.Regexp }

func (r re2) MatchString(s string) bool {
	ok, err := r.Regexp.MatchString(s)
	return err == nil && ok
}

var (
	once     sync.Once
	root     *jsonschema.Schema
	ext      = map[string]*jsonschema.Schema{}
	initErr  error
	printer  = message.NewPrinter(language.English)
	rawFiles = map[string][]byte{}
)

func load() {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseRegexpEngine(func(s string) (jsonschema.Regexp, error) {
		r, err := regexp2.Compile(s, regexp2.ECMAScript)
		if err != nil {
			return nil, err
		}
		return re2{r}, nil
	})
	err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		name := p[strings.LastIndex(p, "/")+1:]
		rawFiles[name] = b
		return c.AddResource(base+name, doc)
	})
	if err != nil {
		initErr = err
		return
	}
	if root, initErr = c.Compile(base + "songbird.schema.json"); initErr != nil {
		return
	}
	for kind, f := range Extensions {
		s, err := c.Compile(base + f)
		if err != nil {
			initErr = err
			return
		}
		ext[kind] = s
	}
}

// Files returns the embedded schema files by name (served to the UI so the
// YAML editor can link them).
func Files() map[string][]byte {
	once.Do(load)
	return rawFiles
}

// Validate checks one document (any JSON-compatible value, e.g. a decoded
// YAML document or a JSON request body). It returns the most specific
// error of each failing branch, deepest path first.
func Validate(doc any) []Issue {
	once.Do(load)
	if initErr != nil {
		return []Issue{{Reason: "schema not loaded: " + initErr.Error()}}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return []Issue{{Reason: err.Error()}}
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return []Issue{{Reason: err.Error()}}
	}
	s := root
	if m, ok := v.(map[string]any); ok {
		if k, _ := m["kind"].(string); ext[k] != nil {
			s = ext[k]
		}
	}
	err = s.Validate(v)
	if err == nil {
		return nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []Issue{{Reason: err.Error()}}
	}
	return issues(ve)
}

// ValidateObject validates a typed object as it would be written.
func ValidateObject(o any) []Issue {
	b, err := json.Marshal(o)
	if err != nil {
		return []Issue{{Reason: err.Error()}}
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return []Issue{{Reason: err.Error()}}
	}
	return Validate(v)
}

// issues keeps the deepest leaf errors, like validate.py, so that a oneOf
// or if/then failure is reported where the value is wrong, not at the root.
func issues(ve *jsonschema.ValidationError) []Issue {
	var leaves []*jsonschema.ValidationError
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			leaves = append(leaves, e)
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	depth := 0
	for _, l := range leaves {
		depth = max(depth, len(l.InstanceLocation))
	}
	seen := map[string]bool{}
	var out []Issue
	for _, l := range leaves {
		if len(l.InstanceLocation) != depth {
			continue
		}
		is := Issue{Path: path(l.InstanceLocation), Reason: explain(l.SchemaURL, l.ErrorKind.LocalizedString(printer))}
		if k := seenKey(is); !seen[k] {
			seen[k] = true
			out = append(out, is)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	if len(out) > 3 { // a oneOf with many branches: keep it readable
		out = out[:3]
	}
	return out
}

// seenKey keeps one issue per path: for anyOf (e.g. IPv4 or IPv6) the
// first branch is the expected one.
func seenKey(i Issue) string { return i.Path }

// hints turn the generic messages of 'not' and 'oneOf' into the rule the
// schema expresses there; keys are suffixes of the failing schema location.
var hints = []struct{ suffix, text string }{
	{"common.schema.json#/$defs/objectName/not", "'none' is a reserved name"},
	{"ruleset.schema.json#/$defs/rule/allOf/0/then/not", "a drop rule cannot have nat"},
	{"ruleset.schema.json#/$defs/nat/allOf/1/then/not", "translateServiceTo is allowed only with nat type dnat"},
	{"color.schema.json#/properties/metadata/not", "a Color cannot itself have a color"},
	{"address.schema.json#/properties/spec/oneOf", "exactly one of subnet, range, fqdn, mac must be set"},
	{"service.schema.json#/$defs/protocolEntry/allOf/0/then/not", "tcp/udp/sctp take port only (sourcePort was removed, schema decision 17)"},
	{"service.schema.json#/$defs/protocolEntry/allOf/1/then/not", "icmp/icmpv6 take icmpType and icmpCode only, no port"},
	{"service.schema.json#/$defs/protocolEntry/allOf/2/then/not", "ip takes protocolNumber only"},
	{"service.schema.json#/$defs/protocolEntry/allOf/3/then/not", "protocol any takes no port, icmpType or protocolNumber"},
	{"interface.schema.json#/properties/spec/allOf/0/then/not", "a physical Interface has neither parent nor vlanId"},
	{"interface.schema.json#/properties/spec/allOf/1/then/not", "a VLAN Interface has no device: it is named after its parent"},
	{"interface.schema.json#/$defs/ipv4Config/allOf/1/then/not", "addresses and gateway are allowed only with mode static"},
	{"interface.schema.json#/$defs/ipv6Config/allOf/1/then/not", "addresses and gateway are allowed only with mode static"},
}

func explain(schemaURL, msg string) string {
	// SchemaURL locates the schema that owns the failing keyword.
	switch {
	case strings.Contains(msg, "'not' failed"):
		schemaURL += "/not"
	case strings.Contains(msg, "'oneOf' failed"):
		schemaURL += "/oneOf"
	}
	for _, h := range hints {
		if strings.HasSuffix(schemaURL, h.suffix) {
			return h.text
		}
	}
	if strings.Contains(msg, "does not match pattern") {
		for _, p := range patterns {
			if strings.HasSuffix(schemaURL, p.suffix) {
				return p.text
			}
		}
	}
	return msg
}

// patterns replace "does not match pattern '<regexp>'" by what is expected.
var patterns = []struct{ suffix, text string }{
	{"system.schema.json#/$defs/encrypted", "secrets must use the ENC[...] envelope, never clear text (R-020.5)"},
	{"common.schema.json#/$defs/objectName", "names are lowercase letters, digits, '.', '_' and '-', starting and ending with a letter or digit (max 63)"},
	{"common.schema.json#/$defs/ipv4Cidr", "not an IPv4 subnet in CIDR notation like 192.168.0.0/24 (netmask notation is not accepted)"},
	{"common.schema.json#/$defs/ipv6Cidr", "not an IPv6 subnet in CIDR notation"},
	{"common.schema.json#/$defs/ipv4Address", "not an IPv4 address"},
	{"common.schema.json#/$defs/colorHex", "not a colour like #2E7D32"},
	{"common.schema.json#/$defs/duration", "not a duration like 30s, 10m, 8h or 7d"},
	{"common.schema.json#/$defs/macAddress", "not a MAC address like 34:29:8f:74:f4:6a"},
	{"common.schema.json#/$defs/portOrRange/anyOf/1", "a port range is a string like \"10000-20000\""},
}

// path renders an instance location as spec.rules[0].nat.
func path(loc []string) string {
	var b strings.Builder
	for _, s := range loc {
		if s != "" && strings.Trim(s, "0123456789") == "" {
			b.WriteString("[" + s + "]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s)
	}
	if b.Len() == 0 {
		return "<root>"
	}
	return b.String()
}
