// Dashboard: state of the pipeline at a glance and real-time charts (R-027).
import { state, h, mount, api, short, fmtNum, fmtBps, objs } from '../core.js';
import { lineChart, bars } from '../chart.js';

function kpi(label, value, note, cls) {
  return h('div', { class: 'card kpi' }, h('div', { class: 'label' }, label), h('div', { class: ['value', cls] }, value), note && h('div', { class: 'note' }, note));
}

// Time ranges of the charts (R-027): the server keeps raw samples for 5
// minutes and averages for 1 hour, 1 day and 1 week (internal/observer).
const RANGES = [
  { id: 'rt', label: 'Real-time', every: 2000, tick: { hour: '2-digit', minute: '2-digit', second: '2-digit' } },
  { id: '5m', label: '5 min', every: 2000, tick: { hour: '2-digit', minute: '2-digit' } },
  { id: '1h', label: '1 h', every: 10000, tick: { hour: '2-digit', minute: '2-digit' } },
  { id: '1d', label: '1 day', every: 30000, tick: { hour: '2-digit', minute: '2-digit' } },
  { id: '1w', label: '1 week', every: 60000, tick: { weekday: 'short', day: 'numeric' } },
];
let range = (() => { try { return localStorage.getItem('songbird-range') || '5m'; } catch { return '5m'; } })();

export default {
  render(root) {
    const kpis = h('div', { class: 'grid kpi' });
    const sys = h('div');
    const net = h('div');
    const rankings = h('div', { class: 'grid cols-2' });
    let dir = 'rx';
    const rangeSeg = h('div', { class: 'seg', role: 'group', 'aria-label': 'Time range' }, RANGES.map((r) => h('button', { class: range === r.id ? 'on' : '', onclick: (e) => {
      range = r.id; try { localStorage.setItem('songbird-range', range); } catch { /* private mode */ }
      [...e.target.parentNode.children].forEach((b) => b.classList.toggle('on', b === e.target)); restart(); paint();
    } }, r.label)));
    const sysNote = h('span', { class: 'muted small' });
    const dirSeg = h('div', { class: 'seg' }, ['rx', 'tx'].map((d) => h('button', { class: dir === d ? 'on' : '', onclick: (e) => { dir = d; [...e.target.parentNode.children].forEach((b) => b.classList.toggle('on', b.textContent.startsWith(d === 'rx' ? 'In' : 'Out'))); paint(); } }, d === 'rx' ? 'Inbound' : 'Outbound')));
    mount(root,
      h('div', { class: 'page-head' }, h('div', null, h('h1', null, 'Dashboard'), h('p', null, 'CPU and memory are read from this host; traffic comes from the simulated Data Plane.')), rangeSeg),
      h('div', { class: 'stack' }, kpis,
        h('div', { class: 'grid cols-2' },
          h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'CPU and memory'), h('span', { class: 'spacer' }), sysNote), h('div', { class: 'body' }, sys)),
          h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'Throughput by Interface'), h('span', { class: 'spacer' }), dirSeg), h('div', { class: 'body' }, net))),
        rankings));

    async function paint() {
      const r = RANGES.find((x) => x.id === range) || RANGES[1];
      const [hist, ev] = await Promise.all([api('GET', `/api/v1/metrics?range=${r.id}`), api('GET', '/api/v1/events?n=0')]).catch(() => [null, null]);
      if (!hist) return;
      const samples = hist.samples;
      const step = hist.stepMs >= 3600000 ? `${hist.stepMs / 3600000} h` : hist.stepMs >= 60000 ? `${hist.stepMs / 60000} min` : `${hist.stepMs / 1000} s`;
      sysNote.textContent = `one point every ${step}`;
      const partial = hist.since > hist.from;
      const axis = { from: hist.from, to: hist.to, tickFmt: (d) => d.toLocaleString(undefined, r.tick), note: partial ? 'service started' : null };
      state.counters = ev.counters;
      const s = state.status || {};
      const c = ev.counters;
      const total = c.accepted + c.dropped;
      const stateCls = { 'in-sync': 'good', failed: 'bad', degraded: 'bad' }[s.state];
      mount(kpis,
        kpi('Reconciler', h('span', { class: 'row' }, h('span', { class: ['badge', stateCls || 'warn'], style: { fontSize: '14px' } }, s.state || '…')), s.appliedCommit ? `running ${short(s.appliedCommit)}` : '', 'sm'),
        kpi('Policies', fmtNum(objs('RuleSet').reduce((n, rs) => n + (rs.spec.rules || []).filter((p) => p.enabled !== false).length, 0)), `${objs('RuleSet').length} RuleSets · ${objs('Zone').length} Zones · ${objs('Interface').length} Interfaces`),
        kpi('Flows accepted', fmtNum(c.accepted), total ? `${((c.accepted / total) * 100).toFixed(1)} % of flows` : ''),
        kpi('Flows dropped', fmtNum(c.dropped), `${fmtNum((c.reasons['default-deny'] || 0) + (c.reasons['no-ruleset'] || 0))} by Default Deny`),
        kpi('Events lost', fmtNum(c.eventsLost), 'rate-limited drop events (R-026.1)'));

      const xs = samples.map((x) => x.t);
      mount(sys, lineChart({ xs, series: [{ name: 'CPU', values: samples.map((x) => x.cpu) }, { name: 'Memory', values: samples.map((x) => x.mem) }], yMax: 100, unit: ' %', ...axis }));
      const ifaces = Object.keys(samples[samples.length - 1]?.rx || {}).slice(0, 4);
      mount(net, lineChart({ xs, series: ifaces.map((n) => ({ name: n, values: samples.map((x) => (x[dir]?.[n] || 0)) })), fmt: (v) => fmtBps(v).replace(/\.0 /, ' '), ...axis }));

      const hits = Object.entries(c.hits).sort((a, b) => b[1] - a[1]).slice(0, 8).map(([k, v]) => ({ label: k, value: v }));
      const reasons = Object.entries(c.reasons).sort((a, b) => b[1] - a[1]).map(([k, v]) => ({ label: k, value: v }));
      mount(rankings,
        h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'Top Policies by flows (RuleSet / Policy)'), h('span', { class: 'spacer' }), h('a', { href: '#/policies', class: 'small' }, 'Policies')),
          h('div', { class: 'body' }, hits.length ? bars(hits, fmtNum) : h('div', { class: 'empty' }, 'No traffic yet'))),
        h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'Verdicts by reason'), h('span', { class: 'spacer' }), h('a', { href: '#/traffic', class: 'small' }, 'Live traffic')),
          h('div', { class: 'body' }, reasons.length ? bars(reasons, fmtNum) : h('div', { class: 'empty' }, 'No traffic yet'))));
    }
    let t;
    const restart = () => { clearInterval(t); t = setInterval(() => { if (!document.querySelector('.overlay')) paint(); }, (RANGES.find((x) => x.id === range) || RANGES[1]).every); };
    paint(); restart();
    return () => clearInterval(t);
  },
};
