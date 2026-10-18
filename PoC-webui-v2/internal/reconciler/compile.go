package reconciler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

// compile is a simulated Compiler: pure and deterministic. It splits the
// tree into one section per driver and hashes each, so the Reconciler can
// skip drivers a commit does not affect (reconciliation.md §2). Metadata
// (comment, color, category, labels, uuid) never reaches a driver: a
// metadata-only commit is a no-op. It also expands Zones into Interface
// pairs and groups into match entries, as the Data Plane compiler would
// (dataplane.md §7, ADR-0006).
func compile(t *m.Tree) PlanSummary {
	sys := t.Sys()
	p := PlanSummary{RuleSets: len(t.RuleSets), LocalIn: map[string][]string{}, IntraZone: map[string]string{}, Sections: map[string]string{}}

	// ---- network: links, VLANs, addresses, routes
	type link struct {
		Name    string
		Spec    m.InterfaceSpec
		Enabled bool
	}
	var links []link
	for _, i := range t.Interfaces {
		s := i.Spec
		s.IPv6 = nil // deferred: never applied (R-057)
		links = append(links, link{i.Metadata.Name, s, s.IsEnabled()})
	}
	var routes []m.StaticRouteSpec
	for _, r := range t.StaticRoutes {
		routes = append(routes, r.Spec)
	}
	p.Sections["network"] = hash([]any{links, routes})

	// ---- local-in (DM-05, DM-34): allowAccess plus the ports of the
	// services the firewall runs on an Interface.
	served := func(refs []string) map[string]bool {
		out := map[string]bool{}
		for _, r := range refs {
			for _, i := range t.Side(r) {
				out[i] = true
			}
		}
		return out
	}
	var dnsOn, ntpOn map[string]bool
	if sys.DNS != nil {
		dnsOn = served(sys.DNS.ServeToInterfaces)
	}
	if sys.NTP != nil && sys.NTP.Enabled {
		ntpOn = served(sys.NTP.ServeToInterfaces)
	}
	dhcpOn := map[string]bool{}
	var dhcp []m.DhcpServerSpec
	for _, d := range t.DhcpServers {
		dhcp = append(dhcp, d.Spec)
		if d.Spec.Enabled {
			dhcpOn[d.Spec.Interface] = true
		}
	}
	for _, i := range t.Interfaces {
		n := i.Metadata.Name
		acc := []string{}
		for _, a := range i.Spec.AllowAccess {
			if a == "https" {
				a = fmt.Sprintf("https(tcp/%d)", sys.HTTPSPort())
			}
			acc = append(acc, a)
		}
		has := func(s string) bool { return slices.Contains(i.Spec.AllowAccess, s) }
		if dhcpOn[n] && !has("dhcp") {
			acc = append(acc, "dhcp(udp/67, implicit DM-34)")
		}
		if dnsOn[n] && !has("dns") {
			acc = append(acc, "dns(53, implicit DM-34)")
		}
		if ntpOn[n] && !has("ntp") {
			acc = append(acc, "ntp(udp/123, implicit DM-34)")
		}
		p.LocalIn[n] = acc
	}
	trusted := []string{}
	if sys.Admin != nil {
		for _, h := range sys.Admin.TrustedHosts {
			for _, a := range t.ExpandAddresses(h) {
				trusted = append(trusted, a.Spec.Value())
			}
		}
	}

	// ---- dataplane: RuleSets expanded to Interface pairs, objects resolved
	type entry struct {
		Rule        string
		Action      string
		Log         string
		Nat         *m.Nat
		Source      []string
		Destination []string
		Service     []string
	}
	type ruleset struct {
		Name  string
		Pairs [][2]string
		Rules []entry
	}
	resolveA := func(refs []string) []string {
		if m.IsAny(refs) {
			return []string{"any"}
		}
		var out []string
		for _, r := range refs {
			for _, a := range t.ExpandAddresses(r) {
				out = append(out, a.Spec.Value())
			}
		}
		sort.Strings(out)
		return slices.Compact(out)
	}
	resolveS := func(refs []string) []string {
		if m.IsAny(refs) {
			return []string{"any"}
		}
		var out []string
		for _, r := range refs {
			for _, s := range t.ExpandServices(r) {
				b, _ := json.Marshal(s.Spec.Protocols)
				out = append(out, string(b))
			}
		}
		sort.Strings(out)
		return slices.Compact(out)
	}
	var dp []ruleset
	for _, rs := range t.RuleSets {
		c := ruleset{Name: rs.Metadata.Name}
		for _, f := range t.Side(rs.Spec.From) {
			for _, to := range t.Side(rs.Spec.To) {
				if f != to {
					c.Pairs = append(c.Pairs, [2]string{f, to})
				}
			}
		}
		p.Pairs += len(c.Pairs)
		for _, r := range rs.Spec.Rules {
			if !r.IsEnabled() {
				p.Disabled++
				continue
			}
			p.Policies++
			e := entry{r.Name, r.Action, r.Log, r.Nat, resolveA(r.Source), resolveA(r.Destination), resolveS(r.Service)}
			p.MatchEntries += len(e.Source) * len(e.Destination) * len(e.Service) * max(1, len(c.Pairs))
			if r.Nat != nil && r.Nat.Type == "dnat" {
				p.DNATEntries += len(e.Destination) * len(e.Service)
				if r.Nat.TranslateIPTo != "" {
					e.Nat = &m.Nat{Type: "dnat", TranslateIPTo: fmt.Sprint(resolveA([]string{r.Nat.TranslateIPTo})), TranslateServiceTo: fmt.Sprint(resolveS([]string{r.Nat.TranslateServiceTo}))}
				}
			}
			c.Rules = append(c.Rules, e)
		}
		dp = append(dp, c)
	}
	for _, z := range t.Zones {
		v := z.Spec.IntraZoneTraffic
		if v == "" {
			v = "deny"
		}
		p.IntraZone[z.Metadata.Name] = v // DM-39
	}
	feeds := []string{}
	if sys.ThreatFeedsOn() {
		for _, f := range sys.ThreatFeeds.Sources {
			if f.Enabled == nil || *f.Enabled {
				feeds = append(feeds, f.URL)
			}
		}
	}
	p.Sections["dataplane"] = hash([]any{dp, p.LocalIn, p.IntraZone, trusted, sys.HTTPSPort(), sys.Session, sys.Logging, feeds, interfaceAddresses(t)})
	p.Sections["dhcp"] = hash(dhcp)
	p.Sections["dns"] = hash([]any{sys.DNS, sys.NTP, dnsOn, ntpOn, interfaceAddresses(t)})
	return p
}

func interfaceAddresses(t *m.Tree) map[string][]string {
	out := map[string][]string{}
	for _, i := range t.Interfaces {
		out[i.Metadata.Name] = i.Spec.StaticAddresses()
	}
	return out
}

func hash(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])[:12]
}
