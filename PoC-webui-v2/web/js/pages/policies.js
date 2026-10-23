// Policies grouped by RuleSet (domain-model.md v0.7, ruleset.schema.json):
// one RuleSet per ordered (from, to) pair of Interfaces or Zones, holding an
// ordered list of rules. A member Interface may be used next to its Zone
// (DM-38): for a Flow the applicable RuleSets are evaluated from the most to
// the least specific and the first matching Policy decides (DM-42, R-014.4);
// unmatched traffic is dropped (Default Deny, R-033). NAT is a section of
// the rule (ADR-0010). Create, edit, delete, reorder (R-009), disable
// (R-009.2); required fields block save (R-009.1).
// A Policy row has no buttons: every action is in its context menu (right
// click, or the context-menu key / Shift+F10 on a focused row); double click
// opens the editor, drag and drop reorders.
import {
  state, h, icon, mount, objs, find, isAdmin, dot, refChip, save, modal, multiPicker, validationBox,
  confirmDialog, showYAML, yamlEditor, objectForm, newObject, deleteObject, api, fmtNum, colorHex,
  addrValue, svcValue, findingBadge, findingsFor, metaFields, contextMenu, colorStyle,
} from '../core.js';

const collapsed = new Set();
let filter = '';

// ---- the network side of a RuleSet ---------------------------------------------------------

const zoneOf = (iface) => objs('Zone').find((z) => z.spec.interfaces.includes(iface))?.metadata.name || '';
const side = (ref) => find('Zone', ref)?.spec.interfaces || (find('Interface', ref) ? [ref] : []);
const intraZone = (rs) => find('Interface', rs.spec.from) && find('Interface', rs.spec.to) && zoneOf(rs.spec.from) && zoneOf(rs.spec.from) === zoneOf(rs.spec.to);

// DM-42: the RuleSets that govern a Flow in -> out, most specific first.
function applicable(i, o) {
  if (i === o) return [];
  const zi = zoneOf(i), zo = zoneOf(o);
  const out = [];
  for (const [f, t] of [[i, o], [i, zo], [zi, o], [zi, zo]]) {
    if (!f || !t || f === t) continue;
    const rs = objs('RuleSet').find((x) => x.spec.from === f && x.spec.to === t);
    if (rs && !out.includes(rs)) out.push(rs);
  }
  return out;
}
// Interface pairs governed by more than one RuleSet: where the order matters.
function layeredPairs() {
  const ifs = objs('Interface').map((i) => i.metadata.name);
  const out = [];
  for (const i of ifs) for (const o of ifs) { const seq = applicable(i, o); if (seq.length > 1) out.push({ i, o, seq }); }
  return out;
}
// Where a RuleSet sits in the sequences it belongs to.
function precedence(rs) {
  const before = new Set(), after = new Set();
  for (const { seq } of layeredPairs()) {
    const k = seq.indexOf(rs);
    if (k < 0) continue;
    seq.slice(0, k).forEach((x) => before.add(x.metadata.name));
    seq.slice(k + 1).forEach((x) => after.add(x.metadata.name));
  }
  return { before: [...before], after: [...after] };
}

function sideChip(ref) {
  const z = find('Zone', ref);
  const title = z ? `Zone: ${z.spec.interfaces.join(', ')}` : find('Interface', ref) ? `Interface ${ref}` : 'missing';
  return h('span', { class: ['chip', z && 'group'], title }, ref);
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
  for (const a of objs('Address')) if (a.spec.predefined) out.push({ value: a.metadata.name, label: `${a.metadata.name}  ·  ${addrValue(a.spec)}`, group: 'Predefined' });
  for (const a of objs('Address')) if (!a.spec.predefined && fits(a.metadata.name, ifaces)) out.push({ value: a.metadata.name, label: `${a.metadata.name}  ·  ${addrValue(a.spec)}${a.spec.fqdn || a.spec.mac ? '  (not in enabled rules yet, DM-43)' : ''}`, group: 'Addresses' });
  for (const g of objs('AddressGroup')) if (fits(g.metadata.name, ifaces)) out.push({ value: g.metadata.name, label: g.metadata.name, group: 'Address groups' });
  return out;
}
function serviceOptions() {
  return [
    ...objs('Service').filter((s) => s.spec.predefined).map((s) => ({ value: s.metadata.name, label: `${s.metadata.name}  ·  every protocol and port`, group: 'Predefined' })),
    ...objs('Service').filter((s) => !s.spec.predefined).map((s) => ({ value: s.metadata.name, label: `${s.metadata.name}  ·  ${svcValue(s.spec)}`, group: 'Services' })),
    ...objs('ServiceGroup').map((s) => ({ value: s.metadata.name, label: s.metadata.name, group: 'Service groups' })),
  ];
}

async function saveRuleSet(rs) {
  return save('PUT', `/api/v1/objects/RuleSet/${encodeURIComponent(rs.metadata.name)}`, rs);
}

// ---- rule editor ---------------------------------------------------------------------------------

function editRule(rsName, index, at) {
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

  const pos = isNew ? (at ?? rs.spec.rules.length) : index;
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
    if (isNew) z.spec.rules.splice(pos, 0, r); else z.spec.rules[index] = r;
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
    const ns = (key) => (key === 'service' ? 'service' : 'address');
    const picker = (key, label, options, help) => field(key, label, ro ? h('div', { class: 'chips' }, p[key].map((s) => refChip(s, null, ns(key)))) : multiPicker(p[key], options, changed, `select ${key}…`, ns(key)), true, help, true);
    mount(pickers,
      picker('source', `Source (enters from ${rs.spec.from})`, addressOptions(fromIfs), `"any" and "any6" are predefined Addresses. Objects bound to an Interface are listed only if it is in ${rs.spec.from} (DM-12).`),
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
    h('div', { class: 'row sub wrap' }, 'RuleSet', h('strong', null, rs.metadata.name), h('span', { class: 'muted' }, '·'), sideChip(rs.spec.from), icon('arrow'), sideChip(rs.spec.to),
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
        'Still in the schema; domain model 0.7 removes it (every Flow is an Event, R-026).'),
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

// duplicate inserts a disabled copy right below, with a free name.
async function duplicate(rsName, i) {
  const rs = structuredClone(find('RuleSet', rsName));
  const p = structuredClone(rs.spec.rules[i]);
  delete p.uuid;
  const names = new Set(rs.spec.rules.map((r) => r.name));
  let n = 2;
  while (names.has(`${p.name}-${n}`)) n++;
  p.name = `${p.name}-${n}`;
  p.enabled = false;
  rs.spec.rules.splice(i + 1, 0, p);
  await saveRuleSet(rs);
}

// The context menu of a Policy row: every action of the row lives here.
function ruleMenu(ev, rs, i, tr) {
  ev.preventDefault();
  const name = rs.metadata.name, p = rs.spec.rules[i], n = rs.spec.rules.length, admin = isAdmin();
  const at = ev.type === 'contextmenu' && ev.clientX ? { x: ev.clientX, y: ev.clientY } : (() => { const r = tr.getBoundingClientRect(); return { x: r.left + 48, y: r.bottom }; })();
  tr.classList.add('ctx-open');
  contextMenu(at, `${i + 1} · ${p.name}`, [
    { label: admin ? 'Edit…' : 'View…', icon: 'edit', kbd: 'Enter', onClick: () => editRule(name, i) },
    admin && { label: p.enabled === false ? 'Enable' : 'Disable', icon: 'power', onClick: () => toggle(name, i) },
    admin && { label: 'Duplicate below', icon: 'copy', onClick: () => duplicate(name, i) },
    admin && '-',
    admin && { label: 'Move to top', icon: 'top', disabled: i === 0, onClick: () => move(name, i, 0) },
    admin && { label: 'Move up', icon: 'up', kbd: 'Alt ↑', disabled: i === 0, onClick: () => move(name, i, i - 1) },
    admin && { label: 'Move down', icon: 'down', kbd: 'Alt ↓', disabled: i === n - 1, onClick: () => move(name, i, i + 1) },
    admin && { label: 'Move to bottom', icon: 'bottom', disabled: i === n - 1, onClick: () => move(name, i, n - 1) },
    admin && '-',
    admin && { label: 'New Policy above', icon: 'insert', onClick: () => editRule(name, null, i) },
    { label: 'RuleSet YAML', icon: 'code', onClick: () => showYAML('RuleSet', name) },
    admin && '-',
    admin && { label: 'Delete…', icon: 'trash', kbd: 'Del', danger: true, onClick: () => removeRule(name, i) },
  ], () => tr.classList.remove('ctx-open'));
}

// From / To: Zones and Interfaces. A member Interface may be used next to
// its Zone (DM-38): its RuleSet is evaluated before the Zone's (DM-42).
function sideOptions() {
  const out = [{ value: '', label: '— select —' }];
  for (const z of objs('Zone')) out.push({ value: z.metadata.name, label: `${z.metadata.name} (Zone: ${z.spec.interfaces.join(', ')})` });
  for (const i of objs('Interface')) {
    const z = zoneOf(i.metadata.name);
    out.push({ value: i.metadata.name, label: z ? `${i.metadata.name} (member of ${z}: evaluated before it)` : `${i.metadata.name} (Interface)` });
  }
  return out;
}

function ruleSetForm(rs) {
  const isNew = !rs;
  objectForm({
    kind: 'RuleSet', isNew, obj: rs || newObject('RuleSet', { from: '', to: '', rules: [] }),
    intro: isNew && h('p', { class: 'sub small', style: { margin: 0 } }, 'One RuleSet per ordered pair; there is no wildcard Interface. A member Interface may be used next to its Zone: for its traffic the more specific RuleSet is evaluated first, then the Zone\'s (DM-38, DM-42).'),
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
  if (p.nat.type === 'snat') return h('span', { title: 'Source NAT' }, 'snat ', p.nat.translateIpTo ? refChip(p.nat.translateIpTo, null, 'address') : h('span', { class: 'muted' }, 'masquerade'));
  return h('span', { class: 'row wrap', style: { gap: '4px' }, title: 'Destination NAT, matched before translation' }, 'dnat →', refChip(p.nat.translateIpTo, null, 'address'),
    p.nat.translateServiceTo && refChip(p.nat.translateServiceTo, null, 'service'));
}

function ruleRow(rs, p, i, n) {
  const key = `${rs.metadata.name}/${p.name}`;
  const hits = state.counters?.hits?.[key] || 0;
  const admin = isAdmin();
  const name = rs.metadata.name;
  const errs = findingsFor('RuleSet', name).filter((e) => e.path === `spec.rules[${i}]` || e.path.startsWith(`spec.rules[${i}].`));
  const tr = h('tr', { class: [p.enabled === false && 'disabled'], tabindex: '0', draggable: admin ? 'true' : null, 'aria-label': `Policy ${i + 1} ${p.name}`,
    title: 'Right click for actions · double click to edit',
    ondragstart: (e) => { e.dataTransfer.setData('text/plain', JSON.stringify({ rs: name, i })); e.dataTransfer.effectAllowed = 'move'; },
    ondragover: (e) => { e.preventDefault(); tr.classList.add('drag-over'); },
    ondragleave: () => tr.classList.remove('drag-over'),
    ondrop: (e) => { e.preventDefault(); tr.classList.remove('drag-over'); try { const d = JSON.parse(e.dataTransfer.getData('text/plain')); if (d.rs === name) move(d.rs, d.i, i); } catch { /* not ours */ } },
    ondblclick: () => editRule(name, i),
    oncontextmenu: (e) => ruleMenu(e, rs, i, tr),
    // touch screens have no right click: a tap opens the same menu
    onclick: (e) => { if (matchMedia('(pointer: coarse)').matches && !e.target.closest('button, a')) ruleMenu(Object.assign(e, { preventDefault() {} }), rs, i, tr); },
    onkeydown: (e) => {
      if (e.target !== tr) return;
      if (e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')) ruleMenu(e, rs, i, tr);
      else if (e.key === 'Enter') { e.preventDefault(); editRule(name, i); }
      else if (admin && e.altKey && e.key === 'ArrowUp' && i > 0) { e.preventDefault(); move(name, i, i - 1); }
      else if (admin && e.altKey && e.key === 'ArrowDown' && i < n - 1) { e.preventDefault(); move(name, i, i + 1); }
      else if (admin && e.key === 'Delete') { e.preventDefault(); removeRule(name, i); }
    } },
  h('td', { class: 'num' }, h('span', { class: 'row', style: { justifyContent: 'flex-end' } }, admin && h('span', { class: 'drag', 'aria-hidden': 'true' }, icon('grip')), h('span', { class: 'pos' }, String(i + 1)))),
  h('td', null, h('div', { class: 'row wrap' }, h('span', { style: { fontWeight: 500 } }, p.name), p.enabled === false && h('span', { class: 'badge' }, 'disabled'),
    errs.length ? h('span', { class: ['badge', errs.some((e) => e.severity === 'error') ? 'bad' : 'warn'], title: errs.map((e) => e.reason).join('\n') }, errs.some((e) => e.severity === 'error') ? 'error' : 'warning') : null),
  (p.comment || p.log) && h('div', { class: 'small muted' }, [p.comment, p.log && `log ${p.log}`].filter(Boolean).join(' · '))),
  h('td', null, h('div', { class: 'row wrap', style: { gap: '4px' } }, p.source.map((s) => refChip(s, null, 'address')))),
  h('td', null, h('div', { class: 'row wrap', style: { gap: '4px' } }, p.destination.map((s) => refChip(s, null, 'address')))),
  h('td', null, h('div', { class: 'row wrap', style: { gap: '4px' } }, p.service.map((s) => refChip(s, null, 'service')))),
  h('td', null, h('span', { class: `action-${p.action}` }, p.action)),
  h('td', { class: 'small' }, natCell(p)),
  h('td', { class: 'num', 'data-hit': key, title: 'Flows matched since start (simulated Data Plane)' }, fmtNum(hits)));
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
  const prec = precedence(rs);
  const headMenu = (e) => {
    e.preventDefault();
    contextMenu({ x: e.clientX, y: e.clientY }, `RuleSet ${name}`, [
      isAdmin() && { label: 'New Policy', icon: 'plus', onClick: () => editRule(name) },
      isAdmin() && { label: 'Edit RuleSet…', icon: 'edit', onClick: () => ruleSetForm(rs) },
      { label: 'Show YAML', icon: 'code', onClick: () => showYAML('RuleSet', name) },
      isAdmin() && { label: 'Edit YAML…', icon: 'code', onClick: () => yamlEditor('RuleSet', name) },
      isAdmin() && '-',
      isAdmin() && { label: 'Delete RuleSet…', icon: 'trash', danger: true, onClick: () => deleteObject('RuleSet', name) },
    ]);
  };
  const nearLimit = enabled > 0.8 * limit;
  const rest = prec.after.length ? `Anything else continues in ${prec.after.join(', ')}` : 'Anything else is dropped and logged';
  return h('section', { class: ['zone', isCol && 'collapsed'] },
    h('div', { class: 'head', oncontextmenu: headMenu, title: 'Click to fold · right click for RuleSet actions', onclick: (e) => { if (e.target.closest('button')) return; isCol ? collapsed.delete(name) : collapsed.add(name); window.songbirdRerender(); } },
      icon('chev', 'chev'),
      h('h2', { title: rs.metadata.comment || null }, name),
      h('span', { class: 'flow', title: `Interface pairs: ${pairs.join(', ')}` }, sideChip(rs.spec.from), icon('arrow'), sideChip(rs.spec.to)),
      prec.before.length > 0 && h('span', { class: 'small muted', title: 'DM-42' }, `after ${prec.before.join(', ')}`),
      prec.after.length > 0 && h('span', { class: 'small muted', title: 'DM-42' }, `before ${prec.after.join(', ')}`),
      nearLimit && h('span', { class: 'badge warn', title: 'Enabled Policies / per-RuleSet limit (DM-36, ADR-0006)' }, `${enabled} / ${fmtNum(limit)}`),
      findingBadge('RuleSet', name),
      h('span', { class: 'spacer' }),
      isAdmin() && h('button', { class: 'ghost sm', onclick: () => editRule(name) }, icon('plus'), 'Policy')),
    !isCol && h('div', { class: 'table-wrap' }, h('table', { class: 'policies' },
      h('colgroup', null, ['56px', '18%', '17%', '17%', '15%', '92px', '17%', '64px'].map((w) => h('col', { style: { width: w } }))),
      all.length > 0 && h('thead', null, h('tr', null, ['#', 'Name', 'Source', 'Destination', 'Service', 'Action', 'NAT', 'Hits'].map((t, k) => h('th', { class: [0, 7].includes(k) && 'num' }, t)))),
      h('tbody', null,
        shown.map(([p, i]) => ruleRow(rs, p, i, all.length)),
        h('tr', { class: 'implicit' }, h('td'), h('td', { colspan: 7 }, rest))))));
}

// R-046.3: for every pair governed by several RuleSets, show the order.
function evaluationOrder() {
  const pairs = layeredPairs();
  if (!pairs.length) return null;
  return h('div', { class: 'eval-order', title: 'R-046.3, DM-42' }, pairs.map(({ i, o, seq }) => {
    const z = zoneOf(i) && zoneOf(i) === zoneOf(o) ? zoneOf(i) : '';
    const last = z ? `intraZoneTraffic ${find('Zone', z)?.spec.intraZoneTraffic || 'deny'}` : 'Default Deny';
    return h('p', null, h('strong', null, `${i} → ${o}`), ' is checked against ',
      seq.map((rs, k) => [k > 0 && (k === seq.length - 1 ? ', then ' : ', '), h('span', null, rs.metadata.name)]),
      `, then ${last}.`);
  }));
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
        h('div', null, h('h1', null, 'Policies'), h('p', null, 'Rules are grouped by the Interfaces or Zones the traffic crosses. The first matching rule decides; anything unmatched is dropped. Right click a rule for its actions.')),
        search,
        h('button', { class: 'ghost', title: 'More', 'aria-label': 'More actions', onclick: (e) => { const r = e.currentTarget.getBoundingClientRect(); contextMenu({ x: r.right - 230, y: r.bottom + 4 }, null, [
          { label: 'Collapse all', icon: 'chev', onClick: () => { sets.forEach((z) => collapsed.add(z.metadata.name)); window.songbirdRerender(); } },
          { label: 'Expand all', icon: 'chev', onClick: () => { collapsed.clear(); window.songbirdRerender(); } },
          isAdmin() && { label: 'New RuleSet from YAML…', icon: 'code', onClick: () => yamlEditor('RuleSet', '', { isNew: true, template: `apiVersion: "config.songbird-fw.com/v1alpha1"\nkind: "RuleSet"\nmetadata:\n  name: ""\nspec:\n  from: ""\n  to: ""\n  rules: []\n` }) },
        ]); } }, icon('more')),
        isAdmin() && h('button', { class: 'primary', onclick: () => ruleSetForm() }, icon('plus'), 'RuleSet')),
      evaluationOrder(),
      h('div', { class: 'zones' }, sets.length ? sets.map(ruleSetCard) : h('div', { class: 'empty' }, 'No RuleSets yet: all forwarded traffic is dropped.')));
  },
  refresh() { // update hit counters in place, keeping focus and scroll
    for (const td of document.querySelectorAll('[data-hit]')) td.textContent = fmtNum(state.counters?.hits?.[td.dataset.hit] || 0);
  },
};
