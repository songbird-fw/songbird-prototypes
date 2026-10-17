// Dashboard: state of the pipeline at a glance and real-time charts (R-027).
import { state, h, mount, api, short, fmtNum, fmtBps, objs } from '../core.js';
import { lineChart, bars } from '../chart.js';

function kpi(label, value, note, cls) {
  return h('div', { class: 'card kpi' }, h('div', { class: 'label' }, label), h('div', { class: ['value', cls] }, value), note && h('div', { class: 'note' }, note));
}

export default {
  render(root) {
    const kpis = h('div', { class: 'grid kpi' });
    const sys = h('div');
    const net = h('div');
    const rankings = h('div', { class: 'grid cols-2' });
    let dir = 'rx';
    const dirSeg = h('div', { class: 'seg' }, ['rx', 'tx'].map((d) => h('button', { class: dir === d ? 'on' : '', onclick: (e) => { dir = d; [...e.target.parentNode.children].forEach((b) => b.classList.toggle('on', b.textContent.startsWith(d === 'rx' ? 'In' : 'Out'))); paint(); } }, d === 'rx' ? 'Inbound' : 'Outbound')));
    mount(root,
      h('div', { class: 'page-head' }, h('div', null, h('h1', null, 'Dashboard'), h('p', null, 'CPU and memory are read from this host; traffic comes from the simulated Data Plane.'))),
      h('div', { class: 'stack' }, kpis,
        h('div', { class: 'grid cols-2' },
          h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'CPU and memory'), h('span', { class: 'spacer' }), h('span', { class: 'muted small' }, 'last 5 min, every 2 s')), h('div', { class: 'body' }, sys)),
          h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'Throughput by Interface'), h('span', { class: 'spacer' }), dirSeg), h('div', { class: 'body' }, net))),
        rankings));

    async function paint() {
      const [samples, ev] = await Promise.all([api('GET', '/api/v1/metrics'), api('GET', '/api/v1/events?n=0')]).catch(() => [null, null]);
      if (!samples) return;
      state.counters = ev.counters;
      const s = state.status || {};
      const c = ev.counters;
      const total = c.accepted + c.dropped;
      const stateCls = { 'in-sync': 'good', failed: 'bad', degraded: 'bad' }[s.state];
      mount(kpis,
        kpi('Reconciler', h('span', { class: 'row' }, h('span', { class: ['badge', stateCls || 'warn'], style: { fontSize: '14px' } }, s.state || '…')), s.appliedCommit ? `running ${short(s.appliedCommit)}` : '', 'sm'),
        kpi('Policies', fmtNum(objs('Zone').reduce((n, z) => n + (z.spec.policies || []).filter((p) => p.enabled !== false).length, 0)), `${objs('Zone').length} zones · ${objs('Interface').length} interfaces`),
        kpi('Flows accepted', fmtNum(c.accepted), total ? `${((c.accepted / total) * 100).toFixed(1)} % of flows` : ''),
        kpi('Flows dropped', fmtNum(c.dropped), `${fmtNum(c.reasons['default-deny'] || 0)} by Default Deny`),
        kpi('Events lost', fmtNum(c.eventsLost), 'rate-limited drop events (R-026.1)'));

      const xs = samples.map((x) => x.t);
      mount(sys, lineChart({ xs, series: [{ name: 'CPU', values: samples.map((x) => x.cpu) }, { name: 'Memory', values: samples.map((x) => x.mem) }], yMax: 100, unit: ' %' }));
      const ifaces = objs('Interface').map((i) => i.metadata.name).slice(0, 4);
      mount(net, lineChart({ xs, series: ifaces.map((n) => ({ name: n, values: samples.map((x) => (x[dir]?.[n] || 0)) })), fmt: (v) => fmtBps(v).replace(/\.0 /, ' ') }));

      const hits = Object.entries(c.hits).sort((a, b) => b[1] - a[1]).slice(0, 8).map(([k, v]) => ({ label: k, value: v }));
      const reasons = Object.entries(c.reasons).sort((a, b) => b[1] - a[1]).map(([k, v]) => ({ label: k, value: v }));
      mount(rankings,
        h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'Top policies by flows'), h('span', { class: 'spacer' }), h('a', { href: '#/policies', class: 'small' }, 'Policies')),
          h('div', { class: 'body' }, hits.length ? bars(hits, fmtNum) : h('div', { class: 'empty' }, 'No traffic yet'))),
        h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, 'Verdicts by reason'), h('span', { class: 'spacer' }), h('a', { href: '#/traffic', class: 'small' }, 'Live traffic')),
          h('div', { class: 'body' }, reasons.length ? bars(reasons, fmtNum) : h('div', { class: 'empty' }, 'No traffic yet'))));
    }
    paint();
    const t = setInterval(() => { if (!document.querySelector('.overlay')) paint(); }, 2000);
    return () => clearInterval(t);
  },
};
