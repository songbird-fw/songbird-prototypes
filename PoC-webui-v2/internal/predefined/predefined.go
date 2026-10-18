// Package predefined holds the objects shipped with Songbird (DM-21): the
// Color palette (R-055.1) and the system Services. They are embedded YAML,
// so the UI, the Validator and the factory default use the same list
// (songbird-yaml-schema open question 5).
package predefined

import (
	_ "embed"
	"fmt"
	"reflect"
	"strings"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

//go:embed palette.yaml
var paletteYAML []byte

//go:embed services.yaml
var servicesYAML []byte

var (
	palette  []m.Color
	services []m.Service
)

func init() {
	t := &m.Tree{}
	for _, f := range [][]byte{paletteYAML, servicesYAML} {
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
	palette, services = t.Colors, t.Services
}

// Palette returns a copy of the shipped Colors.
func Palette() []m.Color { return append([]m.Color(nil), palette...) }

// Services returns a copy of the shipped Services.
func Services() []m.Service {
	out := make([]m.Service, len(services))
	for i, s := range services {
		s.Spec.Protocols = append([]m.ProtocolEntry(nil), s.Spec.Protocols...)
		out[i] = s
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

// IsService reports whether a Service name is predefined.
func IsService(name string) bool {
	_, ok := service(name)
	return ok
}

func service(name string) (m.Service, bool) {
	for _, s := range services {
		if s.Metadata.Name == name {
			return s, true
		}
	}
	return m.Service{}, false
}

// ServiceMatches reports whether a Service keeps the defining fields of the
// predefined one with the same name (its spec); metadata is free.
func ServiceMatches(s m.Service) bool {
	p, ok := service(s.Metadata.Name)
	return !ok || reflect.DeepEqual(p.Spec, s.Spec)
}

// Describe renders a Service spec for messages: "tcp/443, udp/443".
func Describe(s m.ServiceSpec) string {
	var parts []string
	for _, p := range s.Protocols {
		switch {
		case p.Port != nil:
			parts = append(parts, p.Protocol+"/"+p.Port.String())
		case p.ICMPType != nil:
			parts = append(parts, fmt.Sprintf("%s type %d", p.Protocol, *p.ICMPType))
		case p.ProtocolNumber != nil:
			parts = append(parts, fmt.Sprintf("ip proto %d", *p.ProtocolNumber))
		default:
			parts = append(parts, p.Protocol)
		}
	}
	return strings.Join(parts, ", ")
}
