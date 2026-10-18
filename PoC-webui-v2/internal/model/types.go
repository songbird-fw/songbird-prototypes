// Package model holds the configuration objects of songbird-yaml-schema
// v1alpha1 (domain-model.md v0.6). Every object is a document with
// apiVersion, kind, metadata and spec; the Go types mirror the JSON Schema
// one field per property, so a file read and written back is unchanged.
//
// Three kinds are not in v1alpha1 yet ("Not covered yet" in the schema
// README): StaticRoute, DhcpServer and User. The PoC defines them as
// proposals with their own schema (internal/schema/ext).
package model

const APIVersion = "config.songbird-fw.com/v1alpha1"

// Any is the wildcard of Policy source, destination and service: ["any"].
const Any = "any"

// Meta is common.schema.json#/$defs/metadata.
type Meta struct {
	Name     string            `yaml:"name" json:"name"`
	UUID     string            `yaml:"uuid,omitempty" json:"uuid,omitempty"`
	Comment  string            `yaml:"comment,omitempty" json:"comment,omitempty"`
	Color    string            `yaml:"color,omitempty" json:"color,omitempty"`
	Category string            `yaml:"category,omitempty" json:"category,omitempty"`
	Labels   map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type Object[S any] struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   Meta   `yaml:"metadata" json:"metadata"`
	Spec       S      `yaml:"spec" json:"spec"`
}

// ---- Color ---------------------------------------------------------------------

type ColorSpec struct {
	Hex        string `yaml:"hex" json:"hex"`
	Predefined bool   `yaml:"predefined,omitempty" json:"predefined,omitempty"`
}

// ---- Interface and Zone ------------------------------------------------------------

type InterfaceSpec struct {
	Type        string      `yaml:"type" json:"type"` // physical, vlan
	Device      string      `yaml:"device,omitempty" json:"device,omitempty"`
	Parent      string      `yaml:"parent,omitempty" json:"parent,omitempty"`
	VlanID      int         `yaml:"vlanId,omitempty" json:"vlanId,omitempty"`
	Enabled     *bool       `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MTU         int         `yaml:"mtu,omitempty" json:"mtu,omitempty"`
	Role        string      `yaml:"role,omitempty" json:"role,omitempty"` // internal (default), external, dmz, transit
	AllowAccess []string    `yaml:"allowAccess,omitempty" json:"allowAccess,omitempty"`
	IPv4        *IPv4Config `yaml:"ipv4,omitempty" json:"ipv4,omitempty"`
	IPv6        *IPv6Config `yaml:"ipv6,omitempty" json:"ipv6,omitempty"`
}

func (s InterfaceSpec) IsEnabled() bool { return s.Enabled == nil || *s.Enabled }

// EffectiveRole applies the schema default.
func (s InterfaceSpec) EffectiveRole() string {
	if s.Role == "" {
		return "internal"
	}
	return s.Role
}

// StaticAddresses are the IPv4 prefixes of a static Interface.
func (s InterfaceSpec) StaticAddresses() []string {
	if s.IPv4 == nil || s.IPv4.Mode != "static" {
		return nil
	}
	return s.IPv4.Addresses
}

type IPv4Config struct {
	Mode      string   `yaml:"mode" json:"mode"` // static, dhcp, none
	Addresses []string `yaml:"addresses,omitempty" json:"addresses,omitempty"`
	Gateway   string   `yaml:"gateway,omitempty" json:"gateway,omitempty"`
}

type IPv6Config struct {
	Mode                string               `yaml:"mode" json:"mode"` // static, slaac, dhcpv6-pd, none
	Addresses           []string             `yaml:"addresses,omitempty" json:"addresses,omitempty"`
	Gateway             string               `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	RouterAdvertisement *RouterAdvertisement `yaml:"routerAdvertisement,omitempty" json:"routerAdvertisement,omitempty"`
}

type RouterAdvertisement struct {
	Enabled             bool   `yaml:"enabled" json:"enabled"`
	ManagedFlag         *bool  `yaml:"managedFlag,omitempty" json:"managedFlag,omitempty"`
	OtherConfigFlag     *bool  `yaml:"otherConfigFlag,omitempty" json:"otherConfigFlag,omitempty"`
	DelegatedPrefixFrom string `yaml:"delegatedPrefixFrom,omitempty" json:"delegatedPrefixFrom,omitempty"`
}

type ZoneSpec struct {
	Interfaces       []string `yaml:"interfaces" json:"interfaces"`
	IntraZoneTraffic string   `yaml:"intraZoneTraffic,omitempty" json:"intraZoneTraffic,omitempty"` // allow, deny (default)
}

// ---- Addresses and Services ------------------------------------------------------------

// AddressSpec has exactly one of Subnet, Range, FQDN, MAC (schema oneOf).
type AddressSpec struct {
	Interface string        `yaml:"interface,omitempty" json:"interface,omitempty"`
	Subnet    string        `yaml:"subnet,omitempty" json:"subnet,omitempty"`
	Range     *AddressRange `yaml:"range,omitempty" json:"range,omitempty"`
	FQDN      string        `yaml:"fqdn,omitempty" json:"fqdn,omitempty"`
	MAC       string        `yaml:"mac,omitempty" json:"mac,omitempty"`
}

type AddressRange struct {
	Start string `yaml:"start" json:"start"`
	End   string `yaml:"end" json:"end"`
}

// Form returns which of the four forms the Address uses.
func (s AddressSpec) Form() string {
	switch {
	case s.Subnet != "":
		return "subnet"
	case s.Range != nil:
		return "range"
	case s.FQDN != "":
		return "fqdn"
	case s.MAC != "":
		return "mac"
	}
	return ""
}

// Value renders the Address for logs and the UI.
func (s AddressSpec) Value() string {
	switch s.Form() {
	case "subnet":
		return s.Subnet
	case "range":
		return s.Range.Start + "-" + s.Range.End
	case "fqdn":
		return s.FQDN
	case "mac":
		return s.MAC
	}
	return ""
}

type GroupSpec struct {
	Members []string `yaml:"members" json:"members"`
}

type ServiceSpec struct {
	Protocols []ProtocolEntry `yaml:"protocols" json:"protocols"`
}

type ProtocolEntry struct {
	Protocol       string     `yaml:"protocol" json:"protocol"` // tcp, udp, sctp, icmp, icmpv6, ip
	Port           *PortRange `yaml:"port,omitempty" json:"port,omitempty"`
	SourcePort     *PortRange `yaml:"sourcePort,omitempty" json:"sourcePort,omitempty"`
	ICMPType       *int       `yaml:"icmpType,omitempty" json:"icmpType,omitempty"`
	ICMPCode       *int       `yaml:"icmpCode,omitempty" json:"icmpCode,omitempty"`
	ProtocolNumber *int       `yaml:"protocolNumber,omitempty" json:"protocolNumber,omitempty"`
}

// ---- RuleSet --------------------------------------------------------------------------------

type RuleSetSpec struct {
	From  string `yaml:"from" json:"from"`
	To    string `yaml:"to" json:"to"`
	Rules []Rule `yaml:"rules" json:"rules"`
}

// Rule is a Policy (domain model): position in the RuleSet is its priority.
type Rule struct {
	Name        string   `yaml:"name" json:"name"`
	UUID        string   `yaml:"uuid,omitempty" json:"uuid,omitempty"`
	Comment     string   `yaml:"comment,omitempty" json:"comment,omitempty"`
	Enabled     *bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Source      []string `yaml:"source" json:"source"`
	Destination []string `yaml:"destination" json:"destination"`
	Service     []string `yaml:"service" json:"service"`
	Action      string   `yaml:"action" json:"action"` // accept, drop
	Log         string   `yaml:"log,omitempty" json:"log,omitempty"`
	Nat         *Nat     `yaml:"nat,omitempty" json:"nat,omitempty"`
	Schedule    string   `yaml:"schedule,omitempty" json:"schedule,omitempty"`
}

func (r Rule) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }

type Nat struct {
	Type               string `yaml:"type" json:"type"` // snat, dnat
	TranslateIPTo      string `yaml:"translateIpTo,omitempty" json:"translateIpTo,omitempty"`
	TranslateServiceTo string `yaml:"translateServiceTo,omitempty" json:"translateServiceTo,omitempty"`
}

// IsAny reports whether a source/destination/service list is ["any"].
func IsAny(l []string) bool { return len(l) == 1 && l[0] == Any }

// ---- PoC extensions (not in v1alpha1) ----------------------------------------------------------

// StaticRouteSpec follows domain-model.md (StaticRoute, DM-22, DM-23).
type StaticRouteSpec struct {
	Destination string `yaml:"destination" json:"destination"`
	Gateway     string `yaml:"gateway" json:"gateway"`
	Interface   string `yaml:"interface" json:"interface"`
	Distance    int    `yaml:"distance,omitempty" json:"distance,omitempty"`
	Priority    int    `yaml:"priority,omitempty" json:"priority,omitempty"`
}

const (
	DefaultDistance = 10 // domain model: proposed
	DefaultPriority = 1
)

func (s StaticRouteSpec) Preference() (int, int) {
	d, p := s.Distance, s.Priority
	if d == 0 {
		d = DefaultDistance
	}
	if p == 0 {
		p = DefaultPriority
	}
	return d, p
}

// DhcpServerSpec is the domain model's DhcpConfig (DM-24 … DM-29) as a kind
// of its own, as the schema README suggests ("DhcpServer"), so that the
// Interface stays exactly as in v1alpha1.
type DhcpServerSpec struct {
	Interface    string            `yaml:"interface" json:"interface"`
	Enabled      bool              `yaml:"enabled" json:"enabled"`
	RangeStart   string            `yaml:"rangeStart" json:"rangeStart"`
	RangeEnd     string            `yaml:"rangeEnd" json:"rangeEnd"`
	Gateway      string            `yaml:"gateway,omitempty" json:"gateway,omitempty"`
	DNSServers   []string          `yaml:"dnsServers,omitempty" json:"dnsServers,omitempty"`
	LeaseTime    string            `yaml:"leaseTime,omitempty" json:"leaseTime,omitempty"` // duration, default 24h
	Reservations []DhcpReservation `yaml:"reservations,omitempty" json:"reservations,omitempty"`
}

type DhcpReservation struct {
	MAC  string `yaml:"mac" json:"mac"`
	IP   string `yaml:"ip" json:"ip"`
	Host string `yaml:"host,omitempty" json:"host,omitempty"`
}

// UserSpec: Users and Roles are configuration (components.md, Auth).
type UserSpec struct {
	Role         string `yaml:"role" json:"role"` // admin, read-only
	PasswordHash string `yaml:"passwordHash" json:"passwordHash,omitempty"`
}

type (
	System       = Object[SystemSpec]
	Color        = Object[ColorSpec]
	Interface    = Object[InterfaceSpec]
	Zone         = Object[ZoneSpec]
	Address      = Object[AddressSpec]
	AddressGroup = Object[GroupSpec]
	Service      = Object[ServiceSpec]
	ServiceGroup = Object[GroupSpec]
	RuleSet      = Object[RuleSetSpec]
	StaticRoute  = Object[StaticRouteSpec]
	DhcpServer   = Object[DhcpServerSpec]
	User         = Object[UserSpec]
)
