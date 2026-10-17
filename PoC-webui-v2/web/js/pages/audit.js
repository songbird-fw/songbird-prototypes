// Audit Log: administrative actions and authentication events (R-035,
// R-043.10), hash-chained and verifiable (R-035.1).
import { h, icon, mount, api, fmtTime, modal } from '../core.js';

let filter = '';

export default {
  render(root) {
    const tbody = h('tbody');
    const verify = h('span');
    let entries = [];
    const paint = () => {
      const f = filter.toLowerCase();
      const shown = entries.filter((e) => !f || [e.actor, e.action, e.target, e.detail, e.source].some((x) => (x || '').toLowerCase().includes(f)));
      mount(tbody, shown.length ? shown.map((e) => h('tr', null,
        h('td', { class: 'num small muted' }, e.seq),
        h('td', { class: 'small mono' }, fmtTime(e.time)),
        h('td', null, e.actor),
        h('td', { class: 'mono small' }, e.source),
        h('td', null, h('span', { class: ['badge', e.action.includes('fail') || e.action.includes('lockout') || e.action.includes('refused') || e.action.includes('rolled') || e.action.includes('invalid') ? 'bad' : e.action.startsWith('apply.applied') ? 'good' : ''] }, e.action)),
        h('td', { class: 'small' }, e.target || ''),
        h('td', { class: 'small sub' }, e.detail || ''),
        h('td', null, (e.before || e.after) && h('button', { class: 'ghost sm', onclick: () => modal({ title: `#${e.seq} ${e.action} ${e.target || ''}`,
          body: h('div', { class: 'grid cols-2' }, h('div', null, h('h3', null, 'Before'), h('pre', { class: 'code' }, JSON.stringify(e.before ?? null, null, 2))),
            h('div', null, h('h3', null, 'After'), h('pre', { class: 'code' }, JSON.stringify(e.after ?? null, null, 2)))) }) }, 'diff')),
        h('td', { class: 'mono small muted', title: `hash ${e.hash}\nprev ${e.prev}` }, e.hash.slice(0, 10))))
        : h('tr', null, h('td', { colspan: 9, class: 'empty' }, 'No entries.')));
    };
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Audit log'), h('p', null, 'Append-only. Each entry contains the SHA-256 of the previous one, so editing or removing an entry breaks the chain.')),
        h('span', { class: 'spacer' }), verify,
        h('input', { placeholder: 'Filter…', value: filter, style: { width: '200px' }, oninput: (e) => { filter = e.target.value; paint(); } }),
        h('button', { onclick: async () => {
          const v = await api('GET', '/api/v1/audit/verify');
          mount(verify, v.ok ? h('span', { class: 'badge good' }, `chain intact · ${v.entries} entries`) : h('span', { class: 'badge bad' }, `broken: ${v.reason}`));
        } }, icon('check'), 'Verify chain')),
      h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
        h('thead', null, h('tr', null, ['#', 'Time', 'Actor', 'Source', 'Action', 'Target', 'Detail', '', 'Hash'].map((t, k) => h('th', { class: k === 0 && 'num' }, t)))),
        tbody))));
    api('GET', '/api/v1/audit').then((e) => { entries = e; paint(); });
  },
};
