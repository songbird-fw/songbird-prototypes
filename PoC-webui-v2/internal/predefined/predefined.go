// Package predefined holds the objects shipped with Songbird (DM-21): the
// 16-colour palette (R-055.1) and the predefined Addresses any / any6 and
// Service any (R-012.1, R-010.1). They are embedded YAML, so the UI, the
// Validator and the factory default use the same list (schema open
// question 5: "ship the palette as an embedded YAML").
package predefined

import (
	_ "embed"
	"fmt"
	"reflect"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

//go:embed palette.yaml
var paletteYAML []byte

//go:embed objects.yaml
var objectsYAML []byte

var (
	palette   []m.Color
	addresses []m.Address
	services  []m.Service
)

func init() {
	t := &m.Tree{}
	for _, f := range [][]byte{paletteYAML, objectsYAML} {
		docs, err := m.SplitDocs(f)
		if err != nil {
			panic(err)
		}
		for _, d := range docs {
			if _, err := t.PutYAML(d.Kind, d.Data); err != nil {
				panic(fmt.Sprintf("predefined %s/%s: %v", d.Kind, d.Name, err))
			}
		}
	}
	palette, addresses, services = t.Colors, t.Addresses, t.Services
}

// Palette returns a copy of the shipped Colors.
func Palette() []m.Color { return append([]m.Color(nil), palette...) }

// Addresses returns a copy of the shipped Addresses (any, any6).
func Addresses() []m.Address { return append([]m.Address(nil), addresses...) }

// Services returns a copy of the shipped Services (any).
func Services() []m.Service {
	out := make([]m.Service, len(services))
	for i, s := range services {
		s.Spec.Protocols = append([]m.ProtocolEntry(nil), s.Spec.Protocols...)
		out[i] = s
	}
	return out
}

// Docs returns every shipped object, for injection into a tree.
func Docs() []m.Doc {
	var out []m.Doc
	for _, c := range Palette() {
		out = append(out, c)
	}
	for _, a := range Addresses() {
		out = append(out, a)
	}
	for _, s := range Services() {
		out = append(out, s)
	}
	return out
}

// ColorHex returns the fixed hex of a predefined Color.
func ColorHex(name string) (string, bool) {
	for _, c := range palette {
		if c.Metadata.Name == name {
			return c.Spec.Hex, true
		}
	}
	return "", false
}

func PaletteNames() []string {
	out := make([]string, len(palette))
	for i, c := range palette {
		out[i] = c.Metadata.Name
	}
	return out
}

func AddressNames() []string {
	var out []string
	for _, a := range addresses {
		out = append(out, a.Metadata.Name)
	}
	return out
}

func ServiceNames() []string {
	var out []string
	for _, s := range services {
		out = append(out, s.Metadata.Name)
	}
	return out
}

// Address returns the shipped Address with that name.
func Address(name string) (m.Address, bool) {
	for _, a := range addresses {
		if a.Metadata.Name == name {
			return a, true
		}
	}
	return m.Address{}, false
}

// Service returns the shipped Service with that name.
func Service(name string) (m.Service, bool) {
	for _, s := range services {
		if s.Metadata.Name == name {
			return s, true
		}
	}
	return m.Service{}, false
}

// AddressMatches reports whether an Address keeps the spec of the shipped
// one with the same name (validate.py: predefined-immutable).
func AddressMatches(a m.Address) bool {
	p, ok := Address(a.Metadata.Name)
	return !ok || reflect.DeepEqual(p.Spec, a.Spec)
}

func ServiceMatches(s m.Service) bool {
	p, ok := Service(s.Metadata.Name)
	return !ok || reflect.DeepEqual(p.Spec, s.Spec)
}
