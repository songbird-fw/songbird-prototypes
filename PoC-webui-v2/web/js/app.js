import { state, h, icon, mount, api, loadConfig, short, isAdmin, toast, findings, sys } from './core.js';
import dashboard from './pages/dashboard.js';
import policies from './pages/policies.js';
import network from './pages/network.js';
import system from './pages/system.js';
import objects from './pages/objects.js';
import traffic from './pages/traffic.js';
import changes from './pages/changes.js';
import diagnostics from './pages/diagnostics.js';
import audit from './pages/audit.js';
import users from './pages/users.js';

const ROUTES = [
  { group: 'Monitor' },
  { path: 'dashboard', label: 'Dashboard', icon: 'gauge', page: dashboard },
  { path: 'traffic', label: 'Live traffic', icon: 'activity', page: traffic },
  { path: 'diagnostics', label: 'Diagnostics', icon: 'heart', page: diagnostics },
  { group: 'Configuration' },
  { path: 'policies', label: 'Policies', icon: 'shield', page: policies },
  { path: 'network', label: 'Network', icon: 'network', page: network },
  { path: 'objects', label: 'Objects', icon: 'box', page: objects },
  { path: 'system', label: 'System', icon: 'cog', page: system },
  { path: 'changes', label: 'Changes', icon: 'history', page: changes },
  { group: 'Administration' },
  { path: 'users', label: 'Users', icon: 'users', page: users },
  { path: 'audit', label: 'Audit log', icon: 'file', page: audit },
];

const app = document.getElementById('app');
let current = null;     // current route
let leave = null;       // cleanup of the current page
let content, banner, topStatus;

// ---- theme (R-044, R-044.1: toggle reachable from every page) --------------
function theme() {
  try { return localStorage.getItem('songbird-theme'); } catch { return null; }
}
function applyTheme(t) {
  if (t) document.documentElement.dataset.theme = t; else delete document.documentElement.dataset.theme;
}
// The viewer's choice wins; otherwise System.admin.ui.theme; "auto" follows the OS.
function configuredTheme() {
  const t = sys().admin?.ui?.theme;
  return t === 'dark' || t === 'light' ? t : null;
}
function effectiveTheme() {
  return theme() || configuredTheme() || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
}
function toggleTheme() {
  const next = effectiveTheme() === 'dark' ? 'light' : 'dark';
  try { localStorage.setItem('songbird-theme', next); } catch { /* private mode */ }
  applyTheme(next);
  renderTop();
}
applyTheme(theme());
state.listeners.add(() => { if (!theme()) applyTheme(configuredTheme()); });

// ---- login ---------------------------------------------------------------------
function loginPage() {
  const user = h('input', { autocomplete: 'username', value: '' });
  const pass = h('input', { type: 'password', autocomplete: 'current-password' });
  const err = h('div', { class: 'err', style: { color: 'var(--critical)', minHeight: '18px', fontSize: '13px' }, role: 'alert' });
  const btn = h('button', { class: 'primary', type: 'submit', style: { justifyContent: 'center', minHeight: '38px' } }, 'Sign in');
  const form = h('form', { class: 'card', onsubmit: async (e) => {
    e.preventDefault(); btn.disabled = true; err.textContent = '';
    try {
      state.me = await api('POST', '/api/v1/login', { username: user.value, password: pass.value });
      location.hash = '#/dashboard';
      start();
    } catch (ex) {
      err.textContent = ex.status === 429 ? 'Too many failed attempts from this address. Try again in 10 minutes.' : ex.message;
      btn.disabled = false; pass.select();
    }
  } },
  h('div', { class: 'title' }, h('span', { class: 'logo' }, icon('shield')), h('div', null, h('h1', null, 'Songbird'), h('small', null, 'Sign in to manage the firewall.'))),
  h('div', { class: 'field' }, h('label', null, 'Username'), user),
  h('div', { class: 'field' }, h('label', null, 'Password'), pass),
  err, btn,
  h('div', { class: 'muted small' }, 'Demo accounts: admin / songbird, or auditor / auditor for read-only access.'));
  mount(app, h('div', { class: 'login' }, form));
  setTimeout(() => user.focus(), 0);
}

// ---- shell -------------------------------------------------------------------------
function shell() {
  content = h('div', { class: 'content' });
  banner = h('div');
  topStatus = h('div', { class: 'top' });
  // icons only; the label appears on hover and keyboard focus
  const nav = h('nav', { class: 'nav', 'aria-label': 'Main' }, ROUTES.map((r) => r.group
    ? h('div', { class: 'group', role: 'separator', 'aria-label': r.group })
    : h('a', { href: `#/${r.path}`, 'data-path': r.path, 'aria-label': r.label },
      icon(r.icon), h('span', { class: 'label', 'aria-hidden': 'true' }, r.label))));
  mount(app, h('div', { class: 'shell' },
    h('aside', { class: 'side' },
      h('a', { class: 'brand', href: '#/dashboard', 'aria-label': 'Songbird', title: 'Songbird — configuration is YAML in Git, every save is a commit' }, h('span', { class: 'logo' }, icon('shield'))),
      nav),
    h('div', { class: 'main' }, topStatus, banner, content)));
}


function stateBadge(s) {
  if (!s) return h('span', { class: 'badge' }, '…');
  const cls = { 'in-sync': 'good', reconciling: 'warn', starting: 'warn', failed: 'bad', degraded: 'bad' }[s.state] || '';
  return h('span', { class: ['badge', cls], title: 'Reconciler state (reconciliation.md §9)' }, s.state);
}

// Validator findings on HEAD: errors block an Apply, warnings never do.
function findingsPill() {
  const f = findings();
  const errs = f.filter((e) => e.severity === 'error').length, warns = f.length - errs;
  if (!f.length) return h('a', { class: 'pill', href: '#/diagnostics', title: 'HEAD passes the schema and every semantic rule' }, h('span', { class: 'k' }, 'Validator'), icon('check', 'ok-ico'), 'valid');
  return h('a', { class: 'pill', href: '#/diagnostics', title: 'Validator findings on HEAD' }, h('span', { class: 'k' }, 'Validator'),
    errs ? h('span', { class: 'badge bad' }, `${errs} error${errs > 1 ? 's' : ''}`) : null,
    warns ? h('span', { class: 'badge warn' }, `${warns} warning${warns > 1 ? 's' : ''}`) : null);
}

function renderTop() {
  if (!topStatus) return;
  const s = state.status;
  mount(topStatus,
    h('span', { class: 'pill', title: 'Desired state: HEAD of the configuration repository' }, h('span', { class: 'k' }, 'Revision'), h('code', null, short(state.config?.revision))),
    h('span', { class: 'pill' }, h('span', { class: 'k' }, 'Reconciler'), stateBadge(s)),
    s && s.appliedCommit && s.appliedCommit !== s.desiredCommit && h('span', { class: 'pill' }, h('span', { class: 'k' }, 'Running'), h('code', null, short(s.appliedCommit))),
    findingsPill(),
    h('span', { class: 'spacer' }),
    h('span', { class: 'pill' }, h('span', { class: 'k' }, state.me?.r), state.me?.u),
    h('button', { class: 'ghost', onclick: toggleTheme, title: 'Toggle dark theme', 'aria-label': 'Toggle dark theme' }, icon(effectiveTheme() === 'dark' ? 'sun' : 'moon')),
    h('button', { class: 'ghost', title: 'Sign out', 'aria-label': 'Sign out', onclick: async () => { await api('POST', '/api/v1/logout').catch(() => {}); state.me = null; stop(); location.hash = '#/login'; } }, icon('logout')));

  // Banner while desired != applied (reconciliation.md §9)
  let b = null;
  if (s && (s.state === 'failed' || s.state === 'degraded')) {
    b = h('div', { class: 'banner critical', role: 'alert' }, icon('alert'),
      h('div', null, h('strong', null, `Configuration ${short(s.desiredCommit)} is not applied. `),
        s.lastError?.message || '', ' Running: ', h('code', null, short(s.appliedCommit) || 'nothing'), '.'),
      h('span', { class: 'spacer' }),
      h('a', { href: '#/diagnostics' }, 'Diagnostics'),
      isAdmin() && s.appliedCommit && h('a', { href: `#/changes/${s.appliedCommit}` }, 'Revert…'));
  } else if (s && s.state === 'reconciling') {
    b = h('div', { class: 'banner info' }, icon('zap'), 'Applying configuration…');
  }
  mount(banner, b);
}

function route() {
  const [path, ...rest] = location.hash.replace(/^#\/?/, '').split('/');
  if (!state.me) { loginPage(); return; }
  if (path === 'login' || !path) { location.hash = '#/dashboard'; return; }
  const r = ROUTES.find((x) => x.path === path) || ROUTES[1];
  if (!content) shell();
  for (const a of app.querySelectorAll('.nav a')) a.classList.toggle('active', a.dataset.path === r.path);
  if (leave) { leave(); leave = null; }
  current = r;
  document.title = `${r.label} · Songbird`;
  leave = r.page.render(content, rest) || null;
  renderTop();
}

// ---- polling -------------------------------------------------------------------------
let timer = null;
async function poll() {
  try {
    [state.status, state.counters] = await Promise.all([api('GET', '/api/v1/status'), api('GET', '/api/v1/events?n=0').then((r) => r.counters)]);
    if (state.config && state.status.desiredCommit && state.status.desiredCommit !== state.config.revision) {
      // someone (UI, CLI, revert, pull) committed: reload the tree
      await loadConfig(); // emits: the page re-renders unless a dialog is open
      if (document.querySelector('.overlay')) toast('Configuration changed', `HEAD is now ${short(state.config.revision)}. Saving an edited object may conflict.`, 'warn');
    }
    renderTop();
    current?.page.refresh?.();
  } catch { /* 401 handled in api() */ }
}

function rerender() {
  if (current && !document.querySelector('.overlay')) {
    if (leave) leave();
    leave = current.page.render(content, location.hash.split('/').slice(2)) || null;
  }
}

// after any config reload, re-render the page (deferred so a dialog that
// is closing after a save is gone first)
state.listeners.add(() => { renderTop(); setTimeout(rerender, 0); });

async function start() {
  await loadConfig();
  state.status = await api('GET', '/api/v1/status');
  content = null;
  route();
  clearInterval(timer);
  timer = setInterval(poll, 2000);
}
function stop() { clearInterval(timer); content = null; if (leave) leave(); leave = null; }

window.addEventListener('hashchange', route);
window.songbirdRerender = rerender;

(async () => {
  try { state.me = await api('GET', '/api/v1/me'); await start(); }
  catch { loginPage(); }
})();
