// Network: Interfaces as a tree, VLANs under their physical parent (R-015),
// with management access (allowAccess: https, ping, ssh; R-056, DM-05);
// Zones as sets of Interfaces (R-046, DM-37, DM-39); the DHCP pools of
// System.systemServices.dhcp (R-017 … R-017.6) and static routes with the
// preference rule of DM-23. StaticRoute is a PoC extension kind: the
// v1alpha1 schema does not cover it yet.
import { state, h, icon, mount, objs, find, sys, isAdmin, colorStyle, refChip, objectForm, newObject, deleteObject, showYAML, findingBadge, findingsFor, metaFields, usedBy, save, confirmDialog, contextMenu } from '../core.js';

const lines = (v) => (v || []).join('\n');
const splitLines = (s) => s.split(/[\n,]+/).map((x) => x.trim()).filter(Boolean);
const zoneOf = (iface) => objs('Zone').find((z) => z.spec.interfaces.includes(iface));
const ext = (kind) => h('span', { class: 'badge outline', title: 'PoC extension kind: not in songbird-yaml-schema v1alpha1 (internal/schema/ext)' }, 'PoC kind');

// Management Services only (schema decision 12). DNS, NTP and DHCP are
// System Services: they open their own ports where they listen (DM-34).
const ACCESS = [
  { value: 'https', label: 'https', title: 'Web UI / API on System.admin.httpsPort, restricted by trustedHosts' },
  { value: 'ping', label: 'ping', title: 'ICMP echo' },
  { value: 'ssh', label: 'ssh', title: 'CLI over SSH (tcp/22)' },
];

// ---- forms ----------------------------------------------------------------------------------

function ifaceForm(iface) {
  const isNew = !iface;
  const obj = iface || newObject('Interface', { type: 'vlan', ipv4: { mode: 'static', addresses: [] } });
  objectForm({
    kind: 'Interface', isNew, obj,
    fields: [
      ...metaFields({ placeholder: 'lan3' }),
      { key: 'spec.type', label: 'Type', type: 'select', required: true, options: ['physical', 'vlan'], rerender: true, immutable: true },
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
        help: 'Management Services, reachable through the addresses of this Interface (owner rule). Default: none (R-056). DNS, NTP and DHCP are System Services: they open their own ports where they listen (DM-34).' },
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
    intro: h('p', { class: 'sub small', style: { margin: 0 } }, 'A Zone groups Interfaces treated alike. A RuleSet may reference the Zone or one of its members; the member\'s RuleSet is evaluated first (DM-38, DM-42).'),
    fields: [
      ...metaFields({ placeholder: 'internal' }),
      { key: 'spec.interfaces', label: 'Interfaces', type: 'multi', required: true, placeholder: 'add Interfaces…',
        options: (o) => objs('Interface').filter((i) => { const z = zoneOf(i.metadata.name); return !z || z.metadata.name === o.metadata.name; }).map((i) => ({ value: i.metadata.name, label: `${i.metadata.name}${i.metadata.comment ? ' · ' + i.metadata.comment : ''}` })),
        help: 'An Interface belongs to at most one Zone (DM-37); members of other Zones are not listed.' },
      { key: 'spec.intraZoneTraffic', label: 'Traffic between members', type: 'select', options: [{ value: '', label: 'deny (default)' }, { value: 'allow', label: 'allow' }],
        help: 'Verdict between two members when no Policy of their RuleSets matches (DM-39). Both produce Flow Events.' },
    ],
  });
}

// A DHCP pool is a block of System.spec.systemServices.dhcp, keyed by the
// Interface name (schema decision 16): the form edits the System object.
const lines2 = (v) => (v || []).join('\n');
function dhcpForm(ifname) {
  const o = find('System', 'system');
  if (!o) return;
  const pools = o.spec.systemServices?.dhcp || {};
  const isNew = !ifname;
  const obj = structuredClone(o);
  obj.x = { iface: ifname || '', pool: structuredClone(pools[ifname] || { range: { start: '', end: '' } }) };
  const free = () => objs('Interface').filter((i) => !pools[i.metadata.name]);
  objectForm({
    kind: 'System', obj, isNew: false, title: isNew ? 'New DHCP pool' : `DHCP pool on ${ifname}`,
    intro: h('p', { class: 'sub small', style: { margin: 0 } }, 'One pool per Interface, in System.spec.systemServices.dhcp. The Interface needs a static IPv4 address; the pool, the gateway and the reservations lie in its subnet (R-017.5).'),
    fields: [
      { key: 'x.iface', label: 'Interface', type: 'select', required: true, immutable: !isNew, lockedIf: () => !isNew,
        options: () => [{ value: '', label: '— select —' }, ...(isNew ? free() : objs('Interface').filter((i) => i.metadata.name === ifname)).map((i) => ({ value: i.metadata.name, label: `${i.metadata.name}${i.spec.ipv4?.mode === 'static' ? ' · ' + (i.spec.ipv4.addresses || []).join(', ') : ' · no static address'}` }))] },
      { key: 'x.pool.enabled', label: 'State', type: 'checkbox', invert: true, checkLabel: 'Enabled', help: 'A pool can be prepared without serving it' },
      { key: 'x.pool.range.start', label: 'Range start', required: true, placeholder: '192.168.10.100' },
      { key: 'x.pool.range.end', label: 'Range end', required: true, placeholder: '192.168.10.199', help: 'Inside the subnet, without the Interface address (DM-25, DM-26)' },
      { key: 'x.pool.gateway', label: 'Gateway', placeholder: 'default: the Interface address' },
      { key: 'x.pool.leaseTime', label: 'Lease time', placeholder: '12h', help: 'Duration: 30m, 12h, 7d (R-017.1)' },
      { key: 'x.pool.domain', label: 'Domain', placeholder: 'example.lan' },
      { key: 'x.pool.dnsServers', label: 'DNS servers', type: 'textarea', format: lines2, parse: splitLines, placeholder: 'default: the firewall itself',
        help: 'If the firewall is advertised (or this is empty), the DNS server must listen on this Interface or its Zone (R-017.6)' },
      { key: 'x.pool.ntpServers', label: 'NTP servers', type: 'textarea', format: lines2, parse: splitLines, placeholder: 'none advertised',
        help: 'Same rule against systemServices.ntp.listenOn' },
      { key: 'x.pool.reservations', label: 'Reservations', type: 'textarea', full: true, placeholder: '52:54:00:12:34:56 192.168.10.50 laptop',
        help: 'One "MAC address [hostname]" per line (R-017.2, DM-29)',
        format: (v) => (v || []).map((r) => [r.mac, r.address, r.hostname].filter(Boolean).join(' ')).join('\n'),
        parse: (s2) => s2.split('\n').map((l) => l.trim().split(/\s+/)).filter((x) => x[0]).map(([mac, address, hostname]) => (hostname ? { mac, address: address || '', hostname } : { mac, address: address || '' })) },
    ],
    toObject: (d) => {
      const { iface, pool } = d.x;
      delete d.x;
      if (!iface) return d;
      d.spec.systemServices ||= {};
      d.spec.systemServices.dhcp = { ...(d.spec.systemServices.dhcp || {}), [iface]: pool };
      return d;
    },
  });
}

async function deletePool(ifname) {
  if (!(await confirmDialog(`Delete the DHCP pool on ${ifname}?`, 'System is committed without it. Leases already handed out expire on their own.', 'Delete', true))) return;
  const o = structuredClone(find('System', 'system'));
  delete o.spec.systemServices.dhcp[ifname];
  if (!Object.keys(o.spec.systemServices.dhcp).length) delete o.spec.systemServices.dhcp;
  await save('PUT', '/api/v1/objects/System/system', o);
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

// System Services listening on an Interface (DM-34): they open their ports.
function implicitAccess(name) {
  const ss = sys().systemServices || {};
  const served = (refs) => (refs || []).some((r) => r === name || find('Zone', r)?.spec.interfaces.includes(name));
  const out = [];
  const pool = ss.dhcp?.[name];
  if (pool && pool.enabled !== false) out.push('dhcp');
  if (served(ss.dns?.listenOn)) out.push('dns');
  if (ss.ntp?.enabled && served(ss.ntp?.listenOn)) out.push('ntp');
  return out;
}

// Row actions live in the context menu, like the Policies.
function rowMenu(kind, o, edit) {
  return (e) => {
    e.preventDefault();
    contextMenu({ x: e.clientX, y: e.clientY }, `${kind} ${o.metadata.name}`, [
      { label: isAdmin() ? 'Edit…' : 'View…', icon: 'edit', onClick: () => edit(o) },
      { label: 'Show YAML', icon: 'code', onClick: () => showYAML(kind, o.metadata.name) },
      isAdmin() && '-',
      isAdmin() && { label: 'Delete…', icon: 'trash', danger: true, onClick: () => deleteObject(kind, o.metadata.name) },
    ]);
  };
}

function ifaceRow(i, child) {
  const s = i.spec;
  const z = zoneOf(i.metadata.name);
  const implicit = implicitAccess(i.metadata.name).filter((a) => !(s.allowAccess || []).includes(a));
  return h('tr', { class: [child && 'tree-child', s.enabled === false && 'disabled'], oncontextmenu: rowMenu('Interface', i, ifaceForm), ondblclick: () => ifaceForm(i) },
    h('td', null, h('div', { class: 'row wrap' }, h('button', { class: 'link', onclick: () => ifaceForm(i) }, i.metadata.name),
      s.type === 'vlan' ? h('span', { class: 'badge outline', title: `802.1Q tag on ${s.parent}` }, `VLAN ${s.vlanId}`) : h('span', { class: 'mono small muted' }, s.device),
      s.enabled === false && h('span', { class: 'badge' }, 'disabled'), findingBadge('Interface', i.metadata.name)),
    i.metadata.comment && h('div', { class: 'small muted' }, i.metadata.comment)),
    h('td', { class: 'mono' }, s.ipv4?.mode === 'static' ? (s.ipv4.addresses || []).map((a) => h('div', null, a)) : h('span', { class: 'muted small' }, s.ipv4?.mode || '—'),
      s.ipv4?.gateway && h('div', { class: 'small muted' }, `gw ${s.ipv4.gateway}`),
      s.ipv6 && s.ipv6.mode !== 'none' && h('div', { class: 'small muted', title: 'IPv6 is deferred: kept, not applied (R-057)' }, `ipv6 ${s.ipv6.mode} (deferred)`)),
    h('td', null, z ? refChip(z.metadata.name) : h('span', { class: 'muted small' }, '—')),
    h('td', null, h('div', { class: 'row wrap' },
      (s.allowAccess || []).map((a) => h('span', { class: 'badge accent' }, a)),
      implicit.map((a) => h('span', { class: 'badge outline', title: 'System Service listening here: opens its own port (DM-34)' }, `${a} · service`)),
      !(s.allowAccess || []).length && !implicit.length && h('span', { class: 'muted small' }, 'none'))));
}

const TABS = [
  { id: 'interfaces', label: 'Interfaces', kind: 'Interface' },
  { id: 'zones', label: 'Zones', kind: 'Zone' },
  { id: 'dhcp', label: 'DHCP', kind: 'dhcp' },
  { id: 'routes', label: 'Static routes', kind: 'StaticRoute' },
];
let tab = 'interfaces';

function interfacesView() {
  const all = objs('Interface');
  const physical = all.filter((i) => i.spec.type === 'physical');
  const orphans = all.filter((i) => i.spec.type === 'vlan' && !physical.some((p) => p.metadata.name === i.spec.parent));
  return h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
    h('thead', null, h('tr', null, ['Interface', 'IPv4', 'Zone', 'Access to the firewall'].map((t) => h('th', null, t)))),
    h('tbody', null, physical.map((p) => [ifaceRow(p, false), ...all.filter((c) => c.spec.type === 'vlan' && c.spec.parent === p.metadata.name).sort((a, b) => a.spec.vlanId - b.spec.vlanId).map((c) => ifaceRow(c, true))]),
      orphans.map((o) => ifaceRow(o, false))))));
}

function zonesView() {
  const zones = objs('Zone');
  const free = objs('Interface').filter((i) => !zoneOf(i.metadata.name));
  return h('div', { class: 'stack' },
    h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Zone', 'Interfaces', 'Between members', 'Used by'].map((t) => h('th', null, t)))),
      h('tbody', null, zones.length ? zones.map((z) => {
        const uses = usedBy('Zone', z.metadata.name);
        return h('tr', { oncontextmenu: rowMenu('Zone', z, zoneForm), ondblclick: () => zoneForm(z) },
          h('td', null, h('div', { class: 'row' }, h('button', { class: 'link', onclick: () => zoneForm(z) }, z.metadata.name), findingBadge('Zone', z.metadata.name)),
            z.metadata.comment && h('div', { class: 'small muted' }, z.metadata.comment)),
          h('td', null, h('div', { class: 'row wrap' }, z.spec.interfaces.map((i) => refChip(i)))),
          h('td', null, z.spec.interfaces.length > 1 ? h('span', { class: ['badge', z.spec.intraZoneTraffic === 'allow' ? 'warn' : ''] }, z.spec.intraZoneTraffic || 'deny') : h('span', { class: 'muted small' }, 'one member')),
          h('td', { class: 'small', title: uses.join('\n') }, uses.length ? `${uses.length} reference${uses.length > 1 ? 's' : ''}` : h('span', { class: 'muted' }, 'unused')));
      }) : h('tr', null, h('td', { colspan: 4, class: 'empty' }, 'No Zones: RuleSets reference Interfaces directly.')))))),
    h('p', { class: 'muted small' }, 'Interfaces outside any Zone: ', free.length ? free.map((i) => i.metadata.name).join(', ') : 'none', '. They are referenced directly by RuleSets.'));
}

function dhcpView() {
  const pools = Object.entries(sys().systemServices?.dhcp || {}).sort(([a], [b]) => a.localeCompare(b));
  const errs = findingsFor('System', 'system').filter((e) => e.path.startsWith('spec.systemServices.dhcp'));
  const menu = (n) => (e) => {
    e.preventDefault();
    contextMenu({ x: e.clientX, y: e.clientY }, `DHCP pool ${n}`, [
      { label: isAdmin() ? 'Edit…' : 'View…', icon: 'edit', onClick: () => dhcpForm(n) },
      { label: 'Show System YAML', icon: 'code', onClick: () => showYAML('System', 'system') },
      isAdmin() && '-',
      isAdmin() && { label: 'Delete…', icon: 'trash', danger: true, onClick: () => deletePool(n) },
    ]);
  };
  return h('div', { class: 'stack' },
    h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Interface', 'State', 'Range', 'Lease', 'Advertises', 'Reservations'].map((t) => h('th', null, t)))),
      h('tbody', null, pools.length ? pools.map(([n, p]) => {
        const mine = errs.filter((e) => e.path.startsWith(`spec.systemServices.dhcp.${n}`));
        return h('tr', { oncontextmenu: menu(n), ondblclick: () => dhcpForm(n) },
          h('td', null, h('div', { class: 'row' }, refChip(n), h('button', { class: 'link small', onclick: () => dhcpForm(n) }, 'edit'),
            mine.length ? h('span', { class: ['badge', mine.some((e) => e.severity === 'error') ? 'bad' : 'warn'], title: mine.map((e) => e.reason).join('\n') }, `${mine.length} finding${mine.length > 1 ? 's' : ''}`) : null)),
          h('td', null, p.enabled === false ? h('span', { class: 'badge' }, 'prepared, off') : h('span', { class: 'badge good' }, 'on')),
          h('td', { class: 'mono' }, `${p.range.start} – ${p.range.end}`),
          h('td', { class: 'small' }, p.leaseTime || h('span', { class: 'muted' }, '12h')),
          h('td', { class: 'small' }, h('div', null, 'dns ', h('span', { class: 'mono' }, (p.dnsServers || []).join(', ') || 'this firewall')),
            p.ntpServers && h('div', null, 'ntp ', h('span', { class: 'mono' }, p.ntpServers.join(', '))), p.domain && h('div', { class: 'muted' }, p.domain)),
          h('td', { class: 'small' }, (p.reservations || []).length ? p.reservations.map((r) => h('div', { class: 'mono' }, `${r.mac} → ${r.address}${r.hostname ? ' (' + r.hostname + ')' : ''}`)) : h('span', { class: 'muted' }, '—')));
      }) : h('tr', null, h('td', { colspan: 6, class: 'empty' }, 'No DHCP pools')))))),
    h('p', { class: 'muted small' }, 'Pools are System Services (System.spec.systemServices.dhcp, one block per Interface). The firewall answers DHCP on UDP 67 where a pool is enabled, without a Policy (DM-34).'));
}

function routesView() {
  const routes = objs('StaticRoute');
  const active = state.config.activeRoutes || {};
  const pref = (r) => [r.spec.distance || 10, r.spec.priority || 1];
  return h('div', { class: 'stack' },
    h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Name', 'Destination', 'Gateway', 'Interface', 'Distance', 'Priority', 'State'].map((t, k) => h('th', { class: [4, 5].includes(k) && 'num' }, t)))),
      h('tbody', null, routes.length ? [...routes].sort((a, b) => a.spec.destination.localeCompare(b.spec.destination) || pref(a)[0] - pref(b)[0] || pref(a)[1] - pref(b)[1]).map((r) => h('tr', { oncontextmenu: rowMenu('StaticRoute', r, routeForm), ondblclick: () => routeForm(r) },
        h('td', null, h('div', { class: 'row' }, h('button', { class: 'link tinted', style: colorStyle(r.metadata.color), onclick: () => routeForm(r) }, r.metadata.name), findingBadge('StaticRoute', r.metadata.name)),
          r.metadata.comment && h('div', { class: 'small muted' }, r.metadata.comment)),
        h('td', { class: 'mono' }, r.spec.destination), h('td', { class: 'mono' }, r.spec.gateway), h('td', null, refChip(r.spec.interface)),
        h('td', { class: 'num' }, r.spec.distance || h('span', { class: 'muted' }, '10')), h('td', { class: 'num' }, r.spec.priority || h('span', { class: 'muted' }, '1')),
        h('td', null, active[r.metadata.name] ? h('span', { class: 'badge good' }, 'active') : h('span', { class: 'badge' }, 'backup'))))
        : h('tr', null, h('td', { colspan: 7, class: 'empty' }, 'No static routes')))))),
    h('p', { class: 'muted small' }, 'For each destination the active route has the lowest distance, then the lowest priority; the others are backups (DM-23, R-001.5). Link state is runtime state: a route whose Interface is down is skipped and the backup takes over (reconciliation.md §8). The PoC shows the configured preference only.'));
}

const NEW = { interfaces: ['Interface', () => ifaceForm()], zones: ['Zone', () => zoneForm()], dhcp: ['DHCP pool', () => dhcpForm()], routes: ['Route', () => routeForm()] };
const count = (kind) => (kind === 'dhcp' ? Object.keys(sys().systemServices?.dhcp || {}).length : objs(kind).length);

export default {
  render(root, rest) {
    if (rest?.[0] && TABS.some((t) => t.id === rest[0])) tab = rest[0];
    const t = TABS.find((x) => x.id === tab);
    const [label, create] = NEW[tab];
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Network'), h('p', null, 'Interfaces, Zones, DHCP pools and routing. Logical Interfaces (VLANs) are nested under their physical Interface. Interfaces have no role: what they may do follows from their Zone, their management access and the System Services listening on them.')),
        h('span', { class: 'spacer' }), t.kind === 'StaticRoute' && ext(t.kind),
        isAdmin() && h('button', { class: 'primary', onclick: create }, icon('plus'), label)),
      h('nav', { class: 'tabs' }, TABS.map((x) => h('a', { href: `#/network/${x.id}`, class: x.id === tab && 'on' }, x.label, h('span', { class: 'count' }, count(x.kind))))),
      h('div', { class: 'hint', style: { marginBottom: '12px' } }, 'Row actions:', h('kbd', null, 'right click'), '· edit', h('kbd', null, 'double click')),
      { interfaces: interfacesView, zones: zonesView, dhcp: dhcpView, routes: routesView }[tab]());
  },
};
