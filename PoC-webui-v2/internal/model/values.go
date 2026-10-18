package model

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// PortRange is common.schema.json#/$defs/portOrRange: a single port is a
// YAML number (443), a range a string ("10000-20000"). Lo > Hi is kept as
// read, so that the Validator can report it (DM-17).
type PortRange struct{ Lo, Hi int }

func (p PortRange) Single() bool { return p.Lo == p.Hi }

func (p PortRange) String() string {
	if p.Single() {
		return strconv.Itoa(p.Lo)
	}
	return fmt.Sprintf("%d-%d", p.Lo, p.Hi)
}

var rangeRe = regexp.MustCompile(`^([0-9]{1,5})-([0-9]{1,5})$`)

func parsePortString(s string) (PortRange, error) {
	m := rangeRe.FindStringSubmatch(s)
	if m == nil {
		return PortRange{}, fmt.Errorf("%q is not a port range like 10000-20000", s)
	}
	lo, _ := strconv.Atoi(m[1])
	hi, _ := strconv.Atoi(m[2])
	return PortRange{lo, hi}, nil
}

func (p PortRange) MarshalYAML() (any, error) {
	if p.Single() {
		return p.Lo, nil
	}
	return p.String(), nil
}

func (p *PortRange) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: port must be a number or a range string", n.Line)
	}
	if n.Tag == "!!int" {
		v, err := strconv.Atoi(n.Value)
		if err != nil {
			return err
		}
		*p = PortRange{v, v}
		return nil
	}
	r, err := parsePortString(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*p = r
	return nil
}

func (p PortRange) MarshalJSON() ([]byte, error) {
	if p.Single() {
		return json.Marshal(p.Lo)
	}
	return json.Marshal(p.String())
}

func (p *PortRange) UnmarshalJSON(b []byte) error {
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*p = PortRange{n, n}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("port must be a number or a range string")
	}
	r, err := parsePortString(s)
	if err != nil {
		return err
	}
	*p = r
	return nil
}

var durationRe = regexp.MustCompile(`^(?:([0-9]+)d)?(?:([0-9]+)h)?(?:([0-9]+)m)?(?:([0-9]+)s)?$`)

// ParseDuration parses common.schema.json#/$defs/duration ("30s", "12h", "7d").
func ParseDuration(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}
	m := durationRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	var d time.Duration
	for i, unit := range []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second} {
		if m[i+1] != "" {
			n, _ := strconv.Atoi(m[i+1])
			d += time.Duration(n) * unit
		}
	}
	return d, true
}

// NewUUID returns a random (version 4) UUID, as the schema's uuid pattern
// expects. It is assigned by the Config Manager on first save, never by hand.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
