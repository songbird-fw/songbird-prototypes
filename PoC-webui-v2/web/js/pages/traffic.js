// Live Traffic Log (R-028) with filters that apply without reload (R-028.1).
// Events arrive as server-sent events relayed from the Event Collector.
import { state, h, icon, mount, api, fmtNum, fmtBytes } from '../core.js';

const MAX = 3000;
let buf = [];          // newest last
let paused = false;
let pending = [];      // received while paused
const f = { verdict: '', type: '', reason: '', src: '', dst: '', sport: '', dport: '', zone: '' };

function match(e) {
  if (f.verdict && e.verdict !== f.verdict) return false;
  if (f.type && e.type !== f.type) return false;
  if (f.reason && e.reason !== f.reason) return false;
  if (f.src && !e.src.startsWith(f.src)) return false;
  if (f.dst && !e.dst.startsWith(f.dst)) return false;
  if (f.sport && String(e.sport) !== f.sport.trim()) return false;
  if (f.dport && String(e.dport) !== f.dport.trim()) return false;
  if (f.zone && !(`${e.zone || ''}/${e.policy || ''}`).toLowerCase().includes(f.zone.toLowerCase())) return false;
  return true;
}

function evRow(e) {
  const t = new Date(e.ts);
  return h('tr', null,
    h('td', { class: 'mono small' }, t.toLocaleTimeString(undefined, { hour12: false }) + '.' + String(t.getMilliseconds()).padStart(3, '0')),
    h('td', null, e.verdict === 'accept' ? h('span', { class: 'badge good' }, 'accept') : h('span', { class: 'badge bad' }, 'drop')),
    h('td', { class: 'small' }, e.type.replace('FLOW_', '').toLowerCase()),
    h('td', { class: 'small' }, e.reason),
    h('td', { class: 'small' }, e.proto),
    h('td', { class: 'mono' }, e.src, e.sport ? h('span', { class: 'muted' }, ':' + e.sport) : ''),
    h('td', { class: 'mono' }, e.dst, e.dport ? h('span', { class: 'muted' }, ':' + e.dport) : ''),
    h('td', { class: 'small' }, e.inIf),
    h('td', { class: 'small' }, e.zone ? h('span', null, e.zone, e.policy ? h('span', { class: 'muted' }, ' / ') : '', e.policy || h('span', { class: 'muted' }, ' / default deny')) : h('span', { class: 'muted' }, '—')),
    h('td', { class: 'num small' }, e.type === 'FLOW_END' ? `${fmtNum(e.packets)} pkt · ${fmtBytes(e.bytes)}` : ''));
}

export default {
  render(root) {
    const tbody = h('tbody');
    const info = h('span', { class: 'muted small' });
    const lost = h('span', { class: 'small' });
    const pauseBtn = h('button', { onclick: () => { paused = !paused; if (!paused) { buf.push(...pending); pending = []; trim(); } paintPause(); paint(); } });
    const paintPause = () => mount(pauseBtn, icon(paused ? 'play' : 'pause'), paused ? `Resume (${pending.length})` : 'Pause');
    const trim = () => { if (buf.length > MAX) buf = buf.slice(buf.length - MAX); };

    const input = (k, ph, w = '130px') => h('input', { placeholder: ph, value: f[k], style: { width: w }, 'aria-label': ph, oninput: (e) => { f[k] = e.target.value; paint(); } });
    const select = (k, label, values) => h('select', { style: { width: 'auto' }, 'aria-label': label, onchange: (e) => { f[k] = e.target.value; paint(); } },
      h('option', { value: '' }, label), values.map((v) => h('option', { value: v, selected: f[k] === v }, v)));

    function paint() {
      const shown = [];
      for (let i = buf.length - 1; i >= 0 && shown.length < 300; i--) if (match(buf[i])) shown.push(buf[i]);
      mount(tbody, shown.length ? shown.map(evRow) : h('tr', null, h('td', { colspan: 10, class: 'empty' }, buf.length ? 'No event matches the filters.' : 'Waiting for events…')));
      info.textContent = `showing ${shown.length}${shown.length === 300 ? '+' : ''} of ${fmtNum(buf.length)} buffered events`;
      const c = state.counters;
      mount(lost, c ? h('span', { title: 'Drop events not emitted because of rate limiting; drops are still counted (R-026.1)' },
        'events_lost ', h('strong', null, fmtNum(c.eventsLost))) : null);
    }

    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'Live traffic'), h('p', null, 'One event per Flow: new, dropped, ended (R-026). The Data Plane is simulated from the applied configuration.')),
        h('span', { class: 'spacer' }), lost, pauseBtn, h('button', { onclick: () => { buf = []; pending = []; paint(); } }, 'Clear')),
      h('div', { class: 'card pad', style: { marginBottom: '12px' } }, h('div', { class: 'row wrap' },
        select('verdict', 'Any verdict', ['accept', 'drop']),
        select('type', 'Any event', ['FLOW_NEW', 'FLOW_DROP', 'FLOW_END']),
        select('reason', 'Any reason', ['policy', 'default-deny', 'no-zone', 'local-in', 'local-in-deny', 'malformed', 'fragment', 'ipv6-unsupported']),
        input('src', 'Source address'), input('sport', 'Src port', '90px'),
        input('dst', 'Destination address'), input('dport', 'Dst port', '90px'),
        input('zone', 'Zone / Policy', '150px'),
        h('button', { class: 'ghost', onclick: () => { Object.keys(f).forEach((k) => (f[k] = '')); window.songbirdRerender(); } }, 'Reset'),
        h('span', { class: 'spacer' }), info)),
      h('section', { class: 'card' }, h('div', { class: 'table-wrap' }, h('table', null,
        h('thead', null, h('tr', null, ['Time', 'Verdict', 'Event', 'Reason', 'Proto', 'Source', 'Destination', 'In', 'Zone / Policy', 'Totals'].map((t, k) => h('th', { class: k === 9 && 'num' }, t)))),
        tbody))));
    paintPause();

    let es, raf = 0, closed = false;
    const schedule = () => { if (!raf) raf = setTimeout(() => { raf = 0; paint(); }, 250); };
    (async () => {
      if (!buf.length) { const r = await api('GET', '/api/v1/events?n=500').catch(() => null); if (r) buf = r.events; }
      if (closed) return;
      paint();
      es = new EventSource('/api/v1/events/stream');
      es.onmessage = (m) => { const e = JSON.parse(m.data); if (paused) { if (pending.length < MAX) pending.push(e); } else { buf.push(e); trim(); schedule(); } };
    })();
    return () => { closed = true; es?.close(); clearTimeout(raf); };
  },
  refresh() {},
};
