// Core helpers: DOM builder, API client, shared state, modals, toasts, forms.
// The UI holds no configuration of its own: it reads the tree from the API
// and every save goes back as a whole object with If-Match (components.md).

export const state = {
  me: null,          // { u, r }
  config: null,      // { revision, objects, activeRoutes, limits }
  status: null,      // ReconcilerStatus
  counters: null,    // Event Collector counters
  listeners: new Set(),
};

export const isAdmin = () => state.me && state.me.r === 'admin';
export const objs = (kind) => (state.config?.objects?.[kind]) || [];

export function emit() { for (const fn of state.listeners) fn(); }

// ---- DOM ------------------------------------------------------------------

export function h(tag, attrs, ...children) {
  const el = tag === 'svg' || attrs?.svg ? document.createElementNS('http://www.w3.org/2000/svg', tag) : document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false || k === 'svg') continue;
    if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
    else if (k === 'style' && typeof v === 'object') Object.assign(el.style, v);
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
  list: 'M8 6h13 M8 12h13 M8 18h13 M3 6h.01 M3 12h.01 M3 18h.01',
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
  eye: 'M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  power: 'M18.4 6.6a9 9 0 1 1-12.8 0 M12 2v10',
  zap: 'M13 2L3 14h9l-1 8 10-12h-9z',
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

// ---- colours (R-055) -----------------------------------------------------------

export function colourValue(name) {
  if (!name) return null;
  return objs('Colour').find((c) => c.metadata.name === name)?.spec.value || null;
}
export function dot(colourName, title) {
  const v = colourValue(colourName);
  if (!v) return null;
  const c = objs('Colour').find((x) => x.metadata.name === colourName);
  return h('span', { class: 'dot', style: { background: v }, title: title || `${colourName}${c?.spec.meaning ? ' — ' + c.spec.meaning : ''}` });
}

// Display a reference (Address, group, VIP, derived, Service...) as a chip.
export function refChip(name, onRemove) {
  let cls = 'chip', colour = null, title = name;
  const find = (k) => objs(k).find((o) => o.metadata.name === name);
  if (name.endsWith(' addresses')) { cls += ' derived'; title = 'Derived: the subnets of the Interface (follows IP changes)'; colour = objs('Interface').find((i) => i.metadata.name + ' addresses' === name)?.metadata.colour; }
  else {
    const o = find('Address') || find('AddressGroup') || find('VirtualIP') || find('Service') || find('ServiceGroup');
    if (o) {
      colour = o.metadata.colour;
      if (o.kind === 'VirtualIP') { cls += ' vip'; title = `VIP ${o.spec.external.ip}:${o.spec.external.port} → ${o.spec.mapped.ip}:${o.spec.mapped.port}`; }
      else if (o.kind === 'Address') title = `${o.spec.type} ${o.spec.value}`;
      else if (o.kind === 'AddressGroup' || o.kind === 'ServiceGroup') title = `group: ${o.spec.members.join(', ')}`;
      else if (o.kind === 'Service') title = o.spec.protocol + (o.spec.ports ? '/' + o.spec.ports : '');
    } else { title = 'missing object'; cls += ' missing'; }
  }
  return h('span', { class: cls, title }, dot(colour), name,
    onRemove && h('button', { type: 'button', title: 'Remove', onclick: onRemove }, '×'));
}

// ---- API ---------------------------------------------------------------------------

export class ApiError extends Error {
  constructor(status, data) { super(data?.error || `HTTP ${status}`); this.status = status; this.data = data || {}; }
}

export async function api(method, path, body, opts = {}) {
  const headers = { 'X-Requested-With': 'songbird' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (opts.rev) headers['If-Match'] = `"${opts.rev}"`;
  const res = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), credentials: 'same-origin' });
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
export async function save(method, path, body) {
  try {
    const r = await api(method, path, body, { rev: state.config.revision });
    if (r.applied) toast('Saved and applied', `commit ${short(r.commit)}`, 'good');
    else if (r.pending) toast('Saved, apply in progress', `commit ${short(r.commit)}`, 'warn');
    else toast('Saved, not applied', r.attempt?.error?.message || r.attempt?.result, 'bad', '#/diagnostics');
    await loadConfig();
    return { ok: true, result: r };
  } catch (e) {
    if (!(e instanceof ApiError)) throw e;
    if (e.status === 409 && e.data.conflict) {
      toast('Conflict', `${e.data.conflict.file} was changed by someone else. The form now shows nothing new: close it and edit again on the current revision.`, 'bad');
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
  setTimeout(() => t.remove(), kind === 'bad' ? 9000 : 4500);
}

export function modal({ title, body, foot, narrow, onClose }) {
  const close = () => { ov.remove(); document.removeEventListener('keydown', esc); onClose && onClose(); };
  const esc = (e) => { if (e.key === 'Escape') close(); };
  const ov = h('div', { class: 'overlay', onmousedown: (e) => { if (e.target === ov) close(); } },
    h('div', { class: ['modal', narrow && 'narrow'], role: 'dialog', 'aria-modal': 'true', 'aria-label': title },
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
    const close = modal({
      title, narrow: true, body: h('p', { style: { margin: 0 } }, text),
      onClose: () => { if (!done) resolve(false); },
      foot: (c) => [h('span', { class: 'spacer' }), h('button', { onclick: c }, 'Cancel'),
        h('button', { class: danger ? 'primary danger-bg' : 'primary', onclick: () => { done = true; resolve(true); c(); } }, okLabel)],
    });
    void close;
  });
}

export function showYAML(kind, name, rev) {
  const pre = h('pre', { class: 'code' }, 'Loading…');
  modal({ title: `${kind}/${name}`, body: [h('div', { class: 'sub small' }, 'The file in the configuration repository — the single source of truth (ADR-0001).'), pre] });
  api('GET', `/api/v1/objects/${kind}/${encodeURIComponent(name)}/yaml${rev ? '?rev=' + rev : ''}`)
    .then((t) => { pre.textContent = `# ${kind.toLowerCase()}s/${name}.yaml\n` + t; })
    .catch((e) => { pre.textContent = e.message; });
}

export function validationBox(errs, msg) {
  if (!errs?.length && !msg) return null;
  return h('div', { class: 'verr', role: 'alert' }, h('strong', null, msg || 'The change is not valid'),
    errs?.length ? h('ul', null, errs.map((e) => h('li', null, h('code', null, `${e.file}`), ' ', h('code', null, e.path), ' — ', e.reason,
      e.constraint && h('span', { class: 'muted' }, ` (${e.constraint})`)))) : null);
}

// ---- generic object form ----------------------------------------------------------------------
// fields: [{ key: 'spec.type', label, type: text|number|select|multi|checkbox|textarea|colour,
//           options: [..] | () => [..], required, help, show: (o) => bool, full, parse, format }]

export function getPath(o, p) { return p.split('.').reduce((a, k) => (a == null ? undefined : a[k]), o); }
export function setPath(o, p, v) {
  const ks = p.split('.'); let cur = o;
  for (const k of ks.slice(0, -1)) { if (cur[k] == null || typeof cur[k] !== 'object') cur[k] = {}; cur = cur[k]; }
  const last = ks[ks.length - 1];
  if (v === undefined || v === '' || (Array.isArray(v) && v.length === 0 && !cur.__keepEmpty)) delete cur[last]; else cur[last] = v;
}

export function colourOptions() {
  return [{ value: '', label: '— none —' }, ...objs('Colour').map((c) => ({ value: c.metadata.name, label: `${c.metadata.name}${c.spec.meaning ? ' — ' + c.spec.meaning : ''}` }))];
}

function opts(f, o) {
  const raw = typeof f.options === 'function' ? f.options(o) : f.options || [];
  return raw.map((x) => (typeof x === 'string' ? { value: x, label: x } : x));
}

// multiPicker: chips + "add" select, used for Policy sources/destinations/services.
export function multiPicker(values, options, onChange, placeholder = 'add…') {
  const box = h('div', { class: 'chips' });
  const render = () => {
    box.replaceChildren();
    values.forEach((v, i) => box.append(refChip(v, () => { values.splice(i, 1); onChange(values); render(); })));
    const sel = h('select', { 'aria-label': placeholder, onchange: () => { if (sel.value) { values.push(sel.value); onChange(values); render(); } } },
      h('option', { value: '' }, values.length ? '+ add' : placeholder));
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
export function objectForm({ kind, obj, isNew, fields, title, onSaved, extra }) {
  const draft = structuredClone(obj);
  let serverErrs = [], serverMsg = '';
  const body = h('div', { class: 'stack' });
  const saveBtn = h('button', { class: 'primary', onclick: () => submit() }, isNew ? 'Create' : 'Save');
  const readOnly = !isAdmin();

  const missing = () => fields.filter((f) => f.required && (!f.show || f.show(draft)) && isEmpty(getPath(draft, f.key)));
  const isEmpty = (v) => v == null || v === '' || (Array.isArray(v) && v.length === 0);

  let timer;
  const changed = () => {
    renderErrors();
    clearTimeout(timer);
    timer = setTimeout(dryRun, 350);
  };

  async function dryRun() {
    const name = draft.metadata?.name;
    if (!name || readOnly) return;
    try {
      const r = await api('POST', `/api/v1/validate/${kind}/${encodeURIComponent(name)}${isNew ? '?create=1' : ''}`, draft);
      serverErrs = r.valid ? [] : (r.validation || []); serverMsg = r.valid ? '' : r.error;
    } catch { /* ignore */ }
    renderErrors();
  }

  const fieldEls = new Map();
  const errBox = h('div');
  function renderErrors() {
    const miss = new Set(missing().map((f) => f.key));
    for (const [key, el] of fieldEls) {
      const own = serverErrs.filter((e) => e.path === key || e.path.startsWith(key + '[') || e.path.startsWith(key + '.'));
      el.classList.toggle('invalid', miss.has(key) || own.length > 0);
      const msg = el.querySelector('.err');
      msg.textContent = miss.has(key) ? 'Required' : own.map((e) => e.reason).join('; ');
    }
    mount(errBox, validationBox(serverErrs, serverErrs.length ? serverMsg : (serverMsg || null)));
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
          input = disabled ? h('div', { class: 'chips' }, vals.map((v) => refChip(v))) : multiPicker(vals, opts(f, draft), (v) => set([...v]));
          break;
        }
        case 'checks': {
          const vals = new Set(cur || []);
          input = h('div', { class: 'row wrap' }, opts(f, draft).map((o) => h('label', { class: 'check' },
            h('input', { type: 'checkbox', disabled, checked: vals.has(o.value), onchange: (e) => { e.target.checked ? vals.add(o.value) : vals.delete(o.value); set(opts(f, draft).map((x) => x.value).filter((x) => vals.has(x))); } }), o.label)));
          break;
        }
        case 'checkbox':
          input = h('label', { class: 'check' }, h('input', { type: 'checkbox', disabled, checked: !!cur, onchange: (e) => set(e.target.checked) }), f.checkLabel || 'Enabled');
          break;
        case 'textarea':
          input = h('textarea', { disabled, placeholder: f.placeholder, oninput: (e) => set(e.target.value) }, f.format ? f.format(cur) : (cur ?? ''));
          break;
        case 'number':
          input = h('input', { type: 'number', disabled, value: cur ?? '', placeholder: f.placeholder, oninput: (e) => set(e.target.value === '' ? undefined : Number(e.target.value)) });
          break;
        case 'colour':
          input = h('div', { class: 'row' }, h('input', { type: 'color', disabled, value: cur || '#888888', style: { width: '48px', padding: '2px' }, oninput: (e) => { set(e.target.value.toUpperCase()); txt.value = e.target.value.toUpperCase(); } }),
            (txt = h('input', { disabled, value: cur || '', placeholder: '#49F527', oninput: (e) => set(e.target.value) })));
          break;
        default:
          input = h('input', { disabled, value: f.format ? f.format(cur) : (cur ?? ''), placeholder: f.placeholder, autocomplete: 'off', oninput: (e) => set(e.target.value) });
      }
      const el = h('div', { class: ['field', (f.full || f.type === 'multi' || f.type === 'textarea') && 'full'] },
        h('label', null, f.label, f.required && h('span', { class: 'req' }, ' *')), input,
        f.help && h('div', { class: 'help' }, typeof f.help === 'function' ? f.help(draft) : f.help), h('div', { class: 'err' }));
      fieldEls.set(f.key, el);
      grid.append(el);
    }
    renderErrors();
  }
  let txt;

  async function submit() {
    if (missing().length) return renderErrors();
    saveBtn.disabled = true; saveBtn.textContent = 'Saving…';
    const name = draft.metadata.name;
    const r = await save('PUT', `/api/v1/objects/${kind}/${encodeURIComponent(name)}${isNew ? '?create=1' : ''}`, draft);
    saveBtn.textContent = isNew ? 'Create' : 'Save';
    if (r.ok) { close(); onSaved && onSaved(); return; }
    if (r.error.status === 422) { serverErrs = r.error.data.validation || []; serverMsg = r.error.data.error; }
    else if (r.error.status === 409) { close(); return; }
    renderErrors();
  }

  renderFields();
  body.append(errBox, grid, extra ? extra(draft, changed) : null);
  const close = modal({
    title: title || (isNew ? `New ${kind}` : `${kind} ${obj.metadata.name}`), body,
    foot: (c) => [
      !isNew && h('button', { class: 'ghost', onclick: () => showYAML(kind, obj.metadata.name) }, icon('code'), 'YAML'),
      h('span', { class: 'spacer' }),
      readOnly && h('span', { class: 'muted small' }, 'read-only Role'),
      h('button', { onclick: c }, 'Cancel'), saveBtn],
  });
  if (!isNew) dryRun();
  return close;
}

export async function deleteObject(kind, name) {
  if (!(await confirmDialog(`Delete ${kind} ${name}?`, 'This creates a commit that removes the file. You can revert it from Changes.', 'Delete', true))) return;
  const r = await save('DELETE', `/api/v1/objects/${kind}/${encodeURIComponent(name)}`);
  if (!r.ok && r.error.status === 422) {
    modal({ title: 'Cannot delete', narrow: true, body: validationBox(r.error.data.validation, r.error.data.error) });
  }
}

// References to an object from Policies and groups (helps before deleting).
export function usedBy(name) {
  const out = [];
  for (const z of objs('Zone')) for (const p of z.spec.policies || [])
    if ([...p.sources, ...p.destinations, ...p.services].includes(name)) out.push(`${z.metadata.name}/${p.name}`);
  for (const g of [...objs('AddressGroup'), ...objs('ServiceGroup')]) if (g.spec.members.includes(name)) out.push(g.metadata.name);
  for (const z of objs('Zone')) for (const p of z.spec.policies || []) if (p.nat?.pool === name) out.push(`${z.metadata.name}/${p.name}`);
  return out;
}

export function newObject(kind, spec) {
  return { apiVersion: 'songbird.io/v1alpha1', kind, metadata: { name: '' }, spec };
}
