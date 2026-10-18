// Local Users and the built-in Roles admin / read-only (R-043.8, R-043.9).
// Users are configuration too: each one is a YAML file with a bcrypt hash.
import { state, h, icon, mount, objs, sys, isAdmin, modal, save, deleteObject, showYAML, validationBox } from '../core.js';

function userForm(u) {
  const isNew = !u;
  const d = { name: u?.metadata.name || '', role: u?.spec.role || 'read-only', password: '' };
  const err = h('div');
  const btn = h('button', { class: 'primary', onclick: async () => {
    if (!d.name || (isNew && !d.password)) { mount(err, validationBox(null, 'Name and password are required')); return; }
    btn.disabled = true;
    const r = await save('PUT', `/api/v1/users/${encodeURIComponent(d.name)}`, { role: d.role, password: d.password });
    btn.disabled = false;
    if (r.ok) close(); else mount(err, validationBox(r.error.data?.validation, r.error.message));
  } }, isNew ? 'Create' : 'Save');
  const close = modal({
    title: isNew ? 'New User' : `User ${d.name}`, narrow: true,
    body: [err,
      h('div', { class: 'field' }, h('label', null, 'Username'), h('input', { value: d.name, disabled: !isNew, autocomplete: 'off', oninput: (e) => (d.name = e.target.value) })),
      h('div', { class: 'field' }, h('label', null, 'Role'), h('select', { onchange: (e) => (d.role = e.target.value) },
        [['admin', 'admin — full access'], ['read-only', 'read-only — view only, 403 on writes']].map(([v, l]) => h('option', { value: v, selected: d.role === v }, l)))),
      h('div', { class: 'field' }, h('label', null, isNew ? 'Password' : 'New password'), h('input', { type: 'password', autocomplete: 'new-password', placeholder: isNew ? 'at least 8 characters' : 'leave empty to keep', oninput: (e) => (d.password = e.target.value) }),
        h('div', { class: 'help' }, 'Stored as a bcrypt hash, cost 12 (R-043.1).'))],
    foot: (c) => [h('span', { class: 'spacer' }), h('button', { onclick: c }, 'Cancel'), btn],
  });
}

export default {
  render(root) {
    const users = objs('User');
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Users'), h('p', null, `Local administrative accounts (a PoC extension kind, users/). Sessions expire after ${sys().admin?.sessionTimeout || '8h'}; ${sys().admin?.lockout?.threshold || 5} failed logins lock the source address for ${sys().admin?.lockout?.duration || '10m'} (System.admin).`)),
        h('span', { class: 'spacer' }), isAdmin() && h('button', { class: 'primary', onclick: () => userForm() }, icon('plus'), 'User')),
      h('section', { class: 'card' }, h('table', null,
        h('thead', null, h('tr', null, ['User', 'Role', 'Two-factor', ''].map((t) => h('th', null, t)))),
        h('tbody', null, users.map((u) => h('tr', null,
          h('td', null, h('span', { class: 'row' }, isAdmin() ? h('button', { class: 'link', onclick: () => userForm(u) }, u.metadata.name) : u.metadata.name,
            u.metadata.name === state.me?.u && h('span', { class: 'badge accent' }, 'you'))),
          h('td', null, h('span', { class: ['badge', u.spec.role === 'admin' ? 'warn' : ''] }, u.spec.role)),
          h('td', { class: 'muted small' }, 'not in PoC (R-043.6)'),
          h('td', { class: 'num' }, h('span', { class: 'row', style: { justifyContent: 'flex-end', gap: '0' } },
            h('button', { class: 'ghost', title: 'YAML', onclick: () => showYAML('User', u.metadata.name) }, icon('code')),
            isAdmin() && u.metadata.name !== state.me?.u && h('button', { class: 'ghost', title: 'Delete', onclick: () => deleteObject('User', u.metadata.name) }, icon('trash'))))))))));
  },
};
