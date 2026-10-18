// Diagnostics: real-time status of each Subsystem (R-032.1), the Reconciler
// (reconciliation.md §9) with the last transaction, and PoC fault injection.
import { state, h, icon, mount, api, short, fmtTime, fmtNum, isAdmin, validationBox, toast, findings } from '../core.js';

const LABEL = { dataplane: 'Data Plane', network: 'Network driver', dhcp: 'DHCP', dns: 'DNS', reconciler: 'Reconciler', api: 'API / UI', gitops: 'GitOps sync', alerting: 'Alerting' };

function statusBadge(s, detail) {
  if (s === 'stopped' && /not in phase|enabled is false|not implemented/.test(detail || '')) return h('span', { class: 'badge' }, 'not configured');
  return h('span', { class: ['badge', { running: 'good', degraded: 'warn', stopped: 'bad' }[s]] }, s);
}

export default {
  render(root) {
    const subsys = h('div', { class: 'subsys' });
    const recBox = h('div');
    const faults = h('div');
    const valid = h('div');
    const overall = h('span');
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Diagnostics'), h('p', null, 'Subsystem status from /health, refreshed every 2 s.')),
        h('span', { class: 'spacer' }), overall, h('a', { href: '/health', target: '_blank', class: 'small' }, 'GET /health')),
      h('div', { class: 'stack' },
        h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'Subsystems')), h('div', { class: 'body' }, subsys)),
        valid,
        recBox,
        faults));

    async function paint() {
      const health = await api('GET', '/health').catch(() => null);
      const s = state.status;
      if (health) {
        mount(overall, h('span', { class: 'row' }, 'Overall', statusBadge(health.status)));
        const order = ['reconciler', 'network', 'dataplane', 'dhcp', 'dns', 'api', 'gitops', 'alerting'];
        mount(subsys, order.filter((k) => health.subsystems[k]).map((k) => {
          const x = health.subsystems[k];
          return h('div', { class: 'item' }, h('div', { class: 'row' }, h('span', { class: 'name' }, LABEL[k] || k), h('span', { class: 'spacer' }), statusBadge(x.status, x.detail)),
            h('div', { class: 'small muted', style: { marginTop: '4px' } }, x.detail || ''));
        }));
      }
      const f = findings();
      mount(valid, h('section', { class: 'card' },
        h('div', { class: 'head' }, h('h2', null, 'Validation of HEAD'), h('span', { class: 'spacer' }),
          h('span', { class: 'muted small' }, `schema ${state.config?.apiVersion || ''} + semantic rules`)),
        h('div', { class: 'body' }, f.length ? validationBox(f.filter((e) => e.severity === 'error'), f.some((e) => e.severity === 'error') ? 'Errors: HEAD cannot be applied' : null, f.filter((e) => e.severity === 'warning'))
          : h('div', { class: 'vok' }, icon('check'), 'No errors and no warnings.'))));
      if (!s) return;
      const a = s.lastAttempt;
      const p = s.plan;
      mount(recBox, h('div', { class: 'grid cols-2' },
        h('section', { class: 'card' },
          h('div', { class: 'head' }, h('h2', null, 'Reconciler'), h('span', { class: 'spacer' }), h('span', { class: ['badge', { 'in-sync': 'good', reconciling: 'warn', failed: 'bad', degraded: 'bad' }[s.state]] }, s.state)),
          h('div', { class: 'body stack' },
            h('table', null, h('tbody', null,
              h('tr', null, h('td', { class: 'sub' }, 'Desired (HEAD)'), h('td', null, h('code', null, short(s.desiredCommit)))),
              h('tr', null, h('td', { class: 'sub' }, 'Applied (running)'), h('td', null, h('code', null, short(s.appliedCommit) || '—'),
                s.desiredCommit !== s.appliedCommit && h('span', { class: 'badge warn', style: { marginLeft: '8px' } }, 'desired ≠ applied'))),
              h('tr', null, h('td', { class: 'sub' }, 'Last known good'), h('td', null, h('code', null, short(s.lastKnownGood) || '—'))),
              a && h('tr', null, h('td', { class: 'sub' }, 'Last attempt'), h('td', null, `${a.result} · ${short(a.commit)} · ${fmtTime(a.time)}`)),
              a && h('tr', null, h('td', { class: 'sub' }, 'Drivers touched'), h('td', null, a.changed?.length ? a.changed.join(', ') : 'none (no-op)')))),
            s.lastError && (s.lastError.validation?.length ? validationBox(s.lastError.validation, `Not applied: ${s.lastError.message}`)
              : h('div', { class: 'verr' }, h('strong', null, `Step ${s.lastError.step}${s.lastError.driver ? ' · ' + s.lastError.driver : ''}: `), s.lastError.message)),
            a?.steps?.length ? h('div', null, h('h3', { style: { marginBottom: '6px' } }, 'Last transaction (§6.2 order)'),
              h('div', { class: 'steps' }, a.steps.map((st) => h('div', { class: 's' },
                st.ok ? icon(st.op === 'rollback' ? 'undo' : 'check', st.op === 'rollback' ? 'bad-ico' : 'ok-ico') : icon('x', 'bad-ico'),
                h('strong', null, st.driver), h('span', { class: st.op === 'rollback' ? 'bad-ico' : 'sub' }, st.op), h('span', { class: 'muted small' }, st.note))))) : null)),
        h('section', { class: 'card' },
          h('div', { class: 'head' }, h('h2', null, 'Compiled plan'), h('span', { class: 'spacer' }), h('span', { class: 'muted small' }, 'simulated Compiler')),
          p ? h('div', { class: 'body stack' },
            h('div', { class: 'grid kpi' },
              h('div', { title: 'RuleSets and the directed Interface pairs they cover once Zones are expanded' }, h('div', { class: 'sub small' }, 'RuleSets'), h('div', { style: { fontSize: '20px', fontWeight: 650 } }, p.ruleSets), h('div', { class: 'muted small' }, `${p.pairs} Interface pairs`)),
              h('div', null, h('div', { class: 'sub small' }, 'Enabled policies'), h('div', { style: { fontSize: '20px', fontWeight: 650 } }, p.policies), p.disabled ? h('div', { class: 'muted small' }, `${p.disabled} disabled, not compiled`) : null),
              h('div', { title: 'Groups and lists expanded: sources × destinations × services × Interface pairs (R-014.1–R-014.3)' }, h('div', { class: 'sub small' }, 'Match entries'), h('div', { style: { fontSize: '20px', fontWeight: 650 } }, fmtNum(p.matchEntries)),
                h('div', { class: 'muted small' }, `${p.dnatEntries} DNAT entries / ${fmtNum(state.config?.limits?.dnatEntries)}`))),
            Object.keys(p.intraZone || {}).length ? h('div', null, h('h3', { style: { marginBottom: '6px' } }, 'Between members of a Zone without a RuleSet (DM-39)'),
              h('div', { class: 'row wrap' }, Object.entries(p.intraZone).map(([z, v]) => h('span', { class: 'pill' }, z, h('strong', null, v))))) : null,
            h('div', null, h('h3', { style: { marginBottom: '6px' } }, 'Local-in (allowAccess + implicit service ports)'),
              h('table', null, h('tbody', null, Object.entries(p.localIn).map(([k, v]) => h('tr', null, h('td', null, k), h('td', null, v.length ? v.join(', ') : h('span', { class: 'muted' }, 'nothing — dropped'))))))),
            h('div', null, h('h3', { style: { marginBottom: '6px' } }, 'Section hashes'),
              h('div', { class: 'row wrap' }, Object.entries(p.sections).map(([k, v]) => h('span', { class: 'pill' }, k, h('code', null, v))))))
            : h('div', { class: 'empty' }, 'Nothing applied yet.'))));

      mount(faults, h('section', { class: 'card' },
        h('div', { class: 'head' }, icon('zap'), h('h2', null, 'Fault injection'), h('span', { class: 'badge outline' }, 'PoC only')),
        h('div', { class: 'body stack' },
          h('p', { class: 'sub', style: { margin: 0 } }, 'Make a driver fail its next commit to see the atomic apply roll back (R-020.6), the Failed state and the banner. Clearing the fault counts as a runtime fact change, so the Reconciler retries HEAD.'),
          h('table', null, h('tbody', null, s.drivers.map((d) => h('tr', null,
            h('td', null, h('strong', null, LABEL[d.name])), h('td', null, statusBadge(d.status)),
            h('td', { class: 'mono small' }, d.appliedHash || '—'),
            h('td', null, h('label', { class: 'check' }, h('input', { type: 'checkbox', disabled: !isAdmin(), checked: d.failNextApply, onchange: (e) => setFault(d, { fail: e.target.checked, stopped: d.stopped }) }), 'Fail next apply')),
            h('td', null, ['dhcp', 'dns'].includes(d.name) && h('label', { class: 'check' }, h('input', { type: 'checkbox', disabled: !isAdmin(), checked: d.stopped, onchange: (e) => setFault(d, { fail: d.failNextApply, stopped: e.target.checked }) }), 'Stop service')))))))));
    }

    async function setFault(d, { fail, stopped }) {
      try { state.status = await api('POST', '/api/v1/diagnostics/faults', { driver: d.name, fail, stopped }); paint(); }
      catch (e) { toast('Not changed', e.message, 'bad'); }
    }

    paint();
    this._paint = paint;
    return () => { this._paint = null; };
  },
  refresh() { if (!document.querySelector('.overlay')) this._paint?.(); },
};
