// Policies grouped by Zone (domain-model.md §2): each Zone is a directed
// Interface pair and owns an ordered list; first match wins (R-014.4).
// Create, edit, delete and reorder (R-009); required fields block save (R-009.1).
import {
  state, h, icon, mount, objs, isAdmin, dot, refChip, save, modal, multiPicker, validationBox,
  colourOptions, confirmDialog, showYAML, objectForm, newObject, deleteObject, api, fmtNum, colourValue,
} from '../core.js';

const collapsed = new Set();
let filter = '';

const zoneOf = (name) => objs('Zone').find((z) => z.metadata.name === name);
const ifaceColour = (n) => objs('Interface').find((i) => i.metadata.name === n)?.metadata.colour;

// Options for the pickers, filtered by the binding rules so the user only
// sees what the Validator would accept (DM-10, DM-11, DM-12).
function addressOptions(zone, side) {
  const want = side === 'src' ? zone.spec.srcInterface : zone.spec.dstInterface;
  const ok = (bound) => !bound || bound === want;
  const out = [];
  for (const i of objs('Interface')) if (i.metadata.name === want) out.push({ value: `${i.metadata.name} addresses`, label: `${i.metadata.name} addresses (derived)`, group: 'Interface addresses' });
  for (const a of objs('Address')) if (ok(a.spec.interface)) out.push({ value: a.metadata.name, label: `${a.metadata.name}  ·  ${a.spec.value}`, group: 'Addresses' });
  for (const g of objs('AddressGroup')) if (ok(g.spec.interface)) out.push({ value: g.metadata.name, label: g.metadata.name, group: 'Address groups' });
  if (side === 'dst') for (const v of objs('VirtualIP')) if (v.spec.external.interface === zone.spec.srcInterface)
    out.push({ value: v.metadata.name, label: `${v.metadata.name}  ·  ${v.spec.external.ip}:${v.spec.external.port}`, group: 'Virtual IPs' });
  return out;
}
function serviceOptions() {
  return [
    ...objs('Service').map((s) => ({ value: s.metadata.name, label: `${s.metadata.name}  ·  ${s.spec.protocol}${s.spec.ports ? '/' + s.spec.ports : ''}`, group: 'Services' })),
    ...objs('ServiceGroup').map((s) => ({ value: s.metadata.name, label: s.metadata.name, group: 'Service groups' })),
  ];
}

async function saveZone(zone) {
  return save('PUT', `/api/v1/objects/Zone/${encodeURIComponent(zone.metadata.name)}`, zone);
}

// ---- policy editor ---------------------------------------------------------------
function editPolicy(zoneName, index) {
  const zone = structuredClone(zoneOf(zoneName));
  const isNew = index == null;
  const p = isNew ? { name: '', sources: [], destinations: [], services: [], action: 'accept', nat: { mode: 'none' } } : structuredClone(zone.spec.policies[index]);
  if (!p.nat) p.nat = { mode: 'none' };
  const ro = !isAdmin();
  let serverErrs = [], serverMsg = '';
  const fields = {};
  const field = (key, label, input, req, help) => (fields[key] = h('div', { class: ['field', ['sources', 'destinations', 'services'].includes(key) && 'full'] },
    h('label', null, label, req && h('span', { class: 'req' }, ' *')), input, help && h('div', { class: 'help' }, help), h('div', { class: 'err' })));
  const errBox = h('div');
  const saveBtn = h('button', { class: 'primary', onclick: () => submit() }, isNew ? 'Create' : 'Save');

  const required = ['name', 'sources', 'destinations', 'services'];
  const missing = () => required.filter((k) => (Array.isArray(p[k]) ? p[k].length === 0 : !String(p[k] || '').trim()));
  const base = isNew ? zone.spec.policies.length : index;
  const prefix = `spec.policies[${base}]`;

  function proposed() {
    const z = structuredClone(zone);
    const pol = structuredClone(p);
    if (pol.nat.mode === 'none') delete pol.nat;
    if (pol.nat && pol.nat.mode !== 'pool') delete pol.nat.pool;
    if (pol.enabled !== false) delete pol.enabled;
    if (!pol.colour) delete pol.colour;
    if (isNew) z.spec.policies.push(pol); else z.spec.policies[index] = pol;
    return z;
  }
  function paint() {
    const miss = new Set(missing());
    for (const [k, el] of Object.entries(fields)) {
      const own = serverErrs.filter((e) => e.path.startsWith(`${prefix}.${k}`));
      el.classList.toggle('invalid', miss.has(k) || own.length > 0);
      el.querySelector('.err').textContent = miss.has(k) ? 'Required' : own.map((e) => e.reason).join('; ');
    }
    mount(errBox, serverErrs.length ? validationBox(serverErrs, serverMsg) : null);
    saveBtn.disabled = ro || miss.size > 0 || serverErrs.length > 0;
  }
  let t;
  const changed = () => { paint(); clearTimeout(t); t = setTimeout(dry, 350); };
  async function dry() {
    if (ro || missing().length) { serverErrs = []; paint(); return; }
    const r = await api('POST', `/api/v1/validate/Zone/${encodeURIComponent(zoneName)}`, proposed()).catch(() => null);
    if (r) { serverErrs = r.valid ? [] : r.validation || []; serverMsg = r.error; }
    paint();
  }
  async function submit() {
    if (missing().length) return paint();
    saveBtn.disabled = true;
    const r = await saveZone(proposed());
    if (r.ok) return close();
    if (r.error.status === 422) { serverErrs = r.error.data.validation || []; serverMsg = r.error.data.error; paint(); }
    else if (r.error.status === 409) close();
  }

  const natPool = h('select', { disabled: ro, onchange: (e) => { p.nat.pool = e.target.value; changed(); } },
    h('option', { value: '' }, '— select an IP pool —'), objs('IpPool').map((x) => h('option', { value: x.metadata.name, selected: p.nat.pool === x.metadata.name }, `${x.metadata.name} (${x.spec.startIp}–${x.spec.endIp})`)));
  const poolField = h('div', { class: ['field', p.nat.mode !== 'pool' && 'hidden'] }, h('label', null, 'IP pool', h('span', { class: 'req' }, ' *')), natPool);

  const body = h('div', { class: 'stack' },
    h('div', { class: 'row sub' }, 'Zone', dot(zone.metadata.colour), h('strong', null, zone.metadata.name), h('span', { class: 'muted' }, '·'), zone.spec.srcInterface, icon('arrow'), zone.spec.dstInterface,
      h('span', { class: 'spacer' }), h('span', { class: 'small muted' }, isNew ? `will be position ${base + 1}` : `position ${index + 1}`)),
    errBox,
    h('div', { class: 'form-grid' },
      field('name', 'Name', h('input', { value: p.name, disabled: ro, autocomplete: 'off', oninput: (e) => { p.name = e.target.value; changed(); } }), true, 'Unique in the Zone. Together with the Zone it identifies the Policy in the Traffic Log.'),
      field('action', 'Action', h('div', { class: 'seg', role: 'group' }, ['accept', 'drop'].map((a) => h('button', { type: 'button', disabled: ro, class: p.action === a ? 'on' : '', onclick: (e) => { p.action = a; [...e.target.parentNode.children].forEach((b) => b.classList.toggle('on', b.textContent === a)); changed(); } }, a))), true),
      field('sources', `Sources (enter from ${zone.spec.srcInterface})`, ro ? h('div', { class: 'chips' }, p.sources.map((s) => refChip(s))) : multiPicker(p.sources, addressOptions(zone, 'src'), changed, 'select sources…'), true,
        `Only objects not bound to an Interface, or bound to ${zone.spec.srcInterface}, are listed (DM-12).`),
      field('destinations', `Destinations (leave via ${zone.spec.dstInterface})`, ro ? h('div', { class: 'chips' }, p.destinations.map((s) => refChip(s))) : multiPicker(p.destinations, addressOptions(zone, 'dst'), changed, 'select destinations…'), true,
        `Virtual IPs entering from ${zone.spec.srcInterface} are listed too (DM-11).`),
      field('services', 'Services', ro ? h('div', { class: 'chips' }, p.services.map((s) => refChip(s))) : multiPicker(p.services, serviceOptions(), changed, 'select services…'), true),
      field('nat', 'NAT', h('select', { disabled: ro, onchange: (e) => { p.nat.mode = e.target.value; poolField.classList.toggle('hidden', p.nat.mode !== 'pool'); changed(); } },
        [['none', 'No NAT'], ['interface', 'Masquerade to the outgoing Interface'], ['pool', 'Source NAT to an IP pool']].map(([v, l]) => h('option', { value: v, selected: p.nat.mode === v }, l)))),
      poolField,
      field('colour', 'Colour', h('select', { disabled: ro, onchange: (e) => { p.colour = e.target.value; changed(); } }, colourOptions().map((o) => h('option', { value: o.value, selected: (p.colour || '') === o.value }, o.label)))),
      field('enabled', 'State', h('label', { class: 'check' }, h('input', { type: 'checkbox', disabled: ro, checked: p.enabled !== false, onchange: (e) => { p.enabled = e.target.checked; changed(); } }), 'Enabled'))));

  const close = modal({
    title: isNew ? 'New Policy' : `Policy ${p.name}`, body,
    foot: (c) => [h('button', { class: 'ghost', onclick: () => showYAML('Zone', zoneName) }, icon('code'), 'Zone YAML'), h('span', { class: 'spacer' }),
      ro && h('span', { class: 'muted small' }, 'read-only Role'), h('button', { onclick: c }, 'Cancel'), saveBtn],
  });
  paint();
  if (!isNew) dry();
}

async function move(zoneName, from, to) {
  const z = structuredClone(zoneOf(zoneName));
  const list = z.spec.policies;
  if (to < 0 || to >= list.length || from === to) return;
  const [p] = list.splice(from, 1);
  list.splice(to, 0, p);
  await saveZone(z);
}

async function removePolicy(zoneName, i) {
  const z = structuredClone(zoneOf(zoneName));
  if (!(await confirmDialog(`Delete Policy ${z.spec.policies[i].name}?`, 'The Zone file is committed without it. Flows it accepted are terminated at their next packet (ADR-0004).', 'Delete', true))) return;
  z.spec.policies.splice(i, 1);
  await saveZone(z);
}

async function toggle(zoneName, i) {
  const z = structuredClone(zoneOf(zoneName));
  const p = z.spec.policies[i];
  if (p.enabled === false) delete p.enabled; else p.enabled = false;
  await saveZone(z);
}

function zoneForm(zone) {
  const isNew = !zone;
  const ifaces = () => objs('Interface').map((i) => ({ value: i.metadata.name, label: `${i.metadata.name} (${i.spec.role})` }));
  objectForm({
    kind: 'Zone', isNew, obj: zone || newObject('Zone', { srcInterface: '', dstInterface: '', policies: [] }),
    fields: [
      { key: 'metadata.name', label: 'Name', required: true, immutable: true, placeholder: 'LAN-WAN1', help: 'Convention: SRC-DST' },
      { key: 'metadata.colour', label: 'Colour', type: 'select', options: colourOptions },
      { key: 'spec.srcInterface', label: 'Source Interface', type: 'select', required: true, options: () => [{ value: '', label: '— select —' }, ...ifaces()], help: 'Where traffic enters' },
      { key: 'spec.dstInterface', label: 'Destination Interface', type: 'select', required: true, options: () => [{ value: '', label: '— select —' }, ...ifaces()], help: 'Where traffic leaves; must differ (DM-06)' },
    ],
  });
}

// ---- page -------------------------------------------------------------------------
function policyRow(zone, p, i, n) {
  const hits = state.counters?.hits?.[`${zone.metadata.name}/${p.name}`] || 0;
  const admin = isAdmin();
  const tr = h('tr', { class: [p.enabled === false && 'disabled'], draggable: admin ? 'true' : null,
    ondragstart: (e) => { e.dataTransfer.setData('text/plain', JSON.stringify({ z: zone.metadata.name, i })); e.dataTransfer.effectAllowed = 'move'; },
    ondragover: (e) => { e.preventDefault(); tr.classList.add('drag-over'); },
    ondragleave: () => tr.classList.remove('drag-over'),
    ondrop: (e) => { e.preventDefault(); tr.classList.remove('drag-over'); try { const d = JSON.parse(e.dataTransfer.getData('text/plain')); if (d.z === zone.metadata.name) move(d.z, d.i, i); } catch { /* not ours */ } },
    ondblclick: () => editPolicy(zone.metadata.name, i) },
  h('td', { class: 'num' }, h('span', { class: 'row' }, admin && h('span', { class: 'drag', title: 'Drag to reorder' }, icon('grip')), i + 1)),
  h('td', null, h('span', { class: 'row' }, dot(p.colour), h('button', { class: 'link', onclick: () => editPolicy(zone.metadata.name, i) }, p.name), p.enabled === false && h('span', { class: 'badge' }, 'disabled'))),
  h('td', null, h('div', { class: 'row wrap' }, p.sources.map((s) => refChip(s)))),
  h('td', null, h('div', { class: 'row wrap' }, p.destinations.map((s) => refChip(s)))),
  h('td', null, h('div', { class: 'row wrap' }, p.services.map((s) => refChip(s)))),
  h('td', null, h('span', { class: `action-${p.action}` }, p.action === 'accept' ? '✓ accept' : '✕ drop')),
  h('td', { class: 'small' }, !p.nat || p.nat.mode === 'none' ? h('span', { class: 'muted' }, '—') : p.nat.mode === 'pool' ? `pool ${p.nat.pool}` : 'masquerade'),
  h('td', { class: 'num', 'data-hit': `${zone.metadata.name}/${p.name}`, title: 'Flows matched since start (simulated Data Plane)' }, fmtNum(hits)),
  h('td', { class: 'num' }, admin && h('span', { class: 'row', style: { justifyContent: 'flex-end', gap: '0' } },
    h('button', { class: 'ghost', title: 'Move up', disabled: i === 0, onclick: () => move(zone.metadata.name, i, i - 1) }, icon('up')),
    h('button', { class: 'ghost', title: 'Move down', disabled: i === n - 1, onclick: () => move(zone.metadata.name, i, i + 1) }, icon('down')),
    h('button', { class: 'ghost', title: p.enabled === false ? 'Enable' : 'Disable', onclick: () => toggle(zone.metadata.name, i) }, icon('power')),
    h('button', { class: 'ghost', title: 'Delete', onclick: () => removePolicy(zone.metadata.name, i) }, icon('trash')))));
  return tr;
}

function matches(p) {
  if (!filter) return true;
  const f = filter.toLowerCase();
  return [p.name, ...p.sources, ...p.destinations, ...p.services, p.action].some((x) => x.toLowerCase().includes(f));
}

function zoneCard(z) {
  const isCol = collapsed.has(z.metadata.name);
  const all = z.spec.policies || [];
  const shown = all.map((p, i) => [p, i]).filter(([p]) => matches(p));
  const limit = state.config.limits?.policiesPerZone;
  const enabled = all.filter((p) => p.enabled !== false).length;
  return h('section', { class: ['card', 'zone', isCol && 'collapsed'] },
    h('div', { class: 'head', onclick: (e) => { if (e.target.closest('button')) return; isCol ? collapsed.delete(z.metadata.name) : collapsed.add(z.metadata.name); window.songbirdRerender(); } },
      icon('chev', 'chev'),
      z.metadata.colour && h('span', { class: 'stripe', style: { background: colourValue(z.metadata.colour) } }),
      h('h2', null, z.metadata.name),
      h('span', { class: 'flow' }, dot(ifaceColour(z.spec.srcInterface)), z.spec.srcInterface, icon('arrow'), dot(ifaceColour(z.spec.dstInterface)), z.spec.dstInterface),
      h('span', { class: 'badge outline', title: `per-Zone limit ${limit} (DM-36)` }, `${enabled} / ${limit}`),
      h('span', { class: 'spacer' }),
      h('button', { class: 'ghost sm', onclick: () => showYAML('Zone', z.metadata.name), title: 'Show the YAML file' }, icon('code')),
      isAdmin() && h('button', { class: 'ghost sm', onclick: () => zoneForm(z), title: 'Edit Zone' }, icon('edit')),
      isAdmin() && h('button', { class: 'ghost sm', onclick: () => deleteObject('Zone', z.metadata.name), title: 'Delete Zone' }, icon('trash')),
      isAdmin() && h('button', { class: 'sm', onclick: () => editPolicy(z.metadata.name) }, icon('plus'), 'Policy')),
    !isCol && h('div', { class: 'table-wrap' }, h('table', { class: 'policies' },
      h('colgroup', null, ['64px', '15%', '19%', '19%', '14%', '90px', '11%', '70px', '150px'].map((w) => h('col', { style: { width: w } }))),
      h('thead', null, h('tr', null, ['#', 'Name', 'Source', 'Destination', 'Service', 'Action', 'NAT', 'Hits', ''].map((t, k) => h('th', { class: [0, 7].includes(k) && 'num' }, t)))),
      h('tbody', null,
        shown.map(([p, i]) => policyRow(z, p, i, all.length)),
        h('tr', { class: 'implicit' }, h('td', { class: 'num' }, '—'), h('td', null, 'Default Deny'), h('td', null, 'any'), h('td', null, 'any'), h('td', null, 'any'),
          h('td', null, h('span', { class: 'action-drop' }, '✕ drop')), h('td', { colspan: 3, class: 'small' }, 'implicit, logged (DM-09)'))))));
}

export default {
  render(root) {
    const zones = [...objs('Zone')].sort((a, b) => a.spec.srcInterface.localeCompare(b.spec.srcInterface) || a.spec.dstInterface.localeCompare(b.spec.dstInterface));
    const search = h('input', { placeholder: 'Filter policies by name, object, service…', value: filter, style: { maxWidth: '320px' },
      oninput: (e) => { filter = e.target.value; const pos = e.target.selectionStart; window.songbirdRerender(); const s = root.querySelector('input'); s?.focus(); s?.setSelectionRange(pos, pos); } });
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Policies'), h('p', null, 'One section per Zone (a directed Interface pair). Order matters: the first matching Policy wins; unmatched traffic is dropped.')),
        h('span', { class: 'spacer' }), search,
        h('button', { onclick: () => { zones.forEach((z) => collapsed.add(z.metadata.name)); window.songbirdRerender(); } }, 'Collapse all'),
        isAdmin() && h('button', { class: 'primary', onclick: () => zoneForm() }, icon('plus'), 'Zone')),
      h('div', { class: 'stack' }, zones.length ? zones.map(zoneCard) : h('div', { class: 'card empty' }, 'No Zones yet.')));
  },
  refresh() { // update hit counters in place, keeping focus and scroll
    for (const td of document.querySelectorAll('[data-hit]')) {
      const n = state.counters?.hits?.[td.dataset.hit] || 0;
      td.textContent = fmtNum(n);
    }
  },
};
