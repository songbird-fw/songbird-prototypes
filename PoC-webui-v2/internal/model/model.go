// Package model holds the configuration objects of domain-model.md v0.4.
// Every object is a CRD-style document: apiVersion, kind, metadata, spec.
// One object is one YAML file in the configuration repository (ADR-0001).
package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

const APIVersion = "songbird.io/v1alpha1"

type Meta struct {
	Name   string `yaml:"name" json:"name"`
	Colour string `yaml:"colour,omitempty" json:"colour,omitempty"`
}

type Object[S any] struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   Meta   `yaml:"metadata" json:"metadata"`
	Spec       S      `yaml:"spec" json:"spec"`
}

// ---- Specs -----------------------------------------------------------------

type InterfaceSpec struct {
	Role        string      `yaml:"role" json:"role"`
	Type        string      `yaml:"type" json:"type"`
	Parent      string      `yaml:"parent,omitempty" json:"parent,omitempty"`
	VlanID      int         `yaml:"vlanId,omitempty" json:"vlanId,omitempty"`
	Addresses   []string    `yaml:"addresses" json:"addresses"`
	AllowAccess []string    `yaml:"allowAccess,omitempty" json:"allowAccess,omitempty"`
	Dhcp        *DhcpConfig `yaml:"dhcp,omitempty" json:"dhcp,omitempty"`
}

type DhcpConfig struct {
	Enabled      bool              `yaml:"enabled" json:"enabled"`
	RangeStart   string            `yaml:"rangeStart" json:"rangeStart"`
	RangeEnd     string            `yaml:"rangeEnd" json:"rangeEnd"`
	Gateway      string            `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	DnsServers   []string          `yaml:"dnsServers,omitempty" json:"dnsServers,omitempty"`
	LeaseTime    int               `yaml:"leaseTime,omitempty" json:"leaseTime,omitempty"`
	Reservations []DhcpReservation `yaml:"reservations,omitempty" json:"reservations,omitempty"`
}

type DhcpReservation struct {
	Mac string `yaml:"mac" json:"mac"`
	IP  string `yaml:"ip" json:"ip"`
}

type ZoneSpec struct {
	SrcInterface string   `yaml:"srcInterface" json:"srcInterface"`
	DstInterface string   `yaml:"dstInterface" json:"dstInterface"`
	Policies     []Policy `yaml:"policies,omitempty" json:"policies"`
}

// Policy lives inside its Zone file: position in the list is its priority.
type Policy struct {
	Name         string     `yaml:"name" json:"name"`
	Colour       string     `yaml:"colour,omitempty" json:"colour,omitempty"`
	Sources      []string   `yaml:"sources" json:"sources"`
	Destinations []string   `yaml:"destinations" json:"destinations"`
	Services     []string   `yaml:"services" json:"services"`
	Action       string     `yaml:"action" json:"action"`
	Nat          *NatConfig `yaml:"nat,omitempty" json:"nat,omitempty"`
	Enabled      *bool      `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

func (p Policy) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

type NatConfig struct {
	Mode string `yaml:"mode" json:"mode"`
	Pool string `yaml:"pool,omitempty" json:"pool,omitempty"`
}

type AddressSpec struct {
	Type      string `yaml:"type" json:"type"`
	Value     string `yaml:"value" json:"value"`
	Interface string `yaml:"interface,omitempty" json:"interface,omitempty"`
	Builtin   bool   `yaml:"builtin,omitempty" json:"builtin,omitempty"`
}

type AddressGroupSpec struct {
	Interface string   `yaml:"interface,omitempty" json:"interface,omitempty"`
	Members   []string `yaml:"members" json:"members"`
}

type VipExternal struct {
	Interface string `yaml:"interface" json:"interface"`
	IP        string `yaml:"ip" json:"ip"`
	Port      string `yaml:"port" json:"port"`
}

type VipMapped struct {
	IP   string `yaml:"ip" json:"ip"`
	Port string `yaml:"port" json:"port"`
}

type VirtualIPSpec struct {
	External VipExternal `yaml:"external" json:"external"`
	Mapped   VipMapped   `yaml:"mapped" json:"mapped"`
	Protocol string      `yaml:"protocol" json:"protocol"`
}

type ServiceSpec struct {
	Protocol string `yaml:"protocol" json:"protocol"`
	Ports    string `yaml:"ports,omitempty" json:"ports,omitempty"`
	Builtin  bool   `yaml:"builtin,omitempty" json:"builtin,omitempty"`
}

type ServiceGroupSpec struct {
	Members []string `yaml:"members" json:"members"`
}

type IpPoolSpec struct {
	StartIP string `yaml:"startIp" json:"startIp"`
	EndIP   string `yaml:"endIp" json:"endIp"`
}

type StaticRouteSpec struct {
	Destination  string `yaml:"destination" json:"destination"`
	Gateway      string `yaml:"gateway" json:"gateway"`
	OutInterface string `yaml:"outInterface" json:"outInterface"`
	Distance     int    `yaml:"distance,omitempty" json:"distance,omitempty"`
	Priority     int    `yaml:"priority,omitempty" json:"priority,omitempty"`
}

type ColourSpec struct {
	Value   string `yaml:"value" json:"value"`
	Meaning string `yaml:"meaning,omitempty" json:"meaning,omitempty"`
	Builtin bool   `yaml:"builtin,omitempty" json:"builtin,omitempty"`
}

// UserSpec: Users and Roles are configuration (components.md, Auth).
type UserSpec struct {
	Role         string `yaml:"role" json:"role"`
	PasswordHash string `yaml:"passwordHash" json:"passwordHash,omitempty"`
}

type (
	Interface    = Object[InterfaceSpec]
	Zone         = Object[ZoneSpec]
	Address      = Object[AddressSpec]
	AddressGroup = Object[AddressGroupSpec]
	VirtualIP    = Object[VirtualIPSpec]
	Service      = Object[ServiceSpec]
	ServiceGroup = Object[ServiceGroupSpec]
	IpPool       = Object[IpPoolSpec]
	StaticRoute  = Object[StaticRouteSpec]
	Colour       = Object[ColourSpec]
	User         = Object[UserSpec]
)

// ---- Tree ------------------------------------------------------------------

// Tree is a complete configuration: the desired state at one revision.
type Tree struct {
	Interfaces    List[InterfaceSpec]    `json:"Interface"`
	Zones         List[ZoneSpec]         `json:"Zone"`
	Addresses     List[AddressSpec]      `json:"Address"`
	AddressGroups List[AddressGroupSpec] `json:"AddressGroup"`
	VirtualIPs    List[VirtualIPSpec]    `json:"VirtualIP"`
	Services      List[ServiceSpec]      `json:"Service"`
	ServiceGroups List[ServiceGroupSpec] `json:"ServiceGroup"`
	IpPools       List[IpPoolSpec]       `json:"IpPool"`
	StaticRoutes  List[StaticRouteSpec]  `json:"StaticRoute"`
	Colours       List[ColourSpec]       `json:"Colour"`
	Users         List[UserSpec]         `json:"User"`
}

// Kind describes one object kind and the repository directory holding it.
type Kind struct {
	Name string
	Dir  string
}

var Kinds = []Kind{
	{"Interface", "interfaces"},
	{"Zone", "zones"},
	{"Address", "addresses"},
	{"AddressGroup", "addressgroups"},
	{"VirtualIP", "virtualips"},
	{"Service", "services"},
	{"ServiceGroup", "servicegroups"},
	{"IpPool", "ippools"},
	{"StaticRoute", "staticroutes"},
	{"Colour", "colours"},
	{"User", "users"},
}

func KindByName(name string) (Kind, bool) {
	for _, k := range Kinds {
		if k.Name == name {
			return k, true
		}
	}
	return Kind{}, false
}

func KindByDir(dir string) (Kind, bool) {
	for _, k := range Kinds {
		if k.Dir == dir {
			return k, true
		}
	}
	return Kind{}, false
}

// FilePath is the repository path of an object.
func FilePath(kind, name string) string {
	k, _ := KindByName(kind)
	return k.Dir + "/" + name + ".yaml"
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

func ValidName(name string) bool { return nameRe.MatchString(name) }

// anyList lets the Tree be handled generically by kind.
type anyList interface {
	putJSON(kind string, data []byte) (string, error)
	putYAML(kind string, data []byte) (string, error)
	remove(name string) bool
	find(name string) (any, bool)
	sortByName()
	names() []string
}

type List[S any] []Object[S]

func (l *List[S]) put(o Object[S]) {
	for i := range *l {
		if (*l)[i].Metadata.Name == o.Metadata.Name {
			(*l)[i] = o
			return
		}
	}
	*l = append(*l, o)
}

func (l *List[S]) putJSON(kind string, data []byte) (string, error) {
	var o Object[S]
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&o); err != nil {
		return "", fmt.Errorf("invalid %s: %w", kind, err)
	}
	o.APIVersion, o.Kind = APIVersion, kind
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
		return "", fmt.Errorf("file declares kind %q, expected %q", o.Kind, kind)
	}
	l.put(o)
	return o.Metadata.Name, nil
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

// Names lists the object names of one kind.
func (t *Tree) Names(kind string) []string {
	if l := t.list(kind); l != nil {
		return l.names()
	}
	return nil
}

func (t *Tree) list(kind string) anyList {
	switch kind {
	case "Interface":
		return &t.Interfaces
	case "Zone":
		return &t.Zones
	case "Address":
		return &t.Addresses
	case "AddressGroup":
		return &t.AddressGroups
	case "VirtualIP":
		return &t.VirtualIPs
	case "Service":
		return &t.Services
	case "ServiceGroup":
		return &t.ServiceGroups
	case "IpPool":
		return &t.IpPools
	case "StaticRoute":
		return &t.StaticRoutes
	case "Colour":
		return &t.Colours
	case "User":
		return &t.Users
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

func (t *Tree) Delete(kind, name string) bool {
	if l := t.list(kind); l != nil {
		return l.remove(name)
	}
	return false
}

func (t *Tree) Get(kind, name string) (any, bool) {
	if l := t.list(kind); l != nil {
		return l.find(name)
	}
	return nil, false
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

// MarshalObject renders one object as the YAML stored in the repository.
func MarshalObject(o any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(o); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
