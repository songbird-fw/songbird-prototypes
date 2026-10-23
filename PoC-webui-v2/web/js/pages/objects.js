// Objects referenced by Policies: Addresses, Address groups, Services,
// Service groups, and the Color palette (R-055). Groups are flat: an
// AddressGroup holds Addresses, a ServiceGroup Services (DM-15, DM-18).
// Predefined objects ship with Songbird and keep their defining fields
// (DM-21): the Addresses any / any6 and the Service any, and the 16 Colors
// (their hex, R-055.1). Their comment stays editable.
import { state, h, icon, mount, objs, isAdmin, dot, refChip, objectForm, newObject, deleteObject, showYAML, usedBy, findingBadge, metaFields, addrValue, addrForm, svcValue, yamlEditor, contextMenu } from '../core.js';

const ifaceOpts = () => [{ value: '', label: '— not bound —' }, ...objs('Interface').map((i) => i.metadata.name)];
const predefinedOf = (kind, name) => (state.config.predefined?.[kind] || []).includes(name);

// "tcp 443", "udp 10000-20000", "icmp 8 0", "ip 47" (no sourcePort: schema decision 17)
function parseProtocols(text) {
  const num = (s) => (/^\d+$/.test(s) ? Number(s) : s);
  return text.split('\n').map((l) => l.trim()).filter(Boolean).map((l) => {
    const [proto, a, b] = l.toLowerCase().split(/\s+/);
    const e = { protocol: proto };
    if (['tcp', 'udp', 'sctp'].includes(proto)) {
      if (a) e.port = num(a);
    } else if (['icmp', 'icmpv6'].includes(proto)) {
      if (a) e.icmpType = Number(a);
      if (b) e.icmpCode = Number(b);
    } else if (proto === 'ip' && a) e.protocolNumber = Number(a);
    return e;
  });
}
function formatProtocols(list) {
  return (list || []).map((p) => [p.protocol, p.port ?? p.icmpType ?? p.protocolNumber, p.icmpCode].filter((x) => x != null).join(' ')).join('\n');
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
      { key: 'x.form', label: 'Form', type: 'select', required: true, rerender: true, paths: ['spec'], exact: true, lockedIf: (o) => !!o.spec.predefined,
        options: [{ value: 'subnet', label: 'subnet (CIDR; a host is /32)' }, { value: 'range', label: 'range' }, { value: 'fqdn', label: 'fqdn' }, { value: 'mac', label: 'mac (source only)' }],
        help: (o) => (o.spec.predefined ? 'Predefined (DM-21): only the comment is editable' : ['fqdn', 'mac'].includes(o.x?.form) ? 'Can be defined, but enabled Policies cannot use it until the Data Plane supports it (DM-43)' : '') },
      { key: 'x.value', label: 'Value', required: true, paths: ['spec.subnet', 'spec.range', 'spec.fqdn', 'spec.mac'], lockedIf: (o) => !!o.spec.predefined,
        placeholder: '192.168.1.0/24 · 10.0.0.1-10.0.0.9 · host.example.com · 34:29:8f:74:f4:6a', help: 'CIDR only, no netmask notation (R-012)' },
      { key: 'spec.interface', label: 'Bound to Interface', type: 'select', options: ifaceOpts, lockedIf: (o) => !!o.spec.predefined, help: 'Usable as source only where traffic enters from it, as destination only where it leaves through it (DM-12)' },
      { key: 'metadata.category', label: 'Category', placeholder: 'e.g. servers' }],
    cols: ['Value', 'Interface'],
    cells: (o) => [h('span', { class: 'mono' }, addrValue(o.spec), h('span', { class: 'muted small' }, ` ${addrForm(o.spec)}`)), o.spec.interface ? refChip(o.spec.interface) : h('span', { class: 'muted' }, '—')],
  },
  {
    kind: 'AddressGroup', one: 'address group', label: 'Address groups',
    blank: () => newObject('AddressGroup', { members: [] }),
    fields: [...metaFields({ placeholder: 'grp-servers' }),
      { key: 'spec.members', label: 'Members', type: 'multi', ns: 'address', required: true, placeholder: 'add Addresses…',
        options: () => objs('Address').map((a) => ({ value: a.metadata.name, label: `${a.metadata.name} · ${addrValue(a.spec)}${a.spec.interface ? ' @' + a.spec.interface : ''}` })),
        help: 'Addresses only: groups are flat (DM-15). Members may be bound to different Interfaces; DM-12 applies to each.' }],
    cols: ['Members'],
    cells: (o) => [h('div', { class: 'row wrap', style: { gap: '4px' } }, o.spec.members.map((m) => refChip(m, null, 'address')))],
  },
  {
    kind: 'Service', one: 'service', label: 'Services',
    blank: () => newObject('Service', { protocols: [] }),
    fields: [...metaFields({ placeholder: 'rdp' }),
      { key: 'spec.protocols', label: 'Protocols', type: 'textarea', required: true, full: true, format: formatProtocols, parse: parseProtocols,
        lockedIf: (o) => !!o.spec.predefined,
        placeholder: 'tcp 3389\nudp 3389', help: (o) => (o.spec.predefined ? 'Predefined (DM-21): every protocol and port' : 'One entry per line: "tcp 443", "udp 10000-20000", "icmp 8", "ip 47". tcp/udp/sctp need a port (R-010); "any" is reserved to the predefined Service.') },
      { key: 'metadata.category', label: 'Category', placeholder: 'e.g. internet' }],
    cols: ['Protocols'],
    cells: (o) => [h('span', { class: 'mono' }, o.spec.predefined ? 'every protocol and port' : svcValue(o.spec))],
  },
  {
    kind: 'ServiceGroup', one: 'service group', label: 'Service groups',
    blank: () => newObject('ServiceGroup', { members: [] }),
    fields: [...metaFields({ placeholder: 'mail' }),
      { key: 'spec.members', label: 'Members', type: 'multi', ns: 'service', required: true, placeholder: 'add Services…',
        options: () => objs('Service').filter((s) => !s.spec.predefined).map((s) => ({ value: s.metadata.name, label: `${s.metadata.name} · ${svcValue(s.spec)}` })),
        help: 'Services only: groups are flat and may mix protocols (DM-18).' }],
    cols: ['Members'],
    cells: (o) => [h('div', { class: 'row wrap', style: { gap: '4px' } }, o.spec.members.map((m) => refChip(m, null, 'service')))],
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
  const locked = o && isPredefined(def, o) && def.kind !== 'Color';
  objectForm({ kind: def.kind, isNew: !o, obj, fields: def.fields, toObject: def.toObject,
    title: locked ? `${def.kind} ${o.metadata.name} (predefined: only the comment is editable)` : undefined });
}

function isPredefined(def, o) {
  return predefinedOf(def.kind, o.metadata.name) && !!o.spec.predefined;
}

function rowMenu(def, o) {
  return (e) => {
    e.preventDefault();
    const pre = isPredefined(def, o);
    contextMenu({ x: e.clientX, y: e.clientY }, `${def.kind} ${o.metadata.name}`, [
      { label: isAdmin() ? 'Edit…' : 'View…', icon: 'edit', onClick: () => edit(def, o) },
      { label: 'Show YAML', icon: 'code', onClick: () => showYAML(def.kind, o.metadata.name) },
      isAdmin() && { label: 'Edit YAML…', icon: 'code', onClick: () => yamlEditor(def.kind, o.metadata.name) },
      isAdmin() && '-',
      isAdmin() && { label: pre ? 'Delete (predefined)' : 'Delete…', icon: 'trash', danger: true, disabled: pre, title: pre ? 'Predefined objects cannot be deleted (DM-21)' : null, onClick: () => deleteObject(def.kind, o.metadata.name) },
    ]);
  };
}

export default {
  render(root, rest) {
    if (rest?.[0] && KINDS.some((k) => k.kind === rest[0])) tab = rest[0];
    const def = KINDS.find((k) => k.kind === tab);
    const list = objs(def.kind);
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Objects'), h('p', null, 'Named objects referenced by Policies, by name. Predefined objects (any, any6, the Service any, the 16 Colors) ship with Songbird and keep their defining fields (DM-21).')),
        h('span', { class: 'spacer' }),
        isAdmin() && h('button', { onclick: () => yamlEditor(def.kind, '', { isNew: true, template: `apiVersion: "config.songbird-fw.com/v1alpha1"\nkind: "${def.kind}"\nmetadata:\n  name: ""\nspec:\n` }) }, icon('code'), 'From YAML'),
        isAdmin() && h('button', { class: 'primary', onclick: () => edit(def) }, icon('plus'), `New ${def.one}`)),
      h('nav', { class: 'tabs' }, KINDS.map((k) => h('a', { href: `#/objects/${k.kind}`, class: k.kind === tab && 'on' }, k.label, h('span', { class: 'count' }, objs(k.kind).length)))),
      h('div', { class: 'hint', style: { marginBottom: '12px' } }, 'Row actions:', h('kbd', null, 'right click'), '· edit', h('kbd', null, 'double click')),
      def.kind === 'Color' && h('div', { class: 'legend card pad', style: { marginBottom: '12px' } }, list.map((c) => h('span', { class: 'k' }, h('span', { class: 'dot', style: { background: c.spec.hex } }), h('strong', { style: { fontWeight: 500 } }, c.metadata.name), c.metadata.comment || h('span', { class: 'muted' }, '—')))),
      h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
        h('thead', null, h('tr', null, h('th', null, 'Name'), def.cols.map((c) => h('th', null, c)), def.kind !== 'Color' && h('th', null, 'Used by'))),
        h('tbody', null, list.length ? list.map((o) => {
          const uses = def.kind === 'Color' ? null : usedBy(def.kind, o.metadata.name);
          const pre = isPredefined(def, o);
          return h('tr', { oncontextmenu: rowMenu(def, o), ondblclick: () => edit(def, o) },
            h('td', null, h('div', { class: 'row wrap' }, def.kind !== 'Color' && dot(o.metadata.color), h('button', { class: 'link', onclick: () => edit(def, o) }, o.metadata.name),
              pre && h('span', { class: 'badge', title: 'Predefined (DM-21)' }, icon('lock'), 'predefined'),
              o.metadata.category && h('span', { class: 'badge outline' }, o.metadata.category), findingBadge(def.kind, o.metadata.name)),
            def.kind !== 'Color' && o.metadata.comment && h('div', { class: 'small muted' }, o.metadata.comment)),
            def.cells(o).map((c) => h('td', null, c)),
            uses && h('td', { class: 'small', title: uses.join('\n') }, uses.length ? `${uses.length} reference${uses.length > 1 ? 's' : ''}` : h('span', { class: 'muted' }, 'unused')));
        }) : h('tr', null, h('td', { colspan: 9, class: 'empty' }, 'Nothing here yet.')))))));
  },
};
