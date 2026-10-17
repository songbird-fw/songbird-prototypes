// Package defaults builds the factory-default configuration committed on
// first boot (components.md: "No bootstrap mode"). The PoC ships a small but
// realistic network so every page has something to show.
package defaults

import (
	"golang.org/x/crypto/bcrypt"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

func obj[S any](kind, name, colour string, spec S) m.Object[S] {
	return m.Object[S]{APIVersion: m.APIVersion, Kind: kind, Metadata: m.Meta{Name: name, Colour: colour}, Spec: spec}
}

func hash(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 12) // R-043.1: cost >= 12
	if err != nil {
		panic(err)
	}
	return string(h)
}

func off() *bool { f := false; return &f }

func Tree() *m.Tree {
	t := &m.Tree{}

	// R-055.1: predefined palette, value immutable, meaning editable.
	for _, c := range []struct{ name, value, meaning string }{
		{"blue", "#2A78D6", "Main LAN"},
		{"green", "#0F9D58", "Backup"},
		{"red", "#D03B3B", "DMZ / exposed"},
		{"orange", "#EB6834", "Guests"},
		{"violet", "#6250D6", "Management"},
		{"grey", "#7A7A75", ""},
	} {
		t.Colours = append(t.Colours, obj("Colour", c.name, "", m.ColourSpec{Value: c.value, Meaning: c.meaning, Builtin: true}))
	}

	t.Interfaces = m.List[m.InterfaceSpec]{
		obj("Interface", "WAN1", "", m.InterfaceSpec{Role: "wan", Type: "physical", Addresses: []string{"203.0.113.10/24"}, AllowAccess: []string{"ping"}}),
		obj("Interface", "LAN", "blue", m.InterfaceSpec{
			Role: "lan", Type: "physical", Addresses: []string{"192.168.1.1/24"}, AllowAccess: []string{"ping", "https"},
			Dhcp: &m.DhcpConfig{Enabled: true, RangeStart: "192.168.1.100", RangeEnd: "192.168.1.199", DnsServers: []string{"192.168.1.1"}, LeaseTime: 86400,
				Reservations: []m.DhcpReservation{{Mac: "52:54:00:12:34:56", IP: "192.168.1.20"}}},
		}),
		obj("Interface", "GUEST", "orange", m.InterfaceSpec{
			Role: "lan", Type: "vlan", Parent: "LAN", VlanID: 10, Addresses: []string{"192.168.10.1/24"},
			Dhcp: &m.DhcpConfig{Enabled: true, RangeStart: "192.168.10.50", RangeEnd: "192.168.10.250", LeaseTime: 7200},
		}),
		obj("Interface", "DMZ", "red", m.InterfaceSpec{Role: "dmz", Type: "vlan", Parent: "LAN", VlanID: 20, Addresses: []string{"172.16.20.1/24"}}),
	}

	t.Addresses = m.List[m.AddressSpec]{
		obj("Address", "all", "", m.AddressSpec{Type: "subnet", Value: "0.0.0.0/0", Builtin: true}),
		obj("Address", "nas", "green", m.AddressSpec{Type: "ip", Value: "192.168.1.20", Interface: "LAN"}),
		obj("Address", "admin-pcs", "violet", m.AddressSpec{Type: "range", Value: "192.168.1.10-192.168.1.19", Interface: "LAN"}),
		obj("Address", "web-01", "red", m.AddressSpec{Type: "ip", Value: "172.16.20.10", Interface: "DMZ"}),
		obj("Address", "web-02", "red", m.AddressSpec{Type: "ip", Value: "172.16.20.11", Interface: "DMZ"}),
		obj("Address", "offsite-backup", "green", m.AddressSpec{Type: "subnet", Value: "198.51.100.0/28", Interface: "WAN1"}),
	}
	t.AddressGroups = m.List[m.AddressGroupSpec]{
		obj("AddressGroup", "web-servers", "red", m.AddressGroupSpec{Interface: "DMZ", Members: []string{"web-01", "web-02"}}),
	}
	t.VirtualIPs = m.List[m.VirtualIPSpec]{
		obj("VirtualIP", "vip-web-https", "red", m.VirtualIPSpec{
			External: m.VipExternal{Interface: "WAN1", IP: "203.0.113.10", Port: "443"},
			Mapped:   m.VipMapped{IP: "172.16.20.10", Port: "8443"}, Protocol: "tcp",
		}),
	}

	t.Services = m.List[m.ServiceSpec]{
		obj("Service", "ALL", "", m.ServiceSpec{Protocol: "any", Builtin: true}),
		obj("Service", "PING", "", m.ServiceSpec{Protocol: "icmp", Builtin: true}),
		obj("Service", "HTTP", "", m.ServiceSpec{Protocol: "tcp", Ports: "80", Builtin: true}),
		obj("Service", "HTTPS", "", m.ServiceSpec{Protocol: "tcp", Ports: "443", Builtin: true}),
		obj("Service", "SSH", "", m.ServiceSpec{Protocol: "tcp", Ports: "22", Builtin: true}),
		obj("Service", "DNS", "", m.ServiceSpec{Protocol: "udp", Ports: "53", Builtin: true}),
		obj("Service", "NTP", "", m.ServiceSpec{Protocol: "udp", Ports: "123", Builtin: true}),
		obj("Service", "rsync", "green", m.ServiceSpec{Protocol: "tcp", Ports: "873"}),
	}
	t.ServiceGroups = m.List[m.ServiceGroupSpec]{
		obj("ServiceGroup", "web", "", m.ServiceGroupSpec{Members: []string{"HTTP", "HTTPS"}}),
	}
	t.IpPools = m.List[m.IpPoolSpec]{
		obj("IpPool", "wan-pool", "", m.IpPoolSpec{StartIP: "203.0.113.20", EndIP: "203.0.113.29"}),
	}
	t.StaticRoutes = m.List[m.StaticRouteSpec]{
		obj("StaticRoute", "default", "", m.StaticRouteSpec{Destination: "0.0.0.0/0", Gateway: "203.0.113.1", OutInterface: "WAN1", Distance: 10, Priority: 1}),
		obj("StaticRoute", "default-backup", "green", m.StaticRouteSpec{Destination: "0.0.0.0/0", Gateway: "203.0.113.2", OutInterface: "WAN1", Distance: 20, Priority: 1}),
	}

	masq := &m.NatConfig{Mode: "interface"}
	t.Zones = m.List[m.ZoneSpec]{
		obj("Zone", "LAN-WAN1", "blue", m.ZoneSpec{SrcInterface: "LAN", DstInterface: "WAN1", Policies: []m.Policy{
			{Name: "backup-offsite", Colour: "green", Sources: []string{"nas"}, Destinations: []string{"offsite-backup"}, Services: []string{"rsync", "SSH"}, Action: "accept", Nat: masq},
			{Name: "web", Sources: []string{"LAN addresses"}, Destinations: []string{"all"}, Services: []string{"web"}, Action: "accept", Nat: masq},
			{Name: "dns-ntp", Sources: []string{"LAN addresses"}, Destinations: []string{"all"}, Services: []string{"DNS", "NTP"}, Action: "accept", Nat: masq},
			{Name: "ping", Sources: []string{"admin-pcs"}, Destinations: []string{"all"}, Services: []string{"PING"}, Action: "accept", Nat: masq},
		}}),
		obj("Zone", "GUEST-WAN1", "orange", m.ZoneSpec{SrcInterface: "GUEST", DstInterface: "WAN1", Policies: []m.Policy{
			{Name: "guest-web", Sources: []string{"GUEST addresses"}, Destinations: []string{"all"}, Services: []string{"web", "DNS"}, Action: "accept", Nat: &m.NatConfig{Mode: "pool", Pool: "wan-pool"}},
		}}),
		obj("Zone", "WAN1-DMZ", "red", m.ZoneSpec{SrcInterface: "WAN1", DstInterface: "DMZ", Policies: []m.Policy{
			{Name: "publish-web", Sources: []string{"all"}, Destinations: []string{"vip-web-https"}, Services: []string{"HTTPS"}, Action: "accept"},
		}}),
		obj("Zone", "LAN-DMZ", "", m.ZoneSpec{SrcInterface: "LAN", DstInterface: "DMZ", Policies: []m.Policy{
			{Name: "admin-ssh", Colour: "violet", Sources: []string{"admin-pcs"}, Destinations: []string{"web-servers"}, Services: []string{"SSH"}, Action: "accept"},
			{Name: "lan-to-web", Sources: []string{"LAN addresses"}, Destinations: []string{"web-servers"}, Services: []string{"web"}, Action: "accept"},
			{Name: "legacy-telnet", Sources: []string{"LAN addresses"}, Destinations: []string{"web-servers"}, Services: []string{"ALL"}, Action: "drop", Enabled: off()},
		}}),
	}

	t.Users = m.List[m.UserSpec]{
		obj("User", "admin", "", m.UserSpec{Role: "admin", PasswordHash: hash("songbird")}),
		obj("User", "auditor", "", m.UserSpec{Role: "read-only", PasswordHash: hash("auditor")}),
	}
	return t
}
