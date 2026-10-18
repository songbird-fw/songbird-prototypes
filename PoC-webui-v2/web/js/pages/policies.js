// Policies grouped by RuleSet (domain-model.md v0.6, ruleset.schema.json):
// one RuleSet per ordered (from, to) pair of Interfaces or Zones, holding an
// ordered list of rules; the first match wins (R-014.4), unmatched traffic is
// dropped (Default Deny, R-033). NAT is a section of the rule (ADR-0010).
// Create, edit, delete, reorder (R-009), disable (R-009.2); required fields
// block save (R-009.1).
import {
  state, h, icon, mount, objs, find, isAdmin, dot, refChip, save, modal, multiPicker, validationBox,
  confirmDialog, showYAML, yamlEditor, objectForm, newObject, deleteObject, api, fmtNum, colorHex,
  addrValue, svcValue, findingBadge, findingsFor, metaFields,
} from '../core.js';

const collapsed = new Set();
let filter = '';

// ---- the network side of a RuleSet ---------------------------------------------------------

const zoneOf = (iface) => objs('Zone').find((z) => z.spec.interfaces.includes(iface))?.metadata.name || '';
const side = (ref) => find('Zone', ref)?.spec.interfaces || (find('Interface', ref) ? [ref] : []);
const intraZone = (rs) => find('Interface', rs.spec.from) && find('Interface', rs.spec.to) && zoneOf(rs.spec.from) && zoneOf(rs.spec.from) === zoneOf(rs.spec.to);

function sideChip(ref) {
  const z = find('Zone', ref);
  const title = z ? `Zone: ${z.spec.interfaces.join(', ')}` : find('Interface', ref) ? `Interface ${ref}` : 'missing';
  return h('span', { class: ['chip', z && 'group'], title }, dot((z || find('Interface', ref))?.metadata.color), ref);
}

// ---- address and service expansion (as the Validator does it) ------------------------------------

function expandAddr(name, seen = new Set()) {
  if (seen.has(name)) return [];
  seen.add(name);
  const a = find('Address', name);
  if (a) return [a];
  const g = find('AddressGroup', name);
  return g ? g.spec.members.flatMap((m) => expandAddr(m, seen)) : [];
}
const bound = (name) => [...new Set(expandAddr(name).map((a) => a.spec.interface).filter(Boolean))];
const fits = (name, ifaces) => bound(name).every((i) => ifaces.includes(i));
const singleHostOrRange = (a) => !!a.spec.range || /\/32$/.test(a.spec.subnet || '');

// Options for the pickers, filtered so that the user only sees what the
// Validator would accept (DM-12: an Address bound to an Interface only on
// the side that contains it; for dnat the destination is the public address
// of the incoming side, DM-40).
function addressOptions(ifaces) {
  const out = [];
  for (const a of objs('Address')) if (fits(a.metadata.name, ifaces)) out.push({ value: a.metadata.name, label: `${a.metadata.name}  ·  ${addrValue(a.spec)}`, group: 'Addresses' });
  for (const g of objs('AddressGroup')) if (fits(g.metadata.name, ifaces)) out.push({ value: g.metadata.name, label: g.metadata.name, group: 'Address groups' });
  return out;
}
function serviceOptions() {
  return [
    ...objs('Service').map((s) => ({ value: s.metadata.name, label: `${s.metadata.name}  ·  ${svcValue(s.spec)}`, group: 'Services' })),
    ...objs('ServiceGroup').map((s) => ({ value: s.metadata.name, label: s.metadata.name, group: 'Service groups' })),
  ];
}

async function saveRuleSet(rs) {
  return save('PUT', `/api/v1/objects/RuleSet/${encodeURIComponent(rs.metadata.name)}`, rs);
}

// ---- rule editor ---------------------------------------------------------------------------------

function editRule(rsName, index) {
  const rs = structuredClone(find('RuleSet', rsName));
  const isNew = index == null;
  const p = isNew ? { name: '', source: [], destination: [], service: [], action: 'accept' } : structuredClone(rs.spec.rules[index]);
  const ro = !isAdmin();
  const fromIfs = side(rs.spec.from), toIfs = side(rs.spec.to);
  let serverErrs = [], serverMsg = '', serverWarn = [];
  const fields = {};
  const field = (key, label, input, req, help, full) => (fields[key] = h('div', { class: ['field', full && 'full'] },
    h('label', null, label, req && h('span', { class: 'req' }, ' *')), input, help && h('div', { class: 'help' }, help), h('div', { class: 'err' })));
  const errBox = h('div');
  const saveBtn = h('button', { class: 'primary', onclick: () => submit() }, isNew ? 'Create' : 'Save');

  const pos = isNew ? rs.spec.rules.length : index;
  const prefix = `spec.rules[${pos}]`;
  const required = ['name', 'source', 'destination', 'service'];
  const missing = () => {
    const m = required.filter((k) => (Array.isArray(p[k]) ? p[k].length === 0 : !String(p[k] || '').trim()));
    if (p.nat?.type === 'dnat' && !p.nat.translateIpTo) m.push('nat.translateIpTo');
    return m;
  };

  function proposed() {
    const z = structuredClone(rs);
    const r = structuredClone(p);
    if (!r.nat?.type) delete r.nat;
    else {
      if (!r.nat.translateIpTo) delete r.nat.translateIpTo;
      if (!r.nat.translateServiceTo || r.nat.type !== 'dnat') delete r.nat.translateServiceTo;
    }
    if (r.action === 'drop') delete r.nat;
    if (r.enabled !== false) delete r.enabled;
    for (const k of ['comment', 'log']) if (!r[k]) delete r[k];
    if (isNew) z.spec.rules.push(r); else z.spec.rules[index] = r;
    return z;
  }
  function paint() {
    const miss = new Set(missing());
    for (const [k, el] of Object.entries(fields)) {
      const own = serverErrs.filter((e) => e.path === `${prefix}.${k}` || e.path.startsWith(`${prefix}.${k}[`) || e.path.startsWith(`${prefix}.${k}.`));
      el.classList.toggle('invalid', miss.has(k) || own.length > 0);
      el.querySelector('.err').textContent = miss.has(k) ? 'Required' : own.map((e) => e.reason).join('; ');
    }
    const mine = serverErrs.filter((e) => !Object.keys(fields).some((k) => e.path.startsWith(`${prefix}.${k}`)));
    mount(errBox, validationBox(mine, mine.length ? serverMsg : null, serverWarn.filter((w) => w.path.startsWith(prefix))));
    saveBtn.disabled = ro || miss.size > 0 || serverErrs.length > 0;
  }
  let t;
  const changed = () => { paint(); clearTimeout(t); t = setTimeout(dry, 350); };
  async function dry() {
    if (ro || missing().length) { serverErrs = []; paint(); return; }
    const r = await api('POST', `/api/v1/validate/RuleSet/${encodeURIComponent(rsName)}`, proposed()).catch(() => null);
    if (r) { serverErrs = r.valid ? [] : r.validation || []; serverMsg = r.error; serverWarn = r.warnings || []; }
    paint();
  }
  async function submit() {
    if (missing().length) return paint();
    saveBtn.disabled = true;
    const r = await saveRuleSet(proposed());
    if (r.ok) return close();
    if (r.error.status === 422) { serverErrs = r.error.data.validation || []; serverMsg = r.error.data.error; serverWarn = r.error.data.warnings || []; paint(); }
    else if (r.error.status === 409) close();
  }

  // NAT section: none, snat (masquerade or IP pool), dnat (translate to)
  const natBox = h('div', { class: 'form-grid full' });
  function renderNat() {
    const type = p.action === 'drop' ? '' : (p.nat?.type || '');
    const sel = (key, label, options, req, help) => field(`nat.${key}`, label, h('select', { disabled: ro, onchange: (e) => { p.nat[key] = e.target.value; changed(); } },
      options.map((o) => h('option', { value: o.value, selected: (p.nat?.[key] || '') === o.value }, o.label))), req, help);
    for (const k of Object.keys(fields)) if (k.startsWith('nat.')) delete fields[k];
    const els = [];
    if (type === 'snat') {
      els.push(sel('translateIpTo', 'Translate source to', [{ value: '', label: 'the address of the outgoing Interface (masquerade)' },
        ...objs('Address').filter((a) => a.spec.subnet || a.spec.range).map((a) => ({ value: a.metadata.name, label: `${a.metadata.name} · ${addrValue(a.spec)} (IP pool)` }))], false,
      'Omitted = masquerade, which also covers a dynamic WAN address (R-002).'));
    } else if (type === 'dnat') {
      els.push(sel('translateIpTo', 'Translate destination to', [{ value: '', label: '— select the internal server —' },
        ...objs('Address').filter((a) => singleHostOrRange(a) && fits(a.metadata.name, toIfs)).map((a) => ({ value: a.metadata.name, label: `${a.metadata.name} · ${addrValue(a.spec)}` }))], true,
      `A single host (/32) or a range, reachable through ${rs.spec.to} (DM-13).`));
      els.push(sel('translateServiceTo', 'Translate port to', [{ value: '', label: 'unchanged' },
        ...objs('Service').filter((s) => s.spec.protocols.length === 1 && ['tcp', 'udp', 'sctp'].includes(s.spec.protocols[0].protocol) && Number.isInteger(s.spec.protocols[0].port))
          .map((s) => ({ value: s.metadata.name, label: `${s.metadata.name} · ${svcValue(s.spec)}` }))], false,
      'A Service with one tcp/udp/sctp entry and a single port, same protocol as the rule\'s service.'));
    }
    mount(natBox, els);
  }
  const natSel = h('select', { disabled: ro, onchange: (e) => { p.nat = e.target.value ? { type: e.target.value } : undefined; renderNat(); renderPickers(); changed(); } },
    [['', 'No NAT'], ['snat', 'snat — rewrite the source'], ['dnat', 'dnat — publish a server (port forwarding)']].map(([v, l]) => h('option', { value: v, selected: (p.nat?.type || '') === v }, l)));

  const pickers = h('div', { class: 'form-grid full' });
  function renderPickers() {
    const dnat = p.nat?.type === 'dnat';
    const picker = (key, label, options, help) => field(key, label, ro ? h('div', { class: 'chips' }, p[key].map((s) => refChip(s))) : multiPicker(p[key], options, changed, `select ${key}…`, true), true, help, true);
    mount(pickers,
      picker('source', `Source (enters from ${rs.spec.from})`, addressOptions(fromIfs), `Objects bound to an Interface are listed only if it is in ${rs.spec.from} (DM-12).`),
      picker('destination', dnat ? `Destination before translation (the public address on ${rs.spec.from})` : `Destination (leaves through ${rs.spec.to})`,
        addressOptions(dnat ? fromIfs : toIfs), dnat ? 'A dnat rule matches the address and service before translation (ADR-0010, DM-40).' : null),
      picker('service', 'Service', serviceOptions(), dnat ? 'Explicit when the port is translated (DM-40).' : null));
  }

  const actionSeg = h('div', { class: 'seg', role: 'group' }, ['accept', 'drop'].map((a) => h('button', { type: 'button', disabled: ro, class: p.action === a ? 'on' : '', onclick: (e) => {
    p.action = a; [...e.target.parentNode.children].forEach((b) => b.classList.toggle('on', b.textContent === a));
    natSel.disabled = ro || a === 'drop'; if (a === 'drop') { delete p.nat; natSel.value = ''; renderNat(); renderPickers(); } changed();
  } }, a)));
  natSel.disabled = ro || p.action === 'drop';

  const body = h('div', { class: 'stack' },
    h('div', { class: 'row sub wrap' }, 'RuleSet', dot(rs.metadata.color), h('strong', null, rs.metadata.name), h('span', { class: 'muted' }, '·'), sideChip(rs.spec.from), icon('arrow'), sideChip(rs.spec.to),
      h('span', { class: 'spacer' }), h('span', { class: 'small muted' }, isNew ? `will be position ${pos + 1}` : `position ${index + 1}`)),
    errBox,
    h('div', { class: 'form-grid' },
      field('name', 'Name', h('input', { value: p.name, disabled: ro, autocomplete: 'off', oninput: (e) => { p.name = e.target.value; changed(); } }), true,
        'Unique in the RuleSet. With the RuleSet it identifies the Policy in the Traffic Log (R-026).'),
      field('action', 'Action', actionSeg, true),
      field('comment', 'Comment', h('input', { value: p.comment || '', disabled: ro, oninput: (e) => { p.comment = e.target.value; changed(); } }), false, null, true),
      pickers,
      field('nat', 'NAT', natSel, false, 'Not allowed with drop.'),
      field('log', 'Log', h('select', { disabled: ro, onchange: (e) => { p.log = e.target.value; changed(); } },
        [['', 'security (default)'], ['all', 'all'], ['none', 'none']].map(([v, l]) => h('option', { value: v, selected: (p.log || '') === v }, l))), false,
        'Stored as in the schema. The PoC Data Plane emits every Flow (R-026); see RA open issue #8.'),
      natBox,
      field('enabled', 'State', h('label', { class: 'check' }, h('input', { type: 'checkbox', disabled: ro, checked: p.enabled !== false, onchange: (e) => { p.enabled = e.target.checked; changed(); } }), 'Enabled'),
        false, 'A disabled rule stays in the YAML with enabled: false (R-009.2).')));
  renderPickers(); renderNat();

  const close = modal({
    title: isNew ? 'New Policy' : `Policy ${p.name}`, body,
    foot: (c) => [h('button', { class: 'ghost', onclick: () => showYAML('RuleSet', rsName) }, icon('code'), 'RuleSet YAML'), h('span', { class: 'spacer' }),
      ro && h('span', { class: 'muted small' }, 'read-only Role'), h('button', { onclick: c }, 'Cancel'), saveBtn],
  });
  paint();
  if (!isNew) dry();
}

async function move(rsName, from, to) {
  const rs = structuredClone(find('RuleSet', rsName));
  const list = rs.spec.rules;
  if (to < 0 || to >= list.length || from === to) return;
  const [p] = list.splice(from, 1);
  list.splice(to, 0, p);
  await saveRuleSet(rs);
}

async function removeRule(rsName, i) {
  const rs = structuredClone(find('RuleSet', rsName));
  if (!(await confirmDialog(`Delete Policy ${rs.spec.rules[i].name}?`, 'The RuleSet file is committed without it. Flows it accepted are terminated at their next packet (ADR-0004, System.session.onRuleChange).', 'Delete', true))) return;
  rs.spec.rules.splice(i, 1);
  await saveRuleSet(rs);
}

async function toggle(rsName, i) {
  const rs = structuredClone(find('RuleSet', rsName));
  const p = rs.spec.rules[i];
  if (p.enabled === false) delete p.enabled; else p.enabled = false;
  await saveRuleSet(rs);
}

// From / To: Zones, Interfaces outside any Zone, and Zone members (usable
// only between two members of the same Zone, DM-38).
function sideOptions() {
  const out = [{ value: '', label: '— select —' }];
  for (const z of objs('Zone')) out.push({ value: z.metadata.name, label: `${z.metadata.name} (Zone: ${z.spec.interfaces.join(', ')})` });
  for (const i of objs('Interface')) {
    const z = zoneOf(i.metadata.name);
    out.push({ value: i.metadata.name, label: z ? `${i.metadata.name} (member of ${z}: intra-zone only)` : `${i.metadata.name} (Interface)` });
  }
  return out;
}

function ruleSetForm(rs) {
  const isNew = !rs;
  objectForm({
    kind: 'RuleSet', isNew, obj: rs || newObject('RuleSet', { from: '', to: '', rules: [] }),
    intro: isNew && h('p', { class: 'sub small', style: { margin: 0 } }, 'One RuleSet per ordered pair; "any" is not allowed. An Interface that belongs to a Zone is referenced through its Zone, except between two members of the same Zone (DM-38).'),
    fields: [
      ...metaFields({ placeholder: 'internal-to-untrusted' }),
      { key: 'spec.from', label: 'From (where traffic enters)', type: 'select', required: true, immutable: !isNew, options: sideOptions },
      { key: 'spec.to', label: 'To (where traffic leaves)', type: 'select', required: true, immutable: !isNew, options: sideOptions, help: 'Must differ from From (DM-06)' },
    ],
  });
}

// ---- page -------------------------------------------------------------------------------------

function natCell(p) {
  if (!p.nat) return h('span', { class: 'muted' }, '—');
  if (p.nat.type === 'snat') return h('span', { title: 'Source NAT' }, 'snat ', p.nat.translateIpTo ? refChip(p.nat.translateIpTo) : h('span', { class: 'muted' }, 'masquerade'));
  return h('span', { class: 'row wrap', style: { gap: '4px' }, title: 'Destination NAT, matched before translation' }, 'dnat →', refChip(p.nat.translateIpTo),
    p.nat.translateServiceTo && refChip(p.nat.translateServiceTo));
}

function ruleRow(rs, p, i, n) {
  const key = `${rs.metadata.name}/${p.name}`;
  const hits = state.counters?.hits?.[key] || 0;
  const admin = isAdmin();
  const errs = findingsFor('RuleSet', rs.metadata.name).filter((e) => e.path === `spec.rules[${i}]` || e.path.startsWith(`spec.rules[${i}].`));
  const tr = h('tr', { class: [p.enabled === false && 'disabled'], draggable: admin ? 'true' : null,
    ondragstart: (e) => { e.dataTransfer.setData('text/plain', JSON.stringify({ rs: rs.metadata.name, i })); e.dataTransfer.effectAllowed = 'move'; },
    ondragover: (e) => { e.preventDefault(); tr.classList.add('drag-over'); },
    ondragleave: () => tr.classList.remove('drag-over'),
    ondrop: (e) => { e.preventDefault(); tr.classList.remove('drag-over'); try { const d = JSON.parse(e.dataTransfer.getData('text/plain')); if (d.rs === rs.metadata.name) move(d.rs, d.i, i); } catch { /* not ours */ } },
    ondblclick: () => editRule(rs.metadata.name, i) },
  h('td', { class: 'num' }, h('span', { class: 'row' }, admin && h('span', { class: 'drag', title: 'Drag to reorder' }, icon('grip')), i + 1)),
  h('td', null, h('div', { class: 'row wrap' }, h('button', { class: 'link', onclick: () => editRule(rs.metadata.name, i) }, p.name), p.enabled === false && h('span', { class: 'badge' }, 'disabled'),
    errs.length ? h('span', { class: ['badge', errs.some((e) => e.severity === 'error') ? 'bad' : 'warn'], title: errs.map((e) => e.reason).join('\n') }, errs.some((e) => e.severity === 'error') ? 'error' : 'warning') : null),
  p.comment && h('div', { class: 'small muted' }, p.comment)),
  h('td', null, h('div', { class: 'row wrap' }, p.source.map((s) => refChip(s)))),
  h('td', null, h('div', { class: 'row wrap' }, p.destination.map((s) => refChip(s)))),
  h('td', null, h('div', { class: 'row wrap' }, p.service.map((s) => refChip(s)))),
  h('td', null, h('span', { class: `action-${p.action}` }, p.action === 'accept' ? '✓ accept' : '✕ drop')),
  h('td', { class: 'small' }, natCell(p)),
  h('td', { class: 'small muted' }, p.log || 'security'),
  h('td', { class: 'num', 'data-hit': key, title: 'Flows matched since start (simulated Data Plane)' }, fmtNum(hits)),
  h('td', { class: 'num' }, admin && h('span', { class: 'row', style: { justifyContent: 'flex-end', gap: '0' } },
    h('button', { class: 'ghost', title: 'Move up', 'aria-label': 'Move up', disabled: i === 0, onclick: () => move(rs.metadata.name, i, i - 1) }, icon('up')),
    h('button', { class: 'ghost', title: 'Move down', 'aria-label': 'Move down', disabled: i === n - 1, onclick: () => move(rs.metadata.name, i, i + 1) }, icon('down')),
    h('button', { class: 'ghost', title: p.enabled === false ? 'Enable' : 'Disable', 'aria-label': 'Enable or disable', onclick: () => toggle(rs.metadata.name, i) }, icon('power')),
    h('button', { class: 'ghost', title: 'Delete', 'aria-label': 'Delete', onclick: () => removeRule(rs.metadata.name, i) }, icon('trash')))));
  return tr;
}

function matches(p) {
  if (!filter) return true;
  const f = filter.toLowerCase();
  return [p.name, p.comment || '', ...p.source, ...p.destination, ...p.service, p.action, p.nat?.type || '', p.nat?.translateIpTo || ''].some((x) => x.toLowerCase().includes(f));
}

function ruleSetCard(rs) {
  const name = rs.metadata.name;
  const isCol = collapsed.has(name);
  const all = rs.spec.rules || [];
  const shown = all.map((p, i) => [p, i]).filter(([p]) => matches(p));
  const limit = state.config.limits?.policiesPerRuleSet;
  const enabled = all.filter((p) => p.enabled !== false).length;
  const pairs = side(rs.spec.from).flatMap((f) => side(rs.spec.to).filter((t) => t !== f).map((t) => `${f} → ${t}`));
  return h('section', { class: ['card', 'zone', isCol && 'collapsed'] },
    h('div', { class: 'head', onclick: (e) => { if (e.target.closest('button')) return; isCol ? collapsed.delete(name) : collapsed.add(name); window.songbirdRerender(); } },
      icon('chev', 'chev'),
      rs.metadata.color && h('span', { class: 'stripe', style: { background: colorHex(rs.metadata.color) } }),
      h('h2', null, name),
      h('span', { class: 'flow', title: `Interface pairs: ${pairs.join(', ')}` }, sideChip(rs.spec.from), icon('arrow'), sideChip(rs.spec.to)),
      intraZone(rs) && h('span', { class: 'badge outline', title: 'Both Interfaces belong to the same Zone (DM-38)' }, `intra-zone ${zoneOf(rs.spec.from)}`),
      h('span', { class: 'badge outline', title: `enabled Policies / per-RuleSet limit (DM-36, ADR-0006)` }, `${enabled} / ${fmtNum(limit)}`),
      findingBadge('RuleSet', name),
      rs.metadata.comment && h('span', { class: 'small muted', style: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '360px' } }, rs.metadata.comment),
      h('span', { class: 'spacer' }),
      h('button', { class: 'ghost sm', onclick: () => showYAML('RuleSet', name), title: 'Show the YAML file', 'aria-label': 'Show YAML' }, icon('code')),
      isAdmin() && h('button', { class: 'ghost sm', onclick: () => ruleSetForm(rs), title: 'Edit RuleSet', 'aria-label': 'Edit RuleSet' }, icon('edit')),
      isAdmin() && h('button', { class: 'ghost sm', onclick: () => deleteObject('RuleSet', name), title: 'Delete RuleSet', 'aria-label': 'Delete RuleSet' }, icon('trash')),
      isAdmin() && h('button', { class: 'sm', onclick: () => editRule(name) }, icon('plus'), 'Policy')),
    !isCol && h('div', { class: 'table-wrap' }, h('table', { class: 'policies' },
      h('colgroup', null, ['56px', '14%', '16%', '16%', '13%', '100px', '14%', '72px', '60px', '140px'].map((w) => h('col', { style: { width: w } }))),
      h('thead', null, h('tr', null, ['#', 'Name', 'Source', 'Destination', 'Service', 'Action', 'NAT', 'Log', 'Hits', ''].map((t, k) => h('th', { class: [0, 8].includes(k) && 'num' }, t)))),
      h('tbody', null,
        shown.map(([p, i]) => ruleRow(rs, p, i, all.length)),
        h('tr', { class: 'implicit' }, h('td', { class: 'num' }, '—'), h('td', null, 'Default Deny'), h('td', null, 'any'), h('td', null, 'any'), h('td', null, 'any'),
          h('td', null, h('span', { class: 'action-drop' }, '✕ drop')), h('td', { colspan: 4, class: 'small' }, all.length ? 'implicit, logged (DM-09)' : 'implicit, logged (DM-09) — this RuleSet is empty on purpose'))))));
}

function unruledPairs() {
  // Pairs with no RuleSet: Default Deny, or the Zone's intraZoneTraffic (DM-39).
  const out = [];
  for (const z of objs('Zone')) {
    if (z.spec.interfaces.length < 2) continue;
    out.push(h('span', { class: 'pill', title: 'Traffic between two members with no RuleSet between them (DM-39)' },
      `${z.metadata.name}: intra-zone`, h('strong', null, z.spec.intraZoneTraffic || 'deny')));
  }
  return out;
}

export default {
  render(root) {
    const sets = [...objs('RuleSet')].sort((a, b) => a.spec.from.localeCompare(b.spec.from) || a.spec.to.localeCompare(b.spec.to));
    const search = h('input', { placeholder: 'Filter by name, object, service, NAT…', value: filter, style: { maxWidth: '300px' },
      oninput: (e) => { filter = e.target.value; const pos = e.target.selectionStart; window.songbirdRerender(); const s = root.querySelector('.page-head input'); s?.focus(); s?.setSelectionRange(pos, pos); } });
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Policies'), h('p', null, 'One section per RuleSet: the ordered rules for traffic entering from an Interface or Zone and leaving through another. The first match wins; unmatched traffic is dropped.')),
        h('span', { class: 'spacer' }), search,
        h('button', { onclick: () => { sets.forEach((z) => collapsed.add(z.metadata.name)); window.songbirdRerender(); } }, 'Collapse all'),
        isAdmin() && h('button', { onclick: () => yamlEditor('RuleSet', '', { isNew: true, template: `apiVersion: "config.songbird-fw.com/v1alpha1"\nkind: "RuleSet"\nmetadata:\n  name: ""\nspec:\n  from: ""\n  to: ""\n  rules: []\n` }) }, icon('code'), 'RuleSet from YAML'),
        isAdmin() && h('button', { class: 'primary', onclick: () => ruleSetForm() }, icon('plus'), 'RuleSet')),
      h('div', { class: 'row wrap', style: { marginBottom: '12px' } }, unruledPairs(), h('span', { class: 'muted small' }, 'Pairs without a RuleSet: Default Deny (R-033).')),
      h('div', { class: 'stack' }, sets.length ? sets.map(ruleSetCard) : h('div', { class: 'card empty' }, 'No RuleSets yet: all forwarded traffic is dropped.')));
  },
  refresh() { // update hit counters in place, keeping focus and scroll
    for (const td of document.querySelectorAll('[data-hit]')) td.textContent = fmtNum(state.counters?.hits?.[td.dataset.hit] || 0);
  },
};
