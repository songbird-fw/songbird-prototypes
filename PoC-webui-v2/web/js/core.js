// Core helpers: DOM builder, API client, shared state, modals, toasts, forms.
// The UI holds no configuration of its own: it reads the tree from the API
// and every save goes back as a whole object with If-Match (components.md).
// Objects are v1alpha1 documents (songbird-fw/schema), unchanged.

export const API_VERSION = 'config.songbird-fw.com/v1alpha1';

export const state = {
  me: null,          // { u, r }
  config: null,      // { revision, objects, findings, kinds, predefined, activeRoutes, limits }
  status: null,      // ReconcilerStatus
  counters: null,    // Event Collector counters
  listeners: new Set(),
};

export const isAdmin = () => state.me && state.me.r === 'admin';
export const objs = (kind) => (state.config?.objects?.[kind]) || [];
export const find = (kind, name) => objs(kind).find((o) => o.metadata.name === name);
export const sys = () => objs('System')[0]?.spec || {};

export function emit() { for (const fn of state.listeners) fn(); }

// ---- DOM ------------------------------------------------------------------

export function h(tag, attrs, ...children) {
  const el = tag === 'svg' || attrs?.svg ? document.createElementNS('http://www.w3.org/2000/svg', tag) : document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false || k === 'svg') continue;
    if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
    else if (k === 'style' && typeof v === 'object') for (const [sk, sv] of Object.entries(v)) { if (sk.startsWith('--')) el.style.setProperty(sk, sv); else el.style[sk] = sv; }
    else if (k === 'class') el.setAttribute('class', Array.isArray(v) ? v.filter(Boolean).join(' ') : v);
    else if (k === 'value' && 'value' in el && tag !== 'option') el.value = v;
    else if (k === 'checked' || k === 'selected' || k === 'disabled') el[k] = !!v;
    else el.setAttribute(k, v === true ? '' : v);
  }
  append(el, children);
  return el;
}

function append(el, children) {
  for (const c of children.flat(Infinity)) {
    if (c == null || c === false || c === true) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
}

export function mount(root, ...children) { root.replaceChildren(); append(root, children); }

const ICONS = {
  shield: 'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z',
  gauge: 'M12 14l4-4 M3.3 19a10 10 0 1 1 17.4 0',
  network: 'M9 2h6v6H9z M2 16h6v6H2z M16 16h6v6h-6z M12 8v4 M5 16v-4h14v4',
  box: 'M21 8l-9-5-9 5 9 5 9-5z M3 8v8l9 5 9-5V8 M12 13v8',
  activity: 'M22 12h-4l-3 9L9 3l-3 9H2',
  history: 'M3 3v5h5 M3.05 13A9 9 0 1 0 6 5.3L3 8 M12 7v5l4 2',
  heart: 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8z',
  file: 'M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z M14 2v6h6 M9 13h6 M9 17h6',
  users: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2 M9 7a4 4 0 1 0 0 .01 M23 21v-2a4 4 0 0 0-3-3.9 M16 3.1a4 4 0 0 1 0 7.8',
  plus: 'M12 5v14 M5 12h14',
  edit: 'M12 20h9 M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z',
  trash: 'M3 6h18 M8 6V4h8v2 M19 6l-1 14H6L5 6',
  up: 'M12 19V5 M5 12l7-7 7 7',
  down: 'M12 5v14 M19 12l-7 7-7-7',
  chev: 'M6 9l6 6 6-6',
  code: 'M16 18l6-6-6-6 M8 6l-6 6 6 6',
  lock: 'M5 11h14v10H5z M8 11V7a4 4 0 0 1 8 0v4',
  sun: 'M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10z M12 1v2 M12 21v2 M4.2 4.2l1.4 1.4 M18.4 18.4l1.4 1.4 M1 12h2 M21 12h2 M4.2 19.8l1.4-1.4 M18.4 5.6l1.4-1.4',
  moon: 'M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8z',
  check: 'M20 6L9 17l-5-5',
  x: 'M18 6L6 18 M6 6l12 12',
  alert: 'M10.3 3.9L1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z M12 9v4 M12 17h.01',
  grip: 'M9 5h.01 M9 12h.01 M9 19h.01 M15 5h.01 M15 12h.01 M15 19h.01',
  arrow: 'M5 12h14 M13 6l6 6-6 6',
  logout: 'M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4 M16 17l5-5-5-5 M21 12H9',
  pause: 'M6 4h4v16H6z M14 4h4v16h-4z',
  play: 'M5 3l14 9-14 9z',
  undo: 'M3 7v6h6 M21 17a9 9 0 0 0-15-6.7L3 13',
  power: 'M18.4 6.6a9 9 0 1 1-12.8 0 M12 2v10',
  zap: 'M13 2L3 14h9l-1 8 10-12h-9z',
  cog: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z',
  download: 'M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M7 10l5 5 5-5 M12 15V3',
  upload: 'M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4 M17 8l-5-5-5 5 M12 3v12',
  copy: 'M9 9h11v11H9z M5 15H4V4h11v1',
  more: 'M5 12h.01 M12 12h.01 M19 12h.01',
  // object kinds, shown before names: one circle = one Address, three = a group;
  // one square = one Service, two = a group
  address: 'M5.5 12a6.5 6.5 0 1 0 13 0a6.5 6.5 0 1 0 -13 0',
  addressGroup: 'M2.5 7.5a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0 M14.5 7.5a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0 M8.5 16.5a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0',
  service: 'M5.5 5.5h13v13h-13z',
  serviceGroup: 'M3 3h11v11H3z M10 10h11v11H10z',
  top: 'M5 4h14 M12 20V8 M6 14l6-6 6 6',
  bottom: 'M5 20h14 M12 4v12 M6 10l6 6 6-6',
  insert: 'M12 5v14 M5 12h14 M3 3h4 M17 3h4',
};

export function icon(name, cls) {
  const svg = h('svg', { class: ['icon', cls], viewBox: '0 0 24 24', 'aria-hidden': 'true' });
  for (const d of ICONS[name].split(' M').map((p, i) => (i ? 'M' + p : p))) {
    svg.append(h('path', { svg: true, d }));
  }
  return svg;
}

// ---- formatting --------------------------------------------------------------

export const short = (id) => (id || '').slice(0, 8);
export function fmtTime(t) {
  const d = new Date(t);
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' });
}
export function ago(t) {
  const s = Math.round((Date.now() - new Date(t).getTime()) / 1000);
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  if (s < 86400) return `${Math.round(s / 3600)} h ago`;
  return `${Math.round(s / 86400)} d ago`;
}
export function fmtNum(n) { return new Intl.NumberFormat().format(n || 0); }
export function fmtBytes(n) {
  const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0;
  while (n >= 1000 && i < u.length - 1) { n /= 1000; i++; }
  return `${n.toFixed(n < 10 && i ? 1 : 0)} ${u[i]}`;
}
export function fmtBps(n) {
  const u = ['bit/s', 'kbit/s', 'Mbit/s', 'Gbit/s']; let i = 0; n *= 8;
  while (n >= 1000 && i < u.length - 1) { n /= 1000; i++; }
  return `${n.toFixed(n < 10 && i ? 1 : 0)} ${u[i]}`;
}

// Address and Service values as the schema writes them.
export function addrValue(spec) {
  if (!spec) return '';
  if (spec.subnet) return spec.subnet;
  if (spec.range) return `${spec.range.start}-${spec.range.end}`;
  return spec.fqdn || spec.mac || '';
}
export function addrForm(spec) {
  return spec?.subnet ? 'subnet' : spec?.range ? 'range' : spec?.fqdn ? 'fqdn' : spec?.mac ? 'mac' : 'subnet';
}
export function protoEntry(p) {
  if (p.port != null) return `${p.protocol}/${p.port}`;
  if (p.icmpType != null) return `${p.protocol} type ${p.icmpType}${p.icmpCode != null ? ' code ' + p.icmpCode : ''}`;
  if (p.protocolNumber != null) return `ip proto ${p.protocolNumber}`;
  return p.protocol;
}
export const svcValue = (spec) => (spec?.protocols || []).map(protoEntry).join(', ');

// ---- colours (R-055) -----------------------------------------------------------
// metadata.color references a Color by name; what it stands for is the
// Color's comment.

export function colorHex(name) { return name ? find('Color', name)?.spec.hex || null : null; }
// Objects are coloured like in FortiGate: the name itself takes the colour
// (tint), with a faint wash behind chips. colorStyle feeds the CSS variable.
export function colorStyle(colorName) {
  const v = colorHex(colorName);
  return v ? { '--c': v } : null;
}
export function dot(colorName, title) {
  const v = colorHex(colorName);
  if (!v) return null;
  const c = find('Color', colorName);
  return h('span', { class: 'dot', style: { background: v }, title: title || `${colorName}${c?.metadata.comment ? ' — ' + c.metadata.comment : ''}` });
}
export function colorOptions() {
  return [{ value: '', label: '— none —' }, ...objs('Color').map((c) => ({ value: c.metadata.name, label: `${c.metadata.name}${c.metadata.comment ? ' — ' + c.metadata.comment : ''}` }))];
}

// Display a reference (Address, group, Service, Interface, Zone) as a chip.
// "any" / "any6" are predefined objects, not a keyword (DM-21): shown plain.
// kind narrows the lookup where names repeat across namespaces (the
// Address any and the Service any).
export function refChip(name, onRemove, kind) {
  let cls = 'chip', color = null, title = name, found = null;
  {
    const kinds = kind === 'service' ? ['Service', 'ServiceGroup'] : kind === 'address' ? ['Address', 'AddressGroup'] : ['Address', 'AddressGroup', 'Service', 'ServiceGroup', 'Interface', 'Zone'];
    const o = kinds.map((k) => find(k, name)).find(Boolean);
    found = o;
    if (o) {
      // Interfaces and Zones are not coloured: the colour classifies objects only
      if (o.kind !== 'Interface' && o.kind !== 'Zone') color = o.metadata.color;
      if (o.spec.predefined) cls += ' any';
      if (o.kind === 'Address') title = `${addrForm(o.spec)} ${addrValue(o.spec)}${o.spec.interface ? ' · bound to ' + o.spec.interface : ''}${o.spec.predefined ? ' · predefined' : ''}`;
      else if (o.kind === 'AddressGroup' || o.kind === 'ServiceGroup') { title = `group: ${o.spec.members.join(', ')}`; cls += ' group'; }
      else if (o.kind === 'Service') title = svcValue(o.spec);
      else if (o.kind === 'Zone') { title = `Zone: ${o.spec.interfaces.join(', ')}`; cls += ' group'; }
      if (o.metadata.comment) title += `\n${o.metadata.comment}`;
    } else { title = 'missing object'; cls += ' missing'; }
  }
  const tint = colorStyle(color);
  const kindIcon = { Address: 'address', AddressGroup: 'addressGroup', Service: 'service', ServiceGroup: 'serviceGroup' }[found?.kind];
  return h('span', { class: [cls, tint && 'tinted'], style: tint, title: color ? `${title}\ncolor: ${color}` : title }, kindIcon && icon(kindIcon, 'kind'), name,
    onRemove && h('button', { type: 'button', title: 'Remove', 'aria-label': `Remove ${name}`, onclick: onRemove }, '×'));
}

// ---- findings of the Validator ---------------------------------------------------

export const findings = () => state.config?.findings || [];
export function filePath(kind, name) {
  const k = (state.config?.kinds || []).find((x) => x.name === kind);
  return k && k.dir === '' ? `${name}.yaml` : `${k?.dir || kind}/${name}.yaml`;
}
export function findingsFor(kind, name) { const f = filePath(kind, name); return findings().filter((e) => e.file === f); }

export function findingBadge(kind, name) {
  const l = findingsFor(kind, name);
  if (!l.length) return null;
  const errs = l.filter((e) => e.severity === 'error').length;
  return h('span', { class: ['badge', errs ? 'bad' : 'warn'], title: l.map((e) => `${e.path}: ${e.reason}`).join('\n') }, errs ? `${errs} error${errs > 1 ? 's' : ''}` : `${l.length} warning${l.length > 1 ? 's' : ''}`);
}

// ---- API ---------------------------------------------------------------------------

export class ApiError extends Error {
  constructor(status, data) { super(data?.error || `HTTP ${status}`); this.status = status; this.data = data || {}; }
}

export async function api(method, path, body, opts = {}) {
  const headers = { 'X-Requested-With': 'songbird' };
  let payload;
  if (body !== undefined) {
    if (opts.yaml) { headers['Content-Type'] = 'application/yaml'; payload = body; }
    else { headers['Content-Type'] = 'application/json'; payload = JSON.stringify(body); }
  }
  if (opts.rev) headers['If-Match'] = `"${opts.rev}"`;
  const res = await fetch(path, { method, headers, body: payload, credentials: 'same-origin' });
  const ct = res.headers.get('Content-Type') || '';
  const data = ct.includes('json') ? await res.json() : await res.text();
  if (res.status === 401 && !path.endsWith('/login')) { state.me = null; location.hash = '#/login'; }
  if (!res.ok) throw new ApiError(res.status, typeof data === 'string' ? { error: data } : data);
  return data;
}

export async function loadConfig() {
  state.config = await api('GET', '/api/v1/config');
  emit();
}

// save sends a write with the revision the user edited and reports the
// outcome of the reconciliation: "saved and applied" or "saved, not applied".
export async function save(method, path, body, opts = {}) {
  try {
    const r = await api(method, path, body, { ...opts, rev: state.config.revision });
    const warn = r.warnings?.length ? ` · ${r.warnings.length} warning${r.warnings.length > 1 ? 's' : ''}: ${r.warnings[0].reason}` : '';
    if (r.applied) toast('Saved and applied', `commit ${short(r.commit)}${warn}`, r.warnings?.length ? 'warn' : 'good');
    else if (r.pending) toast('Saved, apply in progress', `commit ${short(r.commit)}${warn}`, 'warn');
    else toast('Saved, not applied', r.attempt?.error?.message || r.attempt?.result, 'bad', '#/diagnostics');
    await loadConfig();
    return { ok: true, result: r };
  } catch (e) {
    if (!(e instanceof ApiError)) throw e;
    if (e.status === 409 && e.data.conflict) {
      toast('Conflict', `${e.data.conflict.file} was changed by someone else since you opened it. Close the form and edit again on the current revision.`, 'bad');
      await loadConfig();
    } else if (e.status !== 422) {
      toast('Not saved', e.message, 'bad');
    }
    return { ok: false, error: e };
  }
}

// ---- toasts & modals ------------------------------------------------------------------------

export function toast(title, text, kind = '', href) {
  let box = document.querySelector('.toasts');
  if (!box) { box = h('div', { class: 'toasts', role: 'status', 'aria-live': 'polite' }); document.body.append(box); }
  const t = h('div', { class: ['toast', kind] }, h('div', { style: { fontWeight: 600 } }, title), text && h('div', { class: 'sub' }, text),
    href && h('a', { href, class: 'small' }, 'Details'));
  box.append(t);
  setTimeout(() => t.remove(), kind === 'bad' ? 9000 : 5000);
}

export function modal({ title, body, foot, narrow, wide, onClose }) {
  const close = () => { ov.remove(); document.removeEventListener('keydown', esc); onClose && onClose(); };
  const esc = (e) => { if (e.key === 'Escape') close(); };
  const ov = h('div', { class: 'overlay', onmousedown: (e) => { if (e.target === ov) close(); } },
    h('div', { class: ['modal', narrow && 'narrow', wide && 'wide'], role: 'dialog', 'aria-modal': 'true', 'aria-label': title },
      h('div', { class: 'head' }, h('h2', null, title), h('span', { class: 'spacer' }), h('button', { class: 'ghost', onclick: close, 'aria-label': 'Close' }, icon('x'))),
      h('div', { class: 'body' }, body),
      foot && h('div', { class: 'foot' }, foot(close))));
  document.body.append(ov);
  document.addEventListener('keydown', esc);
  setTimeout(() => ov.querySelector('input:not([disabled]),select,textarea')?.focus(), 30);
  return close;
}

export function confirmDialog(title, text, okLabel = 'Confirm', danger = false) {
  return new Promise((resolve) => {
    let done = false;
    modal({
      title, narrow: true, body: h('p', { style: { margin: 0 } }, text),
      onClose: () => { if (!done) resolve(false); },
      foot: (c) => [h('span', { class: 'spacer' }), h('button', { onclick: c }, 'Cancel'),
        h('button', { class: danger ? 'primary danger-bg' : 'primary', onclick: () => { done = true; resolve(true); c(); } }, okLabel)],
    });
  });
}

export function showYAML(kind, name, rev) {
  const pre = h('pre', { class: 'code' }, 'Loading…');
  modal({ title: `${kind}/${name}`, body: [h('div', { class: 'sub small' }, 'The file in the configuration repository — the single source of truth (ADR-0001), in the canonical form of the schema (every string double-quoted).'), pre] });
  api('GET', `/api/v1/objects/${kind}/${encodeURIComponent(name)}/yaml${rev ? '?rev=' + rev : ''}`)
    .then((t) => { pre.textContent = `# ${filePath(kind, name)}\n` + t; })
    .catch((e) => { pre.textContent = e.message; });
}

function where(e) { return `${e.file}${e.line ? ':' + e.line : ''}`; }

export function validationBox(errs, msg, warnings) {
  const out = [];
  if (errs?.length || msg) {
    out.push(h('div', { class: 'verr', role: 'alert' }, h('strong', null, msg || 'The change is not valid'),
      errs?.length ? h('ul', null, errs.map((e) => h('li', null, h('code', null, where(e)), ' ', e.path && h('code', null, e.path), ' — ', e.reason,
        h('span', { class: 'muted' }, ` (${[e.rule, e.constraint].filter(Boolean).join(', ')})`)))) : null));
  }
  if (warnings?.length) {
    out.push(h('div', { class: 'vwarn' }, h('strong', null, `${warnings.length} warning${warnings.length > 1 ? 's' : ''} (saving is allowed)`),
      h('ul', null, warnings.map((e) => h('li', null, e.path && h('code', null, e.path), ' — ', e.reason, h('span', { class: 'muted' }, ` (${[e.rule, e.constraint].filter(Boolean).join(', ')})`))))));
  }
  return out.length ? out : null;
}

// ---- context menu ---------------------------------------------------------------------
// items: [{ label, icon, kbd, danger, disabled, onClick } | '-' ]. Opens at the
// pointer (or under an element for keyboard use); Escape, a click outside,
// scrolling or a resize close it. Arrow keys move between items.

let ctxClose = null;
export function contextMenu(at, title, items, onClose) {
  if (ctxClose) ctxClose();
  const menu = h('div', { class: 'ctx', role: 'menu', 'aria-label': title },
    title && h('div', { class: 'ctx-title' }, title),
    items.filter(Boolean).map((it) => (it === '-' ? h('hr') : h('button', { role: 'menuitem', class: it.danger && 'danger', disabled: it.disabled, title: it.title,
      onclick: () => { close(); it.onClick(); } }, it.icon && icon(it.icon), it.label, it.kbd && h('span', { class: 'kbd' }, it.kbd)))));
  const close = () => {
    menu.remove(); ctxClose = null;
    document.removeEventListener('mousedown', outside, true); document.removeEventListener('keydown', key, true);
    window.removeEventListener('scroll', close, true); window.removeEventListener('resize', close);
    onClose && onClose();
  };
  const outside = (e) => { if (!menu.contains(e.target)) close(); };
  const key = (e) => {
    const btns = [...menu.querySelectorAll('button:not(:disabled)')];
    const i = btns.indexOf(document.activeElement);
    if (e.key === 'Escape') { e.preventDefault(); close(); } else if (e.key === 'ArrowDown') { e.preventDefault(); btns[(i + 1) % btns.length]?.focus(); } else if (e.key === 'ArrowUp') { e.preventDefault(); btns[(i - 1 + btns.length) % btns.length]?.focus(); }
  };
  document.body.append(menu);
  let x = at.x, y = at.y;
  const r = menu.getBoundingClientRect();
  if (x + r.width > innerWidth - 8) x = Math.max(8, innerWidth - r.width - 8);
  if (y + r.height > innerHeight - 8) y = Math.max(8, y - r.height);
  menu.style.left = `${x}px`; menu.style.top = `${y}px`;
  setTimeout(() => {
    document.addEventListener('mousedown', outside, true); document.addEventListener('keydown', key, true);
    window.addEventListener('scroll', close, true); window.addEventListener('resize', close);
    menu.querySelector('button:not(:disabled)')?.focus({ preventScroll: true });
  }, 0);
  ctxClose = close;
  return close;
}

// ---- YAML editor -------------------------------------------------------------------------------
// Edits the file itself. The server checks it against the schema and the
// Validator on every keystroke (dry run), and reports file:line and path.

export function yamlEditor(kind, name, { isNew = false, template = '' } = {}) {
  const ta = h('textarea', { class: 'yaml-editor', spellcheck: 'false', disabled: !isAdmin(), 'aria-label': 'YAML' }, isNew ? template : 'Loading…');
  const errBox = h('div');
  const saveBtn = h('button', { class: 'primary', disabled: true, onclick: () => submit() }, 'Save');
  let t;
  const nameOf = () => (ta.value.match(/^\s*name:\s*"?([a-z0-9._-]+)"?/m) || [])[1] || name;
  async function dry() {
    if (!isAdmin()) return;
    const n = nameOf();
    try {
      const r = await api('POST', `/api/v1/validate/${kind}/${encodeURIComponent(n)}${isNew ? '?create=1' : ''}`, ta.value, { yaml: true });
      mount(errBox, r.valid ? validationBox(null, null, r.warnings) || h('div', { class: 'vok' }, icon('check'), 'Valid against the schema and the Validator') : validationBox(r.validation, r.error, r.warnings));
      saveBtn.disabled = !r.valid;
    } catch (e) { mount(errBox, validationBox(null, e.message)); saveBtn.disabled = true; }
  }
  ta.addEventListener('input', () => { clearTimeout(t); t = setTimeout(dry, 400); });
  ta.addEventListener('keydown', (e) => { if (e.key === 'Tab') { e.preventDefault(); ta.setRangeText('  ', ta.selectionStart, ta.selectionEnd, 'end'); } });
  async function submit() {
    saveBtn.disabled = true;
    const r = await save('PUT', `/api/v1/objects/${kind}/${encodeURIComponent(nameOf())}${isNew ? '?create=1' : ''}`, ta.value, { yaml: true });
    if (r.ok) return close();
    if (r.error.status === 422) mount(errBox, validationBox(r.error.data.validation, r.error.data.error, r.error.data.warnings));
    else if (r.error.status === 409) close();
  }
  const close = modal({
    title: isNew ? `New ${kind} (YAML)` : `${filePath(kind, name)}`, wide: true,
    body: [h('div', { class: 'sub small' }, 'Edit the file directly. ', h('a', { href: `/api/v1/schema/${kind.toLowerCase()}.schema.json`, target: '_blank' }, `${kind.toLowerCase()}.schema.json`),
      ' (v1alpha1) is checked first, then the semantic rules. Renaming is not supported.'), errBox, ta],
    foot: (c) => [h('span', { class: 'spacer' }), !isAdmin() && h('span', { class: 'muted small' }, 'read-only Role'), h('button', { onclick: c }, 'Cancel'), saveBtn],
  });
  if (!isNew) api('GET', `/api/v1/objects/${kind}/${encodeURIComponent(name)}/yaml`).then((y) => { ta.value = y; dry(); }).catch((e) => { ta.value = e.message; });
  else dry();
  return close;
}

// ---- generic object form ----------------------------------------------------------------------
// fields: [{ key: 'spec.x', label, type: text|number|select|multi|checks|checkbox|textarea|color,
//           options: [..] | (draft) => [..], required, help, show: (draft) => bool, full,
//           parse, format, immutable, lockedIf, rerender, paths: [server paths shown on this field], exact }]
// toObject(draft) turns the form state into the API object (e.g. an Address
// form edits "form" + "value" and produces exactly one of subnet/range/...).

export function getPath(o, p) { return p.split('.').reduce((a, k) => (a == null ? undefined : a[k]), o); }
export function setPath(o, p, v) {
  const ks = p.split('.'); let cur = o;
  for (const k of ks.slice(0, -1)) { if (cur[k] == null || typeof cur[k] !== 'object') cur[k] = {}; cur = cur[k]; }
  const last = ks[ks.length - 1];
  if (v === undefined || v === '' || (Array.isArray(v) && v.length === 0)) delete cur[last]; else cur[last] = v;
}
// prune removes empty objects left behind by cleared fields.
export function prune(o) {
  if (Array.isArray(o)) return o.map(prune);
  if (o && typeof o === 'object') {
    for (const [k, v] of Object.entries(o)) {
      const p = prune(v);
      if (p && typeof p === 'object' && !Array.isArray(p) && Object.keys(p).length === 0 && k !== 'spec') delete o[k]; else o[k] = p;
    }
  }
  return o;
}

function opts(f, o) {
  const raw = typeof f.options === 'function' ? f.options(o) : f.options || [];
  return raw.map((x) => (typeof x === 'string' ? { value: x, label: x } : x));
}

// multiPicker: chips + "add" select, used for sources/destinations/services/members.
// kind ('address' | 'service') resolves the chips in the right namespace.
export function multiPicker(values, options, onChange, placeholder = 'add…', kind) {
  const box = h('div', { class: 'chips' });
  const render = () => {
    box.replaceChildren();
    values.forEach((v, i) => box.append(refChip(v, () => { values.splice(i, 1); onChange(values); render(); }, kind)));
    const sel = h('select', { 'aria-label': placeholder, onchange: () => {
      if (!sel.value) return;
      values.push(sel.value);
      onChange(values); render();
    } }, h('option', { value: '' }, values.length ? '+ add' : placeholder));
    const groups = {};
    for (const o of options) if (!values.includes(o.value)) (groups[o.group || ''] ||= []).push(o);
    for (const [g, list] of Object.entries(groups)) {
      const parent = g ? h('optgroup', { label: g }) : sel;
      list.forEach((o) => parent.append(h('option', { value: o.value }, o.label || o.value)));
      if (g) sel.append(parent);
    }
    box.append(sel);
  };
  render();
  return box;
}

// objectForm renders a form for one object and handles dry-run validation,
// required fields (R-009.1) and saving.
export function objectForm({ kind, obj, isNew, fields, title, onSaved, extra, toObject = (d) => d, intro }) {
  const draft = structuredClone(obj);
  let serverErrs = [], serverMsg = '', serverWarn = [];
  const body = h('div', { class: 'stack' });
  const saveBtn = h('button', { class: 'primary', onclick: () => submit() }, isNew ? 'Create' : 'Save');
  const readOnly = !isAdmin();

  const isEmpty = (v) => v == null || v === '' || (Array.isArray(v) && v.length === 0);
  const missing = () => fields.filter((f) => f.required && (!f.show || f.show(draft)) && isEmpty(getPath(draft, f.key)));
  const payload = () => prune(toObject(structuredClone(draft)));

  let timer;
  const changed = () => { renderErrors(); clearTimeout(timer); timer = setTimeout(dryRun, 350); };

  async function dryRun() {
    const name = draft.metadata?.name;
    if (!name || readOnly || missing().length) return;
    try {
      const r = await api('POST', `/api/v1/validate/${kind}/${encodeURIComponent(name)}${isNew ? '?create=1' : ''}`, payload());
      serverErrs = r.valid ? [] : (r.validation || []); serverMsg = r.valid ? '' : r.error; serverWarn = r.warnings || [];
    } catch { /* ignore */ }
    renderErrors();
  }

  const fieldEls = new Map();
  const errBox = h('div');
  function renderErrors() {
    const miss = new Set(missing().map((f) => f.key));
    const claimed = new Set();
    for (const [f, el] of fieldEls) {
      const keys = f.paths || [f.key];
      const own = serverErrs.filter((e) => keys.some((k) => e.path === k || (!f.exact && (e.path.startsWith(k + '[') || e.path.startsWith(k + '.')))));
      own.forEach((e) => claimed.add(e));
      el.classList.toggle('invalid', miss.has(f.key) || own.length > 0);
      el.querySelector('.err').textContent = miss.has(f.key) ? 'Required' : own.map((e) => e.reason).join('; ');
    }
    const rest = serverErrs.filter((e) => !claimed.has(e));
    mount(errBox, validationBox(rest, rest.length || (serverMsg && !serverErrs.length) ? serverMsg : null, serverWarn));
    saveBtn.disabled = readOnly || miss.size > 0 || serverErrs.length > 0;
    saveBtn.title = miss.size ? 'Fill in the required fields' : '';
  }

  const grid = h('div', { class: 'form-grid' });
  function renderFields() {
    grid.replaceChildren(); fieldEls.clear();
    for (const f of fields) {
      if (f.show && !f.show(draft)) continue;
      const cur = getPath(draft, f.key);
      const set = (v) => { setPath(draft, f.key, f.parse ? f.parse(v) : v); if (f.rerender) renderFields(); changed(); };
      let input;
      const disabled = readOnly || (f.immutable && !isNew) || (f.lockedIf && f.lockedIf(draft));
      switch (f.type) {
        case 'select':
          input = h('select', { disabled, onchange: (e) => set(e.target.value) },
            opts(f, draft).map((o) => h('option', { value: o.value, selected: String(cur ?? '') === String(o.value) }, o.label ?? o.value)));
          break;
        case 'multi': {
          const vals = [...(cur || [])];
          input = disabled ? h('div', { class: 'chips' }, vals.map((v) => refChip(v, null, f.ns))) : multiPicker(vals, opts(f, draft), (v) => set([...v]), f.placeholder, f.ns);
          break;
        }
        case 'checks': {
          const vals = new Set(cur || []);
          input = h('div', { class: 'row wrap' }, opts(f, draft).map((o) => h('label', { class: 'check', title: o.title },
            h('input', { type: 'checkbox', disabled, checked: vals.has(o.value), onchange: (e) => { e.target.checked ? vals.add(o.value) : vals.delete(o.value); set(opts(f, draft).map((x) => x.value).filter((x) => vals.has(x))); } }), o.label)));
          break;
        }
        case 'checkbox':
          input = h('label', { class: 'check' }, h('input', { type: 'checkbox', disabled, checked: f.invert ? cur !== false : !!cur, onchange: (e) => set(f.invert ? (e.target.checked ? undefined : false) : e.target.checked) }), f.checkLabel || 'Enabled');
          break;
        case 'textarea':
          input = h('textarea', { disabled, placeholder: f.placeholder, oninput: (e) => set(e.target.value) }, f.format ? f.format(cur) : (cur ?? ''));
          break;
        case 'number':
          input = h('input', { type: 'number', disabled, value: cur ?? '', placeholder: f.placeholder, oninput: (e) => set(e.target.value === '' ? undefined : Number(e.target.value)) });
          break;
        case 'color': {
          let txt;
          input = h('div', { class: 'row' }, h('input', { type: 'color', disabled, value: cur || '#888888', style: { width: '48px', padding: '2px' }, oninput: (e) => { set(e.target.value.toUpperCase()); txt.value = e.target.value.toUpperCase(); } }),
            (txt = h('input', { disabled, value: cur || '', placeholder: '#2E7D32', oninput: (e) => set(e.target.value) })));
          break;
        }
        default:
          input = h('input', { disabled, value: f.format ? f.format(cur) : (cur ?? ''), placeholder: f.placeholder, autocomplete: 'off', oninput: (e) => set(e.target.value) });
      }
      const el = h('div', { class: ['field', (f.full || f.type === 'multi' || f.type === 'textarea') && 'full'] },
        h('label', null, f.label, f.required && h('span', { class: 'req' }, ' *')), input,
        f.help && h('div', { class: 'help' }, typeof f.help === 'function' ? f.help(draft) : f.help), h('div', { class: 'err' }));
      fieldEls.set(f, el);
      grid.append(el);
    }
    renderErrors();
  }

  async function submit() {
    if (missing().length) return renderErrors();
    saveBtn.disabled = true; saveBtn.textContent = 'Saving…';
    const name = draft.metadata.name;
    const r = await save('PUT', `/api/v1/objects/${kind}/${encodeURIComponent(name)}${isNew ? '?create=1' : ''}`, payload());
    saveBtn.textContent = isNew ? 'Create' : 'Save';
    if (r.ok) { close(); onSaved && onSaved(); return; }
    if (r.error.status === 422) { serverErrs = r.error.data.validation || []; serverMsg = r.error.data.error; serverWarn = r.error.data.warnings || []; }
    else if (r.error.status === 409) { close(); return; }
    renderErrors();
  }

  renderFields();
  append(body, [intro, errBox, grid, extra ? extra(draft, changed) : null]); // skips null/false
  const close = modal({
    title: title || (isNew ? `New ${kind}` : `${kind} ${obj.metadata.name}`), body,
    foot: (c) => [
      !isNew && h('button', { class: 'ghost', onclick: () => showYAML(kind, obj.metadata.name) }, icon('code'), 'YAML'),
      !isNew && isAdmin() && h('button', { class: 'ghost', onclick: () => { c(); yamlEditor(kind, obj.metadata.name); } }, icon('edit'), 'Edit YAML'),
      h('span', { class: 'spacer' }),
      readOnly && h('span', { class: 'muted small' }, 'read-only Role'),
      h('button', { onclick: c }, 'Cancel'), saveBtn],
  });
  if (!isNew) dryRun();
  return close;
}

export async function deleteObject(kind, name) {
  const refs = usedBy(kind, name);
  const text = refs.length
    ? `It is referenced by ${refs.slice(0, 5).join(', ')}${refs.length > 5 ? '…' : ''}: the Validator will refuse the deletion (DM-02).`
    : 'This creates a commit that removes the file. You can revert it from Changes.';
  if (!(await confirmDialog(`Delete ${kind} ${name}?`, text, 'Delete', true))) return;
  const r = await save('DELETE', `/api/v1/objects/${kind}/${encodeURIComponent(name)}`);
  if (!r.ok && r.error.status === 422) {
    modal({ title: 'Cannot delete', narrow: true, body: validationBox(r.error.data.validation, r.error.data.error) });
  }
}

// References to an object (helps before deleting, and in the lists).
export function usedBy(kind, name) {
  const out = [];
  const rsName = (rs, r) => `${rs.metadata.name}/${r.name}`;
  const addr = ['Address', 'AddressGroup'].includes(kind), svc = ['Service', 'ServiceGroup'].includes(kind), net = ['Interface', 'Zone'].includes(kind);
  for (const rs of objs('RuleSet')) {
    if (net && (rs.spec.from === name || rs.spec.to === name)) out.push(rs.metadata.name);
    for (const r of rs.spec.rules || []) {
      if (addr && ([...r.source, ...r.destination].includes(name) || r.nat?.translateIpTo === name)) out.push(rsName(rs, r));
      if (svc && (r.service.includes(name) || r.nat?.translateServiceTo === name)) out.push(rsName(rs, r));
    }
  }
  if (addr) for (const g of objs('AddressGroup')) if (g.spec.members.includes(name)) out.push(g.metadata.name);
  if (svc) for (const g of objs('ServiceGroup')) if (g.spec.members.includes(name)) out.push(g.metadata.name);
  if (addr && (sys().admin?.trustedHosts || []).includes(name)) out.push('System (trustedHosts)');
  const ss = sys().systemServices || {};
  if (net) for (const svc of ['dns', 'ntp']) if ((ss[svc]?.listenOn || []).includes(name)) out.push(`System (${svc}.listenOn)`);
  if (kind === 'Interface') {
    for (const z of objs('Zone')) if (z.spec.interfaces.includes(name)) out.push(`Zone ${z.metadata.name}`);
    for (const i of objs('Interface')) if (i.spec.parent === name) out.push(`VLAN ${i.metadata.name}`);
    for (const a of objs('Address')) if (a.spec.interface === name) out.push(a.metadata.name);
    for (const r of objs('StaticRoute')) if (r.spec.interface === name) out.push(`route ${r.metadata.name}`);
    if (ss.dhcp?.[name]) out.push('System (DHCP pool)');
  }
  if (kind === 'Color') for (const k of Object.keys(state.config?.objects || {})) for (const o of objs(k)) if (o.metadata.color === name) out.push(`${k} ${o.metadata.name}`);
  return out;
}

export function newObject(kind, spec, meta = {}) {
  return { apiVersion: API_VERSION, kind, metadata: { name: '', ...meta }, spec };
}

// Common metadata fields of every kind (common.schema.json#/$defs/metadata).
export const metaFields = ({ color = true, placeholder } = {}) => [
  { key: 'metadata.name', label: 'Name', required: true, immutable: true, placeholder, help: 'Lowercase letters, digits, ".", "_", "-"; "none" is reserved' },
  color && { key: 'metadata.color', label: 'Color', type: 'select', options: colorOptions, help: 'A classification from the palette (R-055)' },
  { key: 'metadata.comment', label: 'Comment', full: true, placeholder: 'What this is for' },
].filter(Boolean);
