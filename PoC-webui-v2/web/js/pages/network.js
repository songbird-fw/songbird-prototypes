// Network: Interfaces as a tree, VLANs under their physical parent (R-015),
// with management access (allowAccess, R-056); Zones as sets of Interfaces
// (R-046, DM-37, DM-39); DHCP servers (DM-24 … DM-29) and static routes with
// the preference rule of DM-23. DhcpServer and StaticRoute are PoC
// extension kinds: the v1alpha1 schema does not cover them yet.
import { state, h, icon, mount, objs, find, sys, isAdmin, dot, refChip, objectForm, newObject, deleteObject, showYAML, findingBadge, metaFields, usedBy } from '../core.js';

const lines = (v) => (v || []).join('\n');
const splitLines = (s) => s.split(/[\n,]+/).map((x) => x.trim()).filter(Boolean);
const zoneOf = (iface) => objs('Zone').find((z) => z.spec.interfaces.includes(iface));
const ext = (kind) => h('span', { class: 'badge outline', title: 'PoC extension kind: not in songbird-yaml-schema v1alpha1 (internal/schema/ext)' }, 'PoC kind');

const ACCESS = [
  { value: 'https', label: 'https', title: 'Web UI / API on System.admin.httpsPort, restricted by trustedHosts' },
  { value: 'ping', label: 'ping', title: 'ICMP echo' },
  { value: 'dns', label: 'dns' }, { value: 'dhcp', label: 'dhcp' }, { value: 'ntp', label: 'ntp' },
];

// ---- forms ----------------------------------------------------------------------------------

function ifaceForm(iface) {
  const isNew = !iface;
  const obj = iface || newObject('Interface', { type: 'vlan', role: 'internal', ipv4: { mode: 'static', addresses: [] } });
  objectForm({
    kind: 'Interface', isNew, obj,
    fields: [
      ...metaFields({ placeholder: 'lan3' }),
      { key: 'spec.type', label: 'Type', type: 'select', required: true, options: ['physical', 'vlan'], rerender: true, immutable: true },
      { key: 'spec.role', label: 'Role', type: 'select', options: [{ value: 'internal', label: 'internal (default)' }, 'external', 'dmz', 'transit'],
        help: 'Informational; internal Interfaces reach the UI during Initial Setup (R-031.1); no DHCP server on external (DM-24)' },
      { key: 'spec.device', label: 'Linux device', required: true, show: (o) => o.spec.type === 'physical', placeholder: 'enp3s0', help: 'The netdev name; the object name is what users see' },
      { key: 'spec.parent', label: 'Parent (physical)', type: 'select', required: true, show: (o) => o.spec.type === 'vlan',
        options: () => [{ value: '', label: '— select —' }, ...objs('Interface').filter((i) => i.spec.type === 'physical').map((i) => `${i.metadata.name}`)] },
      { key: 'spec.vlanId', label: 'VLAN ID', type: 'number', required: true, show: (o) => o.spec.type === 'vlan', placeholder: '1–4094' },
      { key: 'spec.ipv4.mode', label: 'IPv4', type: 'select', required: true, options: [{ value: 'static', label: 'static' }, { value: 'dhcp', label: 'dhcp client' }, { value: 'none', label: 'none (e.g. a trunk)' }], rerender: true },
      { key: 'spec.mtu', label: 'MTU', type: 'number', placeholder: '1500', help: '1280–9216' },
      { key: 'spec.ipv4.addresses', label: 'IPv4 addresses', type: 'textarea', required: true, full: true, show: (o) => o.spec.ipv4?.mode === 'static',
        placeholder: '192.168.40.1/24', format: lines, parse: splitLines, help: 'One address/prefix per line, CIDR only. Subnets must not overlap other Interfaces (DM-04).' },
      { key: 'spec.ipv4.gateway', label: 'Default gateway via this Interface', show: (o) => o.spec.ipv4?.mode === 'static', placeholder: 'only on an uplink' },
      { key: 'spec.enabled', label: 'State', type: 'checkbox', invert: true, checkLabel: 'Enabled' },
      { key: 'spec.allowAccess', label: 'Management access to the firewall', type: 'checks', full: true, options: ACCESS,
        help: 'Reachable through the addresses of this Interface (owner rule). Default: none (R-056). Services the firewall runs here also open their own port (DM-34).' },
    ],
    extra: (o) => o.spec.ipv6 && h('div', { class: 'vwarn' }, 'This Interface has an IPv6 section: it is kept as is, and not applied (IPv6 is deferred, R-057). Edit it in YAML.'),
    // the fields of the other type / mode are dropped, as the schema requires
    toObject: (o) => {
      if (o.spec.type === 'vlan') delete o.spec.device; else { delete o.spec.parent; delete o.spec.vlanId; }
      if (o.spec.ipv4 && o.spec.ipv4.mode !== 'static') { delete o.spec.ipv4.addresses; delete o.spec.ipv4.gateway; }
      return o;
    },
  });
}

function zoneForm(zone) {
  const isNew = !zone;
  objectForm({
    kind: 'Zone', isNew, obj: zone || newObject('Zone', { interfaces: [] }),
    intro: h('p', { class: 'sub small', style: { margin: 0 } }, 'A Zone groups Interfaces treated alike. RuleSets then reference the Zone, not its members (DM-38).'),
    fields: [
      ...metaFields({ placeholder: 'internal' }),
      { key: 'spec.interfaces', label: 'Interfaces', type: 'multi', required: true, placeholder: 'add Interfaces…',
        options: (o) => objs('Interface').filter((i) => { const z = zoneOf(i.metadata.name); return !z || z.metadata.name === o.metadata.name; }).map((i) => ({ value: i.metadata.name, label: `${i.metadata.name}${i.metadata.comment ? ' · ' + i.metadata.comment : ''}` })),
        help: 'An Interface belongs to at most one Zone (DM-37); members of other Zones are not listed.' },
      { key: 'spec.intraZoneTraffic', label: 'Traffic between members', type: 'select', options: [{ value: '', label: 'deny (default)' }, { value: 'allow', label: 'allow' }],
        help: 'Verdict between two members when no RuleSet exists for that pair (DM-39). Both produce Flow Events.' },
    ],
  });
}

function dhcpForm(d) {
  const isNew = !d;
  objectForm({
    kind: 'DhcpServer', isNew, obj: d || newObject('DhcpServer', { interface: '', enabled: true, rangeStart: '', rangeEnd: '' }),
    fields: [
      ...metaFields({ placeholder: 'lan3' }),
      { key: 'spec.interface', label: 'Interface', type: 'select', required: true, immutable: !isNew,
        options: () => [{ value: '', label: '— select —' }, ...objs('Interface').filter((i) => i.spec.ipv4?.mode === 'static' && i.spec.role !== 'external').map((i) => `${i.metadata.name}`)],
        help: 'One server per Interface (R-017.3); not on external Interfaces (DM-24)' },
      { key: 'spec.enabled', label: 'State', type: 'checkbox', checkLabel: 'Enabled', help: 'Can be prepared without activating it' },
      { key: 'spec.rangeStart', label: 'Range start', required: true, placeholder: '192.168.10.100' },
      { key: 'spec.rangeEnd', label: 'Range end', required: true, placeholder: '192.168.10.199', help: 'In one subnet of the Interface, without its address (DM-25, DM-26)' },
      { key: 'spec.gateway', label: 'Gateway', placeholder: 'default: the Interface address' },
      { key: 'spec.leaseTime', label: 'Lease time', placeholder: '24h', help: 'Duration: 30m, 12h, 7d (R-017.1)' },
      { key: 'spec.dnsServers', label: 'DNS servers', type: 'textarea', full: true, format: lines, parse: splitLines, placeholder: 'default: the Interface address when the DNS server listens there' },
      { key: 'spec.reservations', label: 'Reservations', type: 'textarea', full: true, placeholder: '52:54:00:12:34:56 192.168.10.50 laptop',
        help: 'One "MAC IP [host]" per line (R-017.2, DM-29)',
        format: (v) => (v || []).map((r) => [r.mac, r.ip, r.host].filter(Boolean).join(' ')).join('\n'),
        parse: (s) => s.split('\n').map((l) => l.trim().split(/\s+/)).filter((x) => x[0]).map(([mac, ip, host]) => (host ? { mac, ip: ip || '', host } : { mac, ip: ip || '' })) },
    ],
  });
}

function routeForm(route) {
  const isNew = !route;
  objectForm({
    kind: 'StaticRoute', isNew, obj: route || newObject('StaticRoute', { destination: '', gateway: '', interface: '' }),
    fields: [
      ...metaFields({ placeholder: 'branch-office' }),
      { key: 'spec.destination', label: 'Destination', required: true, placeholder: '10.0.0.0/8' },
      { key: 'spec.gateway', label: 'Gateway', required: true, placeholder: 'in a subnet of the Interface (DM-22)' },
      { key: 'spec.interface', label: 'Outgoing Interface', type: 'select', required: true, options: () => [{ value: '', label: '— select —' }, ...objs('Interface').map((i) => i.metadata.name)] },
      { key: 'spec.distance', label: 'Distance', type: 'number', placeholder: '10', help: 'Lower is preferred (1–255)' },
      { key: 'spec.priority', label: 'Priority', type: 'number', placeholder: '1', help: 'Tie-breaker for equal distance (1–65535)' },
    ],
  });
}

// ---- views -------------------------------------------------------------------------------------

function implicitAccess(name) {
  const s = sys();
  const served = (refs) => (refs || []).some((r) => r === name || find('Zone', r)?.spec.interfaces.includes(name));
  const out = [];
  if (objs('DhcpServer').some((d) => d.spec.enabled && d.spec.interface === name)) out.push('dhcp');
  if (served(s.dns?.serveToInterfaces)) out.push('dns');
  if (s.ntp?.enabled && served(s.ntp?.serveToInterfaces)) out.push('ntp');
  return out;
}

function actions(kind, o, edit) {
  return h('td', { class: 'num' }, h('span', { class: 'row', style: { justifyContent: 'flex-end', gap: '0' } },
    h('button', { class: 'ghost', title: 'YAML', 'aria-label': 'Show YAML', onclick: () => showYAML(kind, o.metadata.name) }, icon('code')),
    isAdmin() && edit && h('button', { class: 'ghost', title: 'Edit', 'aria-label': 'Edit', onclick: () => edit(o) }, icon('edit')),
    isAdmin() && h('button', { class: 'ghost', title: 'Delete', 'aria-label': 'Delete', onclick: () => deleteObject(kind, o.metadata.name) }, icon('trash'))));
}

function ifaceRow(i, child) {
  const s = i.spec;
  const z = zoneOf(i.metadata.name);
  const implicit = implicitAccess(i.metadata.name).filter((a) => !(s.allowAccess || []).includes(a));
  return h('tr', { class: [child && 'tree-child', s.enabled === false && 'disabled'] },
    h('td', null, h('div', { class: 'row wrap' }, dot(i.metadata.color), h('button', { class: 'link', onclick: () => ifaceForm(i) }, i.metadata.name),
      s.type === 'vlan' ? h('span', { class: 'badge outline', title: `802.1Q tag on ${s.parent}` }, `VLAN ${s.vlanId}`) : h('span', { class: 'mono small muted' }, s.device),
      s.enabled === false && h('span', { class: 'badge' }, 'disabled'), findingBadge('Interface', i.metadata.name)),
    i.metadata.comment && h('div', { class: 'small muted' }, i.metadata.comment)),
    h('td', null, h('span', { class: 'badge' }, s.role || 'internal')),
    h('td', { class: 'mono' }, s.ipv4?.mode === 'static' ? (s.ipv4.addresses || []).map((a) => h('div', null, a)) : h('span', { class: 'muted small' }, s.ipv4?.mode || '—'),
      s.ipv4?.gateway && h('div', { class: 'small muted' }, `gw ${s.ipv4.gateway}`),
      s.ipv6 && s.ipv6.mode !== 'none' && h('div', { class: 'small muted', title: 'IPv6 is deferred: kept, not applied (R-057)' }, `ipv6 ${s.ipv6.mode} (deferred)`)),
    h('td', null, z ? refChip(z.metadata.name) : h('span', { class: 'muted small' }, '—')),
    h('td', null, h('div', { class: 'row wrap' },
      (s.allowAccess || []).map((a) => h('span', { class: 'badge accent' }, a)),
      implicit.map((a) => h('span', { class: 'badge outline', title: 'Opened implicitly: the firewall runs this service here (DM-34)' }, `${a} (implicit)`)),
      !(s.allowAccess || []).length && !implicit.length && h('span', { class: 'muted small' }, 'none'))),
    actions('Interface', i, ifaceForm));
}

const TABS = [
  { id: 'interfaces', label: 'Interfaces', kind: 'Interface' },
  { id: 'zones', label: 'Zones', kind: 'Zone' },
  { id: 'dhcp', label: 'DHCP servers', kind: 'DhcpServer' },
  { id: 'routes', label: 'Static routes', kind: 'StaticRoute' },
];
let tab = 'interfaces';

function interfacesView() {
  const all = objs('Interface');
  const physical = all.filter((i) => i.spec.type === 'physical');
  const orphans = all.filter((i) => i.spec.type === 'vlan' && !physical.some((p) => p.metadata.name === i.spec.parent));
  return h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
    h('thead', null, h('tr', null, ['Interface', 'Role', 'IPv4', 'Zone', 'Management access', ''].map((t) => h('th', null, t)))),
    h('tbody', null, physical.map((p) => [ifaceRow(p, false), ...all.filter((c) => c.spec.type === 'vlan' && c.spec.parent === p.metadata.name).sort((a, b) => a.spec.vlanId - b.spec.vlanId).map((c) => ifaceRow(c, true))]),
      orphans.map((o) => ifaceRow(o, false))))));
}

function zonesView() {
  const zones = objs('Zone');
  const free = objs('Interface').filter((i) => !zoneOf(i.metadata.name));
  return h('div', { class: 'stack' },
    h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Zone', 'Interfaces', 'Between members', 'Used by', ''].map((t) => h('th', null, t)))),
      h('tbody', null, zones.length ? zones.map((z) => {
        const uses = usedBy('Zone', z.metadata.name);
        return h('tr', null,
          h('td', null, h('div', { class: 'row' }, dot(z.metadata.color), h('button', { class: 'link', onclick: () => zoneForm(z) }, z.metadata.name), findingBadge('Zone', z.metadata.name)),
            z.metadata.comment && h('div', { class: 'small muted' }, z.metadata.comment)),
          h('td', null, h('div', { class: 'row wrap' }, z.spec.interfaces.map((i) => refChip(i)))),
          h('td', null, z.spec.interfaces.length > 1 ? h('span', { class: ['badge', z.spec.intraZoneTraffic === 'allow' ? 'warn' : ''] }, z.spec.intraZoneTraffic || 'deny') : h('span', { class: 'muted small' }, 'one member')),
          h('td', { class: 'small', title: uses.join('\n') }, uses.length ? `${uses.length} RuleSet${uses.length > 1 ? 's' : ''}` : h('span', { class: 'muted' }, 'unused')),
          actions('Zone', z, zoneForm));
      }) : h('tr', null, h('td', { colspan: 5, class: 'empty' }, 'No Zones: RuleSets reference Interfaces directly.')))))),
    h('p', { class: 'muted small' }, 'Interfaces outside any Zone: ', free.length ? free.map((i) => i.metadata.name).join(', ') : 'none', '. They are referenced directly by RuleSets.'));
}

function dhcpView() {
  const list = objs('DhcpServer');
  return h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
    h('thead', null, h('tr', null, ['Server', 'Interface', 'State', 'Range', 'Lease', 'Reservations', ''].map((t) => h('th', null, t)))),
    h('tbody', null, list.length ? list.map((d) => h('tr', null,
      h('td', null, h('div', { class: 'row' }, dot(d.metadata.color), h('button', { class: 'link', onclick: () => dhcpForm(d) }, d.metadata.name), findingBadge('DhcpServer', d.metadata.name)),
        d.metadata.comment && h('div', { class: 'small muted' }, d.metadata.comment)),
      h('td', null, refChip(d.spec.interface)),
      h('td', null, d.spec.enabled ? h('span', { class: 'badge good' }, 'on') : h('span', { class: 'badge' }, 'prepared, off')),
      h('td', { class: 'mono' }, `${d.spec.rangeStart} – ${d.spec.rangeEnd}`),
      h('td', { class: 'small' }, d.spec.leaseTime || h('span', { class: 'muted' }, '24h')),
      h('td', { class: 'small' }, (d.spec.reservations || []).length ? (d.spec.reservations || []).map((r) => h('div', { class: 'mono' }, `${r.mac} → ${r.ip}${r.host ? ' (' + r.host + ')' : ''}`)) : h('span', { class: 'muted' }, '—')),
      actions('DhcpServer', d, dhcpForm)))
      : h('tr', null, h('td', { colspan: 7, class: 'empty' }, 'No DHCP servers')))))) ;
}

function routesView() {
  const routes = objs('StaticRoute');
  const active = state.config.activeRoutes || {};
  const pref = (r) => [r.spec.distance || 10, r.spec.priority || 1];
  return h('div', { class: 'stack' },
    h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Name', 'Destination', 'Gateway', 'Interface', 'Distance', 'Priority', 'State', ''].map((t, k) => h('th', { class: [4, 5].includes(k) && 'num' }, t)))),
      h('tbody', null, routes.length ? [...routes].sort((a, b) => a.spec.destination.localeCompare(b.spec.destination) || pref(a)[0] - pref(b)[0] || pref(a)[1] - pref(b)[1]).map((r) => h('tr', null,
        h('td', null, h('div', { class: 'row' }, dot(r.metadata.color), h('button', { class: 'link', onclick: () => routeForm(r) }, r.metadata.name), findingBadge('StaticRoute', r.metadata.name)),
          r.metadata.comment && h('div', { class: 'small muted' }, r.metadata.comment)),
        h('td', { class: 'mono' }, r.spec.destination), h('td', { class: 'mono' }, r.spec.gateway), h('td', null, refChip(r.spec.interface)),
        h('td', { class: 'num' }, r.spec.distance || h('span', { class: 'muted' }, '10')), h('td', { class: 'num' }, r.spec.priority || h('span', { class: 'muted' }, '1')),
        h('td', null, active[r.metadata.name] ? h('span', { class: 'badge good' }, 'active') : h('span', { class: 'badge' }, 'backup')),
        actions('StaticRoute', r, routeForm)))
        : h('tr', null, h('td', { colspan: 8, class: 'empty' }, 'No static routes')))))),
    h('p', { class: 'muted small' }, 'For each destination the active route has the lowest distance, then the lowest priority; the others are backups (DM-23, R-001.5). Link state is runtime state: a route whose Interface is down is skipped and the backup takes over (reconciliation.md §8). The PoC shows the configured preference only.'));
}

const NEW = { interfaces: ['Interface', () => ifaceForm()], zones: ['Zone', () => zoneForm()], dhcp: ['DHCP server', () => dhcpForm()], routes: ['Route', () => routeForm()] };

export default {
  render(root, rest) {
    if (rest?.[0] && TABS.some((t) => t.id === rest[0])) tab = rest[0];
    const t = TABS.find((x) => x.id === tab);
    const [label, create] = NEW[tab];
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Network'), h('p', null, 'Interfaces, Zones, DHCP and routing. Logical Interfaces (VLANs) are nested under their physical Interface.')),
        h('span', { class: 'spacer' }), ['DhcpServer', 'StaticRoute'].includes(t.kind) && ext(t.kind),
        isAdmin() && h('button', { class: 'primary', onclick: create }, icon('plus'), label)),
      h('nav', { class: 'tabs' }, TABS.map((x) => h('a', { href: `#/network/${x.id}`, class: x.id === tab && 'on' }, x.label, h('span', { class: 'count' }, objs(x.kind).length)))),
      { interfaces: interfacesView, zones: zonesView, dhcp: dhcpView, routes: routesView }[tab]());
  },
};
