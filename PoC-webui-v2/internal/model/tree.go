package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Kind describes one object kind: its repository directory and namespace.
type Kind struct {
	Name      string
	Dir       string // repository directory, one file per object ("" = the root)
	Namespace string // names are unique within a namespace (DM-01)
	Extension bool   // not in the v1alpha1 schema (PoC proposal)
}

// Kinds in dependency order. The directories are the canonical export
// layout of the schema (design decision 14): system.yaml at the root and one
// kebab-case plural directory per kind, file name = metadata.name. The two
// PoC extension kinds follow the same convention.
var Kinds = []Kind{
	{"System", "", "system", false},
	{"Color", "colors", "color", false},
	{"Interface", "interfaces", "network", false},
	{"Zone", "zones", "network", false},
	{"StaticRoute", "static-routes", "route", true},
	{"Address", "addresses", "address", false},
	{"AddressGroup", "address-groups", "address", false},
	{"Service", "services", "service", false},
	{"ServiceGroup", "service-groups", "service", false},
	{"RuleSet", "rulesets", "ruleset", false},
	{"User", "users", "user", true},
}

func KindByName(name string) (Kind, bool) {
	for _, k := range Kinds {
		if k.Name == name {
			return k, true
		}
	}
	return Kind{}, false
}

// FilePath is the repository path of an object.
func FilePath(kind, name string) string {
	k, ok := KindByName(kind)
	if !ok {
		return kind + "/" + name + ".yaml"
	}
	if k.Dir == "" {
		return name + ".yaml"
	}
	return k.Dir + "/" + name + ".yaml"
}

// Doc is implemented by every object.
type Doc interface {
	Header() (kind string, meta Meta)
}

func (o Object[S]) Header() (string, Meta) { return o.Kind, o.Metadata }

// ---- Tree ------------------------------------------------------------------------

// Tree is a complete configuration: the desired state at one revision.
type Tree struct {
	System        List[SystemSpec]      `json:"System"`
	Colors        List[ColorSpec]       `json:"Color"`
	Interfaces    List[InterfaceSpec]   `json:"Interface"`
	Zones         List[ZoneSpec]        `json:"Zone"`
	StaticRoutes  List[StaticRouteSpec] `json:"StaticRoute"`
	Addresses     List[AddressSpec]     `json:"Address"`
	AddressGroups List[GroupSpec]       `json:"AddressGroup"`
	Services      List[ServiceSpec]     `json:"Service"`
	ServiceGroups List[GroupSpec]       `json:"ServiceGroup"`
	RuleSets      List[RuleSetSpec]     `json:"RuleSet"`
	Users         List[UserSpec]        `json:"User"`
}

type List[S any] []Object[S]

type anyList interface {
	putJSON(kind string, data []byte) (string, error)
	putYAML(kind string, data []byte) (string, error)
	putObj(o any) bool
	remove(name string) bool
	find(name string) (any, bool)
	sortByName()
	names() []string
	each(fn func(Doc))
}

func (l *List[S]) put(o Object[S]) {
	for i := range *l {
		if (*l)[i].Metadata.Name == o.Metadata.Name {
			(*l)[i] = o
			return
		}
	}
	*l = append(*l, o)
}

func (l *List[S]) putObj(o any) bool {
	x, ok := o.(Object[S])
	if ok {
		l.put(x)
	}
	return ok
}

func (l *List[S]) putJSON(kind string, data []byte) (string, error) {
	var o Object[S]
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&o); err != nil {
		return "", fmt.Errorf("invalid %s: %w", kind, err)
	}
	o.APIVersion, o.Kind = APIVersion, kind
	normalize(&o)
	l.put(o)
	return o.Metadata.Name, nil
}

func (l *List[S]) putYAML(kind string, data []byte) (string, error) {
	var o Object[S]
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&o); err != nil {
		return "", fmt.Errorf("invalid %s: %w", kind, err)
	}
	if o.Kind != kind {
		return "", fmt.Errorf("document declares kind %q, expected %q", o.Kind, kind)
	}
	normalize(&o)
	l.put(o)
	return o.Metadata.Name, nil
}

// normalize makes required-but-empty lists explicit, so JSON and YAML
// render [] and never null.
func normalize[S any](o *Object[S]) {
	switch s := any(&o.Spec).(type) {
	case *RuleSetSpec:
		if s.Rules == nil {
			s.Rules = []Rule{}
		}
		for i := range s.Rules {
			r := &s.Rules[i]
			for _, l := range []*[]string{&r.Source, &r.Destination, &r.Service} {
				if *l == nil {
					*l = []string{}
				}
			}
		}
	case *GroupSpec:
		if s.Members == nil {
			s.Members = []string{}
		}
	case *ZoneSpec:
		if s.Interfaces == nil {
			s.Interfaces = []string{}
		}
	case *ServiceSpec:
		if s.Protocols == nil {
			s.Protocols = []ProtocolEntry{}
		}
	}
}

func (l *List[S]) remove(name string) bool {
	for i := range *l {
		if (*l)[i].Metadata.Name == name {
			*l = append((*l)[:i], (*l)[i+1:]...)
			return true
		}
	}
	return false
}

func (l *List[S]) find(name string) (any, bool) {
	for _, o := range *l {
		if o.Metadata.Name == name {
			return o, true
		}
	}
	return nil, false
}

func (l *List[S]) sortByName() {
	sort.SliceStable(*l, func(i, j int) bool { return (*l)[i].Metadata.Name < (*l)[j].Metadata.Name })
}

func (l *List[S]) names() []string {
	out := make([]string, len(*l))
	for i, o := range *l {
		out[i] = o.Metadata.Name
	}
	return out
}

func (l *List[S]) each(fn func(Doc)) {
	for _, o := range *l {
		fn(o)
	}
}

func (t *Tree) list(kind string) anyList {
	switch kind {
	case "System":
		return &t.System
	case "Color":
		return &t.Colors
	case "Interface":
		return &t.Interfaces
	case "Zone":
		return &t.Zones
	case "StaticRoute":
		return &t.StaticRoutes
	case "Address":
		return &t.Addresses
	case "AddressGroup":
		return &t.AddressGroups
	case "Service":
		return &t.Services
	case "ServiceGroup":
		return &t.ServiceGroups
	case "RuleSet":
		return &t.RuleSets
	case "User":
		return &t.Users
	}
	return nil
}

// Names lists the object names of one kind.
func (t *Tree) Names(kind string) []string {
	if l := t.list(kind); l != nil {
		return l.names()
	}
	return nil
}

func (t *Tree) PutJSON(kind string, data []byte) (string, error) {
	l := t.list(kind)
	if l == nil {
		return "", fmt.Errorf("unknown kind %q", kind)
	}
	return l.putJSON(kind, data)
}

func (t *Tree) PutYAML(kind string, data []byte) (string, error) {
	l := t.list(kind)
	if l == nil {
		return "", fmt.Errorf("unknown kind %q", kind)
	}
	return l.putYAML(kind, data)
}

// Put stores a typed object (Object[S] of a known kind).
func (t *Tree) Put(o Doc) error {
	kind, _ := o.Header()
	l := t.list(kind)
	if l == nil || !l.putObj(o) {
		return fmt.Errorf("cannot store %T as %s", o, kind)
	}
	return nil
}

func (t *Tree) Delete(kind, name string) bool {
	if l := t.list(kind); l != nil {
		return l.remove(name)
	}
	return false
}

func (t *Tree) Get(kind, name string) (Doc, bool) {
	if l := t.list(kind); l != nil {
		if o, ok := l.find(name); ok {
			return o.(Doc), true
		}
	}
	return nil, false
}

// Each visits every object in kind order.
func (t *Tree) Each(fn func(Doc)) {
	for _, k := range Kinds {
		t.list(k.Name).each(fn)
	}
}

func (t *Tree) Sort() {
	for _, k := range Kinds {
		t.list(k.Name).sortByName()
	}
}

// Clone returns a deep copy.
func (t *Tree) Clone() *Tree {
	b, _ := json.Marshal(t)
	var c Tree
	_ = json.Unmarshal(b, &c)
	return &c
}

// Sys returns the System singleton, or an empty spec when it is missing.
func (t *Tree) Sys() SystemSpec {
	if len(t.System) > 0 {
		return t.System[0].Spec
	}
	return SystemSpec{}
}

// Lookups used by the Validator, the Compiler and the simulators.

func (t *Tree) Interface(name string) (Interface, bool) {
	for _, o := range t.Interfaces {
		if o.Metadata.Name == name {
			return o, true
		}
	}
	return Interface{}, false
}

func (t *Tree) Zone(name string) (Zone, bool) {
	for _, o := range t.Zones {
		if o.Metadata.Name == name {
			return o, true
		}
	}
	return Zone{}, false
}

func (t *Tree) Address(name string) (Address, bool) {
	for _, o := range t.Addresses {
		if o.Metadata.Name == name {
			return o, true
		}
	}
	return Address{}, false
}

func (t *Tree) AddressGroup(name string) (AddressGroup, bool) {
	for _, o := range t.AddressGroups {
		if o.Metadata.Name == name {
			return o, true
		}
	}
	return AddressGroup{}, false
}

func (t *Tree) Service(name string) (Service, bool) {
	for _, o := range t.Services {
		if o.Metadata.Name == name {
			return o, true
		}
	}
	return Service{}, false
}

func (t *Tree) ServiceGroup(name string) (ServiceGroup, bool) {
	for _, o := range t.ServiceGroups {
		if o.Metadata.Name == name {
			return o, true
		}
	}
	return ServiceGroup{}, false
}

// ZoneOf returns the Zone an Interface belongs to ("" if none, DM-37).
func (t *Tree) ZoneOf(iface string) string {
	for _, z := range t.Zones {
		for _, i := range z.Spec.Interfaces {
			if i == iface {
				return z.Metadata.Name
			}
		}
	}
	return ""
}

// Side expands a RuleSet side (Interface or Zone) to its Interfaces.
func (t *Tree) Side(ref string) []string {
	if z, ok := t.Zone(ref); ok {
		return z.Spec.Interfaces
	}
	if _, ok := t.Interface(ref); ok {
		return []string{ref}
	}
	return nil
}

// ExpandAddresses resolves an address reference to its Addresses. Groups
// are flat (DM-15): a member that is not an Address is skipped (the
// Validator reports it as wrong-kind).
func (t *Tree) ExpandAddresses(ref string) []Address {
	if a, ok := t.Address(ref); ok {
		return []Address{a}
	}
	var out []Address
	if g, ok := t.AddressGroup(ref); ok {
		for _, n := range g.Spec.Members {
			if a, ok := t.Address(n); ok {
				out = append(out, a)
			}
		}
	}
	return out
}

// ExpandServices resolves a service reference to its Services (flat, DM-18).
func (t *Tree) ExpandServices(ref string) []Service {
	if s, ok := t.Service(ref); ok {
		return []Service{s}
	}
	var out []Service
	if g, ok := t.ServiceGroup(ref); ok {
		for _, n := range g.Spec.Members {
			if s, ok := t.Service(n); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// AnyAddress reports whether a source or destination list contains a
// predefined Address (any = 0.0.0.0/0, any6 = ::/0): "any" is an ordinary
// reference to a shipped object, not a keyword (DM-21).
func (t *Tree) AnyAddress(refs []string) bool {
	for _, r := range refs {
		if a, ok := t.Address(r); ok && a.Spec.Predefined {
			return true
		}
	}
	return false
}

// AnyService reports whether a service list contains the predefined
// Service any (every protocol and port).
func (t *Tree) AnyService(refs []string) bool {
	for _, r := range refs {
		if s, ok := t.Service(r); ok && s.Spec.Predefined {
			return true
		}
	}
	return false
}

// Applicable returns the RuleSets that govern a Flow entering Interface in
// and leaving through Interface out, in evaluation order (DM-42): in → out,
// in → Zone(out), Zone(in) → out, Zone(in) → Zone(out), skipping those that
// do not exist. Traffic that leaves through the Interface it entered is
// never governed by a RuleSet.
func (t *Tree) Applicable(in, out string) []RuleSet {
	if in == out {
		return nil
	}
	zi, zo := t.ZoneOf(in), t.ZoneOf(out)
	var out2 []RuleSet
	seen := map[string]bool{}
	for _, p := range [][2]string{{in, out}, {in, zo}, {zi, out}, {zi, zo}} {
		if p[0] == "" || p[1] == "" || p[0] == p[1] {
			continue
		}
		for _, rs := range t.RuleSets {
			if rs.Spec.From == p[0] && rs.Spec.To == p[1] && !seen[rs.Metadata.Name] {
				seen[rs.Metadata.Name] = true
				out2 = append(out2, rs)
			}
		}
	}
	return out2
}

// Uplink reports whether an Interface carries a default route: an IPv4
// gateway, a DHCP client, or a StaticRoute 0.0.0.0/0 through it. The PoC
// uses it only to simulate Internet traffic (the schema has no Interface
// role, design decision 11).
func (t *Tree) Uplink(name string) bool {
	i, ok := t.Interface(name)
	if !ok {
		return false
	}
	if v4 := i.Spec.IPv4; v4 != nil && (v4.Gateway != "" || v4.Mode == "dhcp") {
		return true
	}
	for _, r := range t.StaticRoutes {
		if r.Spec.Interface == name && r.Spec.Destination == "0.0.0.0/0" {
			return true
		}
	}
	return false
}

// AssignUUIDs gives a uuid to every object and rule that has none, keeping
// the uuid of the previous version (matched by name) when there is one, so
// that a uuid never changes once assigned (schema design decision 1).
func AssignUUIDs(o Doc, prev Doc) Doc {
	switch x := o.(type) {
	case RuleSet:
		var old map[string]string
		if p, ok := prev.(RuleSet); ok {
			old = map[string]string{}
			for _, r := range p.Spec.Rules {
				old[r.Name] = r.UUID
			}
			x.Metadata.UUID = keepUUID(x.Metadata.UUID, p.Metadata.UUID)
		} else if x.Metadata.UUID == "" {
			x.Metadata.UUID = NewUUID()
		}
		rules := make([]Rule, len(x.Spec.Rules)) // never nil: rules is required, [] is valid
		copy(rules, x.Spec.Rules)
		for i := range rules {
			rules[i].UUID = keepUUID(rules[i].UUID, old[rules[i].Name])
		}
		x.Spec.Rules = rules
		return x
	}
	return setUUID(o, prev)
}

func keepUUID(cur, prev string) string {
	if prev != "" {
		return prev
	}
	if cur != "" {
		return cur
	}
	return NewUUID()
}

func setUUID(o, prev Doc) Doc {
	var pm Meta
	if prev != nil {
		_, pm = prev.Header()
	}
	switch x := o.(type) {
	case System:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case Color:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case Interface:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case Zone:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case StaticRoute:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case Address:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case Object[GroupSpec]: // AddressGroup and ServiceGroup share the type
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case Service:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	case User:
		x.Metadata.UUID = keepUUID(x.Metadata.UUID, pm.UUID)
		return x
	}
	return o
}
