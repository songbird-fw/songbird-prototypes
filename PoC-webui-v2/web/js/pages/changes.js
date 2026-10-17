// Git history of the configuration repository: every saved change is a
// commit (ADR-0001). Rollback is a new commit that restores a revision
// (components.md), applied by the normal pipeline.
import { state, h, icon, mount, api, short, ago, fmtTime, isAdmin, confirmDialog, save } from '../core.js';

let selected = null;

function colourDiff(text) {
  const pre = h('pre', { class: 'code' });
  if (!text.trim()) { pre.textContent = 'No file changes.'; return pre; }
  for (const l of text.split('\n')) {
    let cls = null;
    if (l.startsWith('diff --git')) { pre.append(h('span', { class: 'file' }, l.replace(/^diff --git a\/(\S+) b\/.*/, '$1'))); continue; }
    if (/^(index |--- |\+\+\+ |new file|deleted file|similarity|rename )/.test(l)) continue;
    if (l.startsWith('@@')) cls = 'hunk'; else if (l.startsWith('+')) cls = 'add'; else if (l.startsWith('-')) cls = 'del';
    pre.append(cls ? h('span', { class: cls }, l) : l + '\n');
  }
  return pre;
}

export default {
  render(root, rest) {
    if (rest?.[0]) selected = rest[0];
    const list = h('div');
    const detail = h('div', { class: 'card' }, h('div', { class: 'empty' }, 'Select a commit'));
    mount(root,
      h('div', { class: 'page-head' }, h('div', null, h('h1', null, 'Changes'),
        h('p', null, 'History of the configuration repository. The UI, the CLI and pulls all produce commits; the Reconciler applies HEAD.'))),
      h('div', { class: 'split' }, h('section', { class: 'card', style: { maxHeight: 'calc(100vh - 190px)', overflowY: 'auto' } }, list), detail));

    let commits = [];
    const st = () => state.status || {};

    async function show(c) {
      selected = c.id;
      for (const el of list.children) el.classList.toggle('sel', el.dataset.id === c.id);
      const diff = await api('GET', `/api/v1/history/${c.id}/diff`).catch((e) => e.message);
      const isHead = c.id === state.config.revision;
      mount(detail,
        h('div', { class: 'head' }, h('div', null, h('h2', null, c.subject), h('div', { class: 'small sub' }, h('code', null, short(c.id)), ` · ${c.author} · ${fmtTime(c.time)}`)),
          h('span', { class: 'spacer' }),
          isAdmin() && !isHead && h('button', { onclick: () => revert(c) }, icon('undo'), 'Restore this revision')),
        h('div', { class: 'body stack' },
          c.body && h('pre', { class: 'code', style: { background: 'none', border: 'none', padding: 0 } }, c.body),
          colourDiff(diff)));
    }

    async function revert(c) {
      const ok = await confirmDialog(`Restore revision ${short(c.id)}?`,
        'A new commit is created whose content equals this revision. History is kept; the Reconciler applies it like any other change.', 'Restore');
      if (!ok) return;
      const r = await save('POST', `/api/v1/history/${c.id}/revert`);
      if (r.ok) { selected = r.result.commit; load(); }
    }

    async function load() {
      commits = await api('GET', '/api/v1/history?n=200');
      mount(list, commits.map((c) => {
        const tags = [];
        if (c.id === st().desiredCommit) tags.push(h('span', { class: 'badge accent' }, 'HEAD'));
        if (c.id === st().appliedCommit) tags.push(h('span', { class: 'badge good' }, 'running'));
        else if (c.id === st().lastKnownGood) tags.push(h('span', { class: 'badge' }, 'last known good'));
        const el = h('div', { class: ['commit', c.id === selected && 'sel'], 'data-id': c.id, tabindex: 0, onclick: () => show(c), onkeydown: (e) => { if (e.key === 'Enter') show(c); } },
          h('div', { class: 'row' }, h('span', { class: 'subj' }, c.subject), h('span', { class: 'spacer' }), tags),
          h('div', { class: 'small muted row' }, h('code', null, short(c.id)), c.author, '·', h('span', { title: fmtTime(c.time) }, ago(c.time)), '·', `${c.files.length} file${c.files.length === 1 ? '' : 's'}`));
        return el;
      }));
      const sel = commits.find((c) => c.id === selected) || commits.find((c) => c.id.startsWith(selected || '-')) || commits[0];
      if (sel) show(sel);
    }
    load();
  },
};
