// Objects referenced by Policies: Addresses, groups, Virtual IPs, Services,
// IP pools, and the Colour palette (R-055).
import { h, icon, mount, objs, isAdmin, dot, refChip, objectForm, newObject, deleteObject, colourOptions, showYAML, usedBy } from '../core.js';

const ifaceOpts = (none = '— not bound —') => () => [{ value: '', label: none }, ...objs('Interface').map((i) => i.metadata.name)];
const name = { key: 'metadata.name', label: 'Name', required: true, immutable: true };
const colour = { key: 'metadata.colour', label: 'Colour', type: 'select', options: colourOptions };
const builtinLock = (o) => !!o.spec.builtin;

const KINDS = [
  {
    kind: 'Address', one: 'address', label: 'Addresses',
    blank: () => newObject('Address', { type: 'subnet', value: '' }),
    fields: [name, colour,
      { key: 'spec.type', label: 'Type', type: 'select', required: true, options: ['ip', 'subnet', 'range', 'fqdn', 'mac'], rerender: true, help: (o) => (['fqdn', 'mac'].includes(o.spec.type) ? 'Not yet supported by the Data Plane (dataplane.md §7.3)' : '') },
      { key: 'spec.value', label: 'Value', required: true, placeholder: '192.168.1.0/24 · 10.0.0.1-10.0.0.9 · 10.0.0.5' },
      { key: 'spec.interface', label: 'Bound to Interface', type: 'select', options: ifaceOpts(), help: 'Restricts use: as source only where the Zone enters from it, as destination only where it leaves through it (DM-12)' }],
    cols: ['Type', 'Value', 'Interface'],
    cells: (o) => [o.spec.type, h('span', { class: 'mono' }, o.spec.value), o.spec.interface || h('span', { class: 'muted' }, '—')],
  },
  {
    kind: 'AddressGroup', one: 'address group', label: 'Address groups',
    blank: () => newObject('AddressGroup', { members: [] }),
    fields: [name, colour,
      { key: 'spec.members', label: 'Members', type: 'multi', required: true, options: () => objs('Address').map((a) => ({ value: a.metadata.name, label: `${a.metadata.name} · ${a.spec.value}${a.spec.interface ? ' @' + a.spec.interface : ''}` })),
        help: 'Addresses only, flat. All members on the same Interface, or none bound (DM-15, DM-16).' },
      { key: 'spec.interface', label: 'Interface', type: 'select', options: ifaceOpts('— none —'), help: 'Must equal the Interface of the members (DM-16)' }],
    cols: ['Members', 'Interface'],
    cells: (o) => [h('div', { class: 'row wrap' }, o.spec.members.map((m) => refChip(m))), o.spec.interface || h('span', { class: 'muted' }, '—')],
  },
  {
    kind: 'VirtualIP', one: 'virtual IP', label: 'Virtual IPs',
    blank: () => newObject('VirtualIP', { external: { interface: '', ip: '', port: '' }, mapped: { ip: '', port: '' }, protocol: 'tcp' }),
    fields: [name, colour,
      { key: 'spec.external.interface', label: 'External Interface', type: 'select', required: true, options: ifaceOpts('— select —') },
      { key: 'spec.protocol', label: 'Protocol', type: 'select', required: true, options: ['tcp', 'udp'] },
      { key: 'spec.external.ip', label: 'External IP', required: true, help: 'An address of the external Interface (DM-19)' },
      { key: 'spec.external.port', label: 'External port', required: true, placeholder: '443 or 1000-1010' },
      { key: 'spec.mapped.ip', label: 'Mapped IP', required: true },
      { key: 'spec.mapped.port', label: 'Mapped port', required: true, help: 'Single port or a range of the same length (DM-20)' }],
    cols: ['External', 'Mapped', 'Protocol'],
    cells: (o) => [h('span', { class: 'mono' }, `${o.spec.external.interface} ${o.spec.external.ip}:${o.spec.external.port}`), h('span', { class: 'mono' }, `${o.spec.mapped.ip}:${o.spec.mapped.port}`), o.spec.protocol],
  },
  {
    kind: 'Service', one: 'service', label: 'Services',
    blank: () => newObject('Service', { protocol: 'tcp', ports: '' }),
    fields: [name, colour,
      { key: 'spec.protocol', label: 'Protocol', type: 'select', required: true, options: ['tcp', 'udp', 'icmp', 'any'], rerender: true, lockedIf: builtinLock },
      { key: 'spec.ports', label: 'Ports', required: true, show: (o) => ['tcp', 'udp'].includes(o.spec.protocol), placeholder: '443 or 10000-20000', help: 'Mandatory for tcp/udp, absent otherwise (DM-17)', lockedIf: builtinLock }],
    cols: ['Protocol', 'Ports'],
    cells: (o) => [o.spec.protocol, h('span', { class: 'mono' }, o.spec.ports || '—')],
  },
  {
    kind: 'ServiceGroup', one: 'service group', label: 'Service groups',
    blank: () => newObject('ServiceGroup', { members: [] }),
    fields: [name, colour,
      { key: 'spec.members', label: 'Members', type: 'multi', required: true, options: () => objs('Service').map((s) => ({ value: s.metadata.name, label: `${s.metadata.name} · ${s.spec.protocol}${s.spec.ports ? '/' + s.spec.ports : ''}` })),
        help: 'Services only, flat, all with the same protocol (DM-18)' }],
    cols: ['Members'],
    cells: (o) => [h('div', { class: 'row wrap' }, o.spec.members.map((m) => refChip(m)))],
  },
  {
    kind: 'IpPool', one: 'IP pool', label: 'IP pools',
    blank: () => newObject('IpPool', { startIp: '', endIp: '' }),
    fields: [name, colour, { key: 'spec.startIp', label: 'Start IP', required: true }, { key: 'spec.endIp', label: 'End IP', required: true }],
    cols: ['Range'],
    cells: (o) => [h('span', { class: 'mono' }, `${o.spec.startIp} – ${o.spec.endIp}`)],
  },
  {
    kind: 'Colour', one: 'colour', label: 'Colours',
    blank: () => newObject('Colour', { value: '', meaning: '' }),
    fields: [name,
      { key: 'spec.value', label: 'Colour value', type: 'colour', required: true, lockedIf: builtinLock, help: (o) => (o.spec.builtin ? 'Predefined: the value is fixed, the meaning is yours (R-055.1)' : 'Unique among Colours (DM-33)') },
      { key: 'spec.meaning', label: 'Meaning', full: true, placeholder: 'e.g. Backup / Veeam' }],
    cols: ['Value', 'Meaning', 'Used by'],
    cells: (o) => [h('span', { class: 'row' }, h('span', { class: 'dot', style: { background: o.spec.value, width: '14px', height: '14px' } }), h('span', { class: 'mono' }, o.spec.value)),
      o.spec.meaning || h('span', { class: 'muted' }, '—'), colourUse(o.metadata.name)],
  },
];

function colourUse(c) {
  let n = 0;
  for (const k of ['Interface', 'Zone', 'Address', 'AddressGroup', 'VirtualIP', 'Service', 'ServiceGroup', 'IpPool', 'StaticRoute']) {
    for (const o of objs(k)) { if (o.metadata.colour === c) n++; if (k === 'Zone') for (const p of o.spec.policies || []) if (p.colour === c) n++; }
  }
  return n ? `${n} object${n > 1 ? 's' : ''}` : h('span', { class: 'muted' }, '—');
}

let tab = 'Address';

function edit(def, o) {
  // predefined Addresses and Services are read-only; predefined Colours keep their value only
  const locked = o?.spec.builtin && def.kind !== 'Colour';
  objectForm({ kind: def.kind, isNew: !o, obj: o || def.blank(), fields: locked ? def.fields.map((f) => ({ ...f, lockedIf: () => true })) : def.fields,
    title: locked ? `${def.kind} ${o.metadata.name} (predefined, read-only)` : undefined });
}

export default {
  render(root, rest) {
    if (rest?.[0] && KINDS.some((k) => k.kind === rest[0])) tab = rest[0];
    const def = KINDS.find((k) => k.kind === tab);
    const list = objs(def.kind);
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Objects'), h('p', null, 'Named objects referenced by Policies. Predefined objects are locked (DM-21).')),
        h('span', { class: 'spacer' }), isAdmin() && h('button', { class: 'primary', onclick: () => edit(def) }, icon('plus'), `New ${def.one}`)),
      h('nav', { class: 'tabs' }, KINDS.map((k) => h('a', { href: `#/objects/${k.kind}`, class: k.kind === tab && 'on' }, k.label, h('span', { class: 'count' }, objs(k.kind).length)))),
      h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
        h('thead', null, h('tr', null, h('th', null, 'Name'), def.cols.map((c) => h('th', null, c)), def.kind !== 'Colour' && h('th', null, 'Used by'), h('th'))),
        h('tbody', null, list.length ? list.map((o) => {
          const uses = def.kind === 'Colour' ? null : usedBy(o.metadata.name);
          return h('tr', null,
            h('td', null, h('span', { class: 'row' }, def.kind !== 'Colour' && dot(o.metadata.colour), h('button', { class: 'link', onclick: () => edit(def, o) }, o.metadata.name),
              o.spec.builtin && h('span', { class: 'badge', title: 'Predefined (DM-21)' }, icon('lock'), 'builtin'))),
            def.cells(o).map((c) => h('td', null, c)),
            uses && h('td', { class: 'small', title: uses.join('\n') }, uses.length ? `${uses.length} reference${uses.length > 1 ? 's' : ''}` : h('span', { class: 'muted' }, 'unused')),
            h('td', { class: 'num' }, h('span', { class: 'row', style: { justifyContent: 'flex-end', gap: '0' } },
              h('button', { class: 'ghost', title: 'YAML', onclick: () => showYAML(def.kind, o.metadata.name) }, icon('code')),
              isAdmin() && !o.spec.builtin && h('button', { class: 'ghost', title: 'Delete', onclick: () => deleteObject(def.kind, o.metadata.name) }, icon('trash')))));
        }) : h('tr', null, h('td', { colspan: 9, class: 'empty' }, 'Nothing here yet.')))))));
  },
};
