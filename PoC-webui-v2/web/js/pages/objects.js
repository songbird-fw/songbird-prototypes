// Objects referenced by Policies: Addresses, Address groups, Services,
// Service groups, and the Color palette (R-055). Groups may nest; cycles are
// rejected (R-011, R-013). Predefined objects keep their defining fields
// (DM-21): Colors their hex (R-055.1), Services their protocols.
import { state, h, icon, mount, objs, isAdmin, dot, refChip, objectForm, newObject, deleteObject, showYAML, usedBy, findingBadge, metaFields, addrValue, addrForm, svcValue, yamlEditor } from '../core.js';

const ifaceOpts = () => [{ value: '', label: '— not bound —' }, ...objs('Interface').map((i) => i.metadata.name)];
const predefinedService = (name) => (state.config.predefined?.Service || []).includes(name);
const predefinedColor = (name) => (state.config.predefined?.Color || []).includes(name);

// "tcp 443", "udp 10000-20000", "icmp 8 0", "ip 47", "tcp 443 from 1024-65535"
function parseProtocols(text) {
  const num = (s) => (/^\d+$/.test(s) ? Number(s) : s);
  return text.split('\n').map((l) => l.trim()).filter(Boolean).map((l) => {
    const [proto, a, b, c] = l.toLowerCase().split(/\s+/);
    const e = { protocol: proto };
    if (['tcp', 'udp', 'sctp'].includes(proto)) {
      if (a) e.port = num(a);
      if (b === 'from' && c) e.sourcePort = num(c);
    } else if (['icmp', 'icmpv6'].includes(proto)) {
      if (a) e.icmpType = Number(a);
      if (b) e.icmpCode = Number(b);
    } else if (proto === 'ip' && a) e.protocolNumber = Number(a);
    return e;
  });
}
function formatProtocols(list) {
  return (list || []).map((p) => [p.protocol, p.port ?? p.icmpType ?? p.protocolNumber, p.icmpCode, p.sourcePort != null ? `from ${p.sourcePort}` : null].filter((x) => x != null).join(' ')).join('\n');
}

const KINDS = [
  {
    kind: 'Address', one: 'address', label: 'Addresses',
    blank: () => newObject('Address', { subnet: '' }),
    // the form edits one "form" and one "value"; the schema wants exactly one
    // of subnet, range, fqdn, mac
    load: (o) => ({ ...o, x: { form: addrForm(o.spec), value: addrValue(o.spec) } }),
    toObject: (d) => {
      const { form, value = '' } = d.x || {};
      delete d.x;
      for (const k of ['subnet', 'range', 'fqdn', 'mac']) delete d.spec[k];
      if (form === 'range') { const [start, end = ''] = value.split('-').map((s) => s.trim()); d.spec.range = { start, end }; } else if (value) d.spec[form] = value.trim();
      return d;
    },
    fields: [...metaFields({ placeholder: 'web-01' }),
      { key: 'x.form', label: 'Form', type: 'select', required: true, rerender: true, paths: ['spec'], exact: true,
        options: [{ value: 'subnet', label: 'subnet (CIDR; a host is /32)' }, { value: 'range', label: 'range' }, { value: 'fqdn', label: 'fqdn' }, { value: 'mac', label: 'mac (source only)' }],
        help: (o) => (['fqdn', 'mac'].includes(o.x?.form) ? 'Not supported by the Data Plane yet: Policies using it never match (dataplane.md §7.3)' : '') },
      { key: 'x.value', label: 'Value', required: true, paths: ['spec.subnet', 'spec.range', 'spec.fqdn', 'spec.mac'],
        placeholder: '192.168.1.0/24 · 10.0.0.1-10.0.0.9 · host.example.com · 34:29:8f:74:f4:6a', help: 'CIDR only, no netmask notation (R-012)' },
      { key: 'spec.interface', label: 'Bound to Interface', type: 'select', options: ifaceOpts, help: 'Usable as source only where traffic enters from it, as destination only where it leaves through it (DM-12)' },
      { key: 'metadata.category', label: 'Category', placeholder: 'e.g. servers' }],
    cols: ['Value', 'Interface'],
    cells: (o) => [h('span', { class: 'mono' }, addrValue(o.spec), h('span', { class: 'muted small' }, ` ${addrForm(o.spec)}`)), o.spec.interface ? refChip(o.spec.interface) : h('span', { class: 'muted' }, '—')],
  },
  {
    kind: 'AddressGroup', one: 'address group', label: 'Address groups',
    blank: () => newObject('AddressGroup', { members: [] }),
    fields: [...metaFields({ placeholder: 'grp-servers' }),
      { key: 'spec.members', label: 'Members', type: 'multi', required: true, placeholder: 'add members…',
        options: (d) => [...objs('Address').map((a) => ({ value: a.metadata.name, label: `${a.metadata.name} · ${addrValue(a.spec)}${a.spec.interface ? ' @' + a.spec.interface : ''}`, group: 'Addresses' })),
          ...objs('AddressGroup').filter((g) => g.metadata.name !== d.metadata.name).map((g) => ({ value: g.metadata.name, label: g.metadata.name, group: 'Address groups' }))],
        help: 'Addresses and groups; nesting is allowed, cycles are rejected (R-013).' }],
    cols: ['Members'],
    cells: (o) => [h('div', { class: 'row wrap' }, o.spec.members.map((m) => refChip(m)))],
  },
  {
    kind: 'Service', one: 'service', label: 'Services',
    blank: () => newObject('Service', { protocols: [] }),
    fields: [...metaFields({ placeholder: 'rdp' }),
      { key: 'spec.protocols', label: 'Protocols', type: 'textarea', required: true, full: true, format: formatProtocols, parse: parseProtocols,
        lockedIf: (o) => predefinedService(o.metadata.name),
        placeholder: 'tcp 3389\nudp 3389', help: 'One entry per line: "tcp 443", "udp 10000-20000", "icmp 8", "ip 47", "tcp 443 from 1024-65535". tcp/udp/sctp need a port (R-010).' },
      { key: 'metadata.category', label: 'Category', placeholder: 'e.g. internet' }],
    cols: ['Protocols'],
    cells: (o) => [h('span', { class: 'mono' }, svcValue(o.spec))],
  },
  {
    kind: 'ServiceGroup', one: 'service group', label: 'Service groups',
    blank: () => newObject('ServiceGroup', { members: [] }),
    fields: [...metaFields({ placeholder: 'mail' }),
      { key: 'spec.members', label: 'Members', type: 'multi', required: true, placeholder: 'add members…',
        options: (d) => [...objs('Service').map((s) => ({ value: s.metadata.name, label: `${s.metadata.name} · ${svcValue(s.spec)}`, group: 'Services' })),
          ...objs('ServiceGroup').filter((g) => g.metadata.name !== d.metadata.name).map((g) => ({ value: g.metadata.name, label: g.metadata.name, group: 'Service groups' }))],
        help: 'Services and groups; nesting is allowed, cycles are rejected (R-011).' }],
    cols: ['Members'],
    cells: (o) => [h('div', { class: 'row wrap' }, o.spec.members.map((m) => refChip(m)))],
  },
  {
    kind: 'Color', one: 'color', label: 'Colors',
    blank: () => newObject('Color', { hex: '' }),
    fields: [
      { key: 'metadata.name', label: 'Name', required: true, immutable: true, placeholder: 'k8s-blue' },
      { key: 'spec.hex', label: 'Colour', type: 'color', required: true, lockedIf: (o) => !!o.spec.predefined,
        help: (o) => (o.spec.predefined ? 'Predefined: the colour is fixed so palettes stay recognisable; its meaning is yours (R-055.1)' : 'Unique among Colors (DM-33)') },
      { key: 'metadata.comment', label: 'Meaning', full: true, placeholder: 'e.g. Backup / Veeam', help: 'What this colour stands for in this network: shown in the legend and in every tooltip.' }],
    cols: ['Colour', 'Meaning', 'Used by'],
    cells: (o) => {
      const n = usedBy('Color', o.metadata.name).length;
      return [h('span', { class: 'row' }, h('span', { class: 'dot', style: { background: o.spec.hex, width: '14px', height: '14px' } }), h('span', { class: 'mono' }, o.spec.hex)),
        o.metadata.comment || h('span', { class: 'muted' }, '— no meaning yet'), n ? `${n} object${n > 1 ? 's' : ''}` : h('span', { class: 'muted' }, '—')];
    },
  },
];

let tab = 'Address';

function edit(def, o) {
  const obj = o ? (def.load ? def.load(o) : o) : (def.load ? def.load(def.blank()) : def.blank());
  const locked = def.kind === 'Service' && o && predefinedService(o.metadata.name);
  objectForm({ kind: def.kind, isNew: !o, obj, fields: def.fields, toObject: def.toObject,
    title: locked ? `Service ${o.metadata.name} (predefined: protocols are fixed)` : undefined });
}

function isPredefined(def, o) {
  return (def.kind === 'Color' && predefinedColor(o.metadata.name)) || (def.kind === 'Service' && predefinedService(o.metadata.name));
}

export default {
  render(root, rest) {
    if (rest?.[0] && KINDS.some((k) => k.kind === rest[0])) tab = rest[0];
    const def = KINDS.find((k) => k.kind === tab);
    const list = objs(def.kind);
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Objects'), h('p', null, 'Named objects referenced by Policies, by name. Predefined objects ship with Songbird and keep their defining fields (DM-21).')),
        h('span', { class: 'spacer' }),
        isAdmin() && h('button', { onclick: () => yamlEditor(def.kind, '', { isNew: true, template: `apiVersion: "config.songbird-fw.com/v1alpha1"\nkind: "${def.kind}"\nmetadata:\n  name: ""\nspec:\n` }) }, icon('code'), 'From YAML'),
        isAdmin() && h('button', { class: 'primary', onclick: () => edit(def) }, icon('plus'), `New ${def.one}`)),
      h('nav', { class: 'tabs' }, KINDS.map((k) => h('a', { href: `#/objects/${k.kind}`, class: k.kind === tab && 'on' }, k.label, h('span', { class: 'count' }, objs(k.kind).length)))),
      def.kind === 'Color' && h('div', { class: 'legend card pad', style: { marginBottom: '12px' } }, list.map((c) => h('span', { class: 'k' }, h('span', { class: 'dot', style: { background: c.spec.hex } }), h('strong', null, c.metadata.name), c.metadata.comment || h('span', { class: 'muted' }, '—')))),
      h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
        h('thead', null, h('tr', null, h('th', null, 'Name'), def.cols.map((c) => h('th', null, c)), def.kind !== 'Color' && h('th', null, 'Used by'), h('th'))),
        h('tbody', null, list.length ? list.map((o) => {
          const uses = def.kind === 'Color' ? null : usedBy(def.kind, o.metadata.name);
          const pre = isPredefined(def, o);
          return h('tr', null,
            h('td', null, h('div', { class: 'row wrap' }, def.kind !== 'Color' && dot(o.metadata.color), h('button', { class: 'link', onclick: () => edit(def, o) }, o.metadata.name),
              pre && h('span', { class: 'badge', title: 'Predefined (DM-21)' }, icon('lock'), 'predefined'),
              o.metadata.category && h('span', { class: 'badge outline' }, o.metadata.category), findingBadge(def.kind, o.metadata.name)),
            def.kind !== 'Color' && o.metadata.comment && h('div', { class: 'small muted' }, o.metadata.comment)),
            def.cells(o).map((c) => h('td', null, c)),
            uses && h('td', { class: 'small', title: uses.join('\n') }, uses.length ? `${uses.length} reference${uses.length > 1 ? 's' : ''}` : h('span', { class: 'muted' }, 'unused')),
            h('td', { class: 'num' }, h('span', { class: 'row', style: { justifyContent: 'flex-end', gap: '0' } },
              h('button', { class: 'ghost', title: 'YAML', 'aria-label': 'Show YAML', onclick: () => showYAML(def.kind, o.metadata.name) }, icon('code')),
              isAdmin() && !pre && h('button', { class: 'ghost', title: 'Delete', 'aria-label': 'Delete', onclick: () => deleteObject(def.kind, o.metadata.name) }, icon('trash')))));
        }) : h('tr', null, h('td', { colspan: 9, class: 'empty' }, 'Nothing here yet.')))))));
  },
};
