// Interfaces as a tree, VLANs under their physical parent (R-015), with
// administrative access (allowAccess, DM-05) and DHCP (DM-24…DM-29).
// Static routes with the preference rule of DM-23.
import { state, h, icon, mount, objs, isAdmin, dot, objectForm, newObject, deleteObject, colourOptions, showYAML } from '../core.js';

const lines = (v) => (v || []).join('\n');
const splitLines = (s) => s.split(/[\n,]+/).map((x) => x.trim()).filter(Boolean);

function ifaceForm(iface) {
  const isNew = !iface;
  const obj = iface || newObject('Interface', { role: 'lan', type: 'physical', addresses: [] });
  objectForm({
    kind: 'Interface', isNew, obj,
    fields: [
      { key: 'metadata.name', label: 'Name', required: true, immutable: true, placeholder: 'LAN2' },
      { key: 'metadata.colour', label: 'Colour', type: 'select', options: colourOptions },
      { key: 'spec.type', label: 'Type', type: 'select', required: true, options: ['physical', 'vlan'], rerender: true, immutable: true },
      { key: 'spec.role', label: 'Role', type: 'select', required: true, options: ['lan', 'wan', 'dmz'], rerender: true, help: 'DHCP server only on lan (DM-24)' },
      { key: 'spec.parent', label: 'Parent (physical)', type: 'select', required: true, show: (o) => o.spec.type === 'vlan',
        options: () => [{ value: '', label: '— select —' }, ...objs('Interface').filter((i) => i.spec.type === 'physical').map((i) => i.metadata.name)] },
      { key: 'spec.vlanId', label: 'VLAN ID', type: 'number', required: true, show: (o) => o.spec.type === 'vlan', placeholder: '1–4094' },
      { key: 'spec.addresses', label: 'IPv4 addresses', type: 'textarea', required: true, full: true, placeholder: '192.168.1.1/24', format: lines, parse: splitLines,
        help: 'One address/prefix per line. Subnets must not overlap other Interfaces (DM-04). IPv6 is deferred.' },
      { key: 'spec.allowAccess', label: 'Administrative access', type: 'checks', full: true, options: [{ value: 'ping', label: 'ping' }, { value: 'https', label: 'https (Web UI / API)' }],
        help: 'What may be reached on the firewall through this Interface\'s addresses. Default: none (DM-05).' },
      { key: 'spec.dhcp.enabled', label: 'DHCP server', type: 'checkbox', checkLabel: 'Enabled', show: (o) => o.spec.role === 'lan', rerender: true },
      { key: 'spec.dhcp.rangeStart', label: 'Range start', show: (o) => o.spec.role === 'lan' && o.spec.dhcp?.enabled, required: true },
      { key: 'spec.dhcp.rangeEnd', label: 'Range end', show: (o) => o.spec.role === 'lan' && o.spec.dhcp?.enabled, required: true },
      { key: 'spec.dhcp.gateway', label: 'Gateway', show: (o) => o.spec.role === 'lan' && o.spec.dhcp?.enabled, placeholder: 'default: Interface IP' },
      { key: 'spec.dhcp.leaseTime', label: 'Lease time (s)', type: 'number', show: (o) => o.spec.role === 'lan' && o.spec.dhcp?.enabled, placeholder: '86400' },
      { key: 'spec.dhcp.reservations', label: 'Reservations', type: 'textarea', full: true, show: (o) => o.spec.role === 'lan' && o.spec.dhcp?.enabled,
        placeholder: '52:54:00:12:34:56 192.168.1.20', help: 'One "MAC IP" per line (R-017.2, DM-29)',
        format: (v) => (v || []).map((r) => `${r.mac} ${r.ip}`).join('\n'),
        parse: (s) => s.split('\n').map((l) => l.trim().split(/\s+/)).filter((x) => x[0]).map(([mac, ip]) => ({ mac, ip: ip || '' })) },
    ],
  });
}

function routeForm(route) {
  const isNew = !route;
  objectForm({
    kind: 'StaticRoute', isNew, obj: route || newObject('StaticRoute', { destination: '', gateway: '', outInterface: '' }),
    fields: [
      { key: 'metadata.name', label: 'Name', required: true, immutable: true },
      { key: 'metadata.colour', label: 'Colour', type: 'select', options: colourOptions },
      { key: 'spec.destination', label: 'Destination', required: true, placeholder: '10.0.0.0/8' },
      { key: 'spec.gateway', label: 'Gateway', required: true, placeholder: 'in a subnet of the Interface (DM-22)' },
      { key: 'spec.outInterface', label: 'Out Interface', type: 'select', required: true, options: () => [{ value: '', label: '— select —' }, ...objs('Interface').map((i) => i.metadata.name)] },
      { key: 'spec.distance', label: 'Distance', type: 'number', placeholder: '10', help: 'Lower is preferred' },
      { key: 'spec.priority', label: 'Priority', type: 'number', placeholder: '1', help: 'Tie-breaker for equal distance' },
    ],
  });
}

function row(i, child) {
  const s = i.spec;
  const d = s.dhcp;
  return h('tr', { class: child && 'tree-child' },
    h('td', null, h('span', { class: 'row' }, dot(i.metadata.colour), h('button', { class: 'link', onclick: () => ifaceForm(i) }, i.metadata.name),
      s.type === 'vlan' && h('span', { class: 'badge outline' }, `VLAN ${s.vlanId}`))),
    h('td', null, h('span', { class: 'badge' }, s.role)),
    h('td', { class: 'mono' }, s.addresses.map((a) => h('div', null, a))),
    h('td', null, h('div', { class: 'row wrap' }, (s.allowAccess || []).length ? s.allowAccess.map((a) => h('span', { class: 'badge accent' }, a)) : h('span', { class: 'muted small' }, 'none'))),
    h('td', { class: 'small' }, d ? (d.enabled ? h('span', null, h('span', { class: 'badge good' }, 'on'), ' ', h('span', { class: 'mono' }, `${d.rangeStart} – ${d.rangeEnd}`),
      d.reservations?.length ? h('span', { class: 'muted' }, ` · ${d.reservations.length} reserved`) : null) : h('span', { class: 'badge' }, 'configured, off')) : h('span', { class: 'muted' }, '—')),
    h('td', { class: 'num' }, h('span', { class: 'row', style: { justifyContent: 'flex-end', gap: '0' } },
      h('button', { class: 'ghost', title: 'YAML', onclick: () => showYAML('Interface', i.metadata.name) }, icon('code')),
      isAdmin() && h('button', { class: 'ghost', title: 'Delete', onclick: () => deleteObject('Interface', i.metadata.name) }, icon('trash')))));
}

export default {
  render(root) {
    const all = objs('Interface');
    const physical = all.filter((i) => i.spec.type === 'physical');
    const orphans = all.filter((i) => i.spec.type === 'vlan' && !physical.some((p) => p.metadata.name === i.spec.parent));
    const routes = objs('StaticRoute');
    const active = state.config.activeRoutes || {};
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Interfaces & routing'), h('p', null, 'Logical Interfaces (VLANs) are nested under their physical Interface.')),
        h('span', { class: 'spacer' }), isAdmin() && h('button', { class: 'primary', onclick: () => ifaceForm() }, icon('plus'), 'Interface')),
      h('div', { class: 'stack' },
        h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
          h('thead', null, h('tr', null, ['Interface', 'Role', 'Addresses', 'Admin access', 'DHCP server', ''].map((t) => h('th', null, t)))),
          h('tbody', null, physical.map((p) => [row(p, false), ...all.filter((c) => c.spec.type === 'vlan' && c.spec.parent === p.metadata.name).sort((a, b) => a.spec.vlanId - b.spec.vlanId).map((c) => row(c, true))]),
            orphans.map((o) => row(o, false)))))),
        h('div', { class: 'page-head', style: { marginBottom: 0, marginTop: '8px' } },
          h('div', null, h('h2', null, 'Static routes'), h('p', null, 'For each destination the active route has the lowest distance, then the lowest priority; the others are backups (DM-23).')),
          h('span', { class: 'spacer' }), isAdmin() && h('button', { onclick: () => routeForm() }, icon('plus'), 'Route')),
        h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
          h('thead', null, h('tr', null, ['Name', 'Destination', 'Gateway', 'Interface', 'Distance', 'Priority', 'State', ''].map((t, k) => h('th', { class: [4, 5].includes(k) && 'num' }, t)))),
          h('tbody', null, routes.length ? [...routes].sort((a, b) => a.spec.destination.localeCompare(b.spec.destination) || (a.spec.distance || 10) - (b.spec.distance || 10)).map((r) => h('tr', null,
            h('td', null, h('span', { class: 'row' }, dot(r.metadata.colour), h('button', { class: 'link', onclick: () => routeForm(r) }, r.metadata.name))),
            h('td', { class: 'mono' }, r.spec.destination), h('td', { class: 'mono' }, r.spec.gateway), h('td', null, r.spec.outInterface),
            h('td', { class: 'num' }, r.spec.distance || h('span', { class: 'muted' }, '10')), h('td', { class: 'num' }, r.spec.priority || h('span', { class: 'muted' }, '1')),
            h('td', null, active[r.metadata.name] ? h('span', { class: 'badge good' }, 'active') : h('span', { class: 'badge' }, 'backup')),
            h('td', { class: 'num' }, isAdmin() && h('button', { class: 'ghost', title: 'Delete', onclick: () => deleteObject('StaticRoute', r.metadata.name) }, icon('trash')))))
            : h('tr', null, h('td', { colspan: 8, class: 'empty' }, 'No static routes')))))),
        h('p', { class: 'muted small' }, 'Link state is runtime state: a route whose Interface is down is skipped by the kernel and the backup takes over (reconciliation.md §8). The PoC shows configured preference only.')));
  },
};
