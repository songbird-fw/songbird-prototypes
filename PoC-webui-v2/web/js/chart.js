// Minimal SVG line chart: one y-axis, 2px lines, legend + direct end
// labels for <= 4 series, crosshair tooltip on hover (R-027).
import { h } from './core.js';

const SERIES = ['var(--s1)', 'var(--s2)', 'var(--s3)', 'var(--s4)'];

// series: [{ name, values: [number] }], xs: [ms timestamps]
export function lineChart({ xs, series, height = 180, yMax, fmt = (v) => v.toFixed(0), unit = '' }) {
  const wrap = h('div', { class: 'chart' });
  const legend = series.length > 1 ? h('div', { class: 'legend' }, series.map((s, i) =>
    h('span', { class: 'k' }, h('span', { class: 'sw', style: { background: SERIES[i % 4] } }), s.name))) : null;
  if (legend) wrap.append(legend);
  const tip = h('div', { class: 'tip hidden' });
  const host = h('div', { style: { position: 'relative' } });
  wrap.append(host);

  const draw = () => {
    const W = Math.max(280, host.clientWidth || 600), H = height;
    const padL = 54, padR = series.length > 1 ? 70 : 12, padT = 8, padB = 22;
    const iw = W - padL - padR, ih = H - padT - padB;
    const all = series.flatMap((s) => s.values);
    let max = yMax ?? Math.max(1, ...all) * 1.15;
    const nice = niceMax(max); max = yMax ?? nice;
    const n = xs.length;
    const x = (i) => padL + (n <= 1 ? 0 : (i / (n - 1)) * iw);
    const y = (v) => padT + ih - (v / max) * ih;
    const svg = h('svg', { viewBox: `0 0 ${W} ${H}`, height: H, role: 'img', 'aria-label': series.map((s) => s.name).join(', ') });
    for (let k = 0; k <= 4; k++) {
      const v = (max / 4) * k;
      svg.append(h('line', { svg: true, class: 'grid-line', x1: padL, x2: W - padR, y1: y(v), y2: y(v) }));
      svg.append(h('text', { svg: true, class: 'axis-label', x: padL - 8, y: y(v) + 4, 'text-anchor': 'end' }, fmt(v) + unit));
    }
    if (n > 1) {
      const t0 = new Date(xs[0]), t1 = new Date(xs[n - 1]);
      const tf = (d) => d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
      svg.append(h('text', { svg: true, class: 'axis-label', x: padL, y: H - 4 }, tf(t0)));
      svg.append(h('text', { svg: true, class: 'axis-label', x: W - padR, y: H - 4, 'text-anchor': 'end' }, tf(t1)));
    }
    const ends = [];
    series.forEach((s, i) => {
      if (!s.values.length) return;
      const d = s.values.map((v, k) => `${k ? 'L' : 'M'}${x(k).toFixed(1)},${y(v).toFixed(1)}`).join('');
      svg.append(h('path', { svg: true, class: 'series', d, stroke: SERIES[i % 4] }));
      ends.push({ y: y(s.values[s.values.length - 1]), name: s.name, i });
    });
    // direct labels at line ends, nudged apart to avoid collisions
    if (series.length > 1 && series.length <= 4) {
      ends.sort((a, b) => a.y - b.y);
      for (let k = 1; k < ends.length; k++) if (ends[k].y - ends[k - 1].y < 13) ends[k].y = ends[k - 1].y + 13;
      for (const e of ends) svg.append(h('text', { svg: true, class: 'axis-label', x: W - padR + 6, y: e.y + 4, style: { fill: 'var(--text-2)' } }, e.name));
    }
    const cross = h('line', { svg: true, class: 'cross hidden', y1: padT, y2: padT + ih });
    const marks = series.map((_, i) => h('circle', { svg: true, r: 4, fill: SERIES[i % 4], stroke: 'var(--surface)', 'stroke-width': 2, class: 'hidden' }));
    svg.append(cross, ...marks);
    const hit = h('rect', { svg: true, x: padL, y: padT, width: iw, height: ih, fill: 'transparent' });
    hit.addEventListener('mousemove', (ev) => {
      const r = svg.getBoundingClientRect();
      const px = ((ev.clientX - r.left) / r.width) * W;
      const k = Math.max(0, Math.min(n - 1, Math.round(((px - padL) / iw) * (n - 1))));
      cross.setAttribute('x1', x(k)); cross.setAttribute('x2', x(k)); cross.classList.remove('hidden');
      marks.forEach((mk, i) => { const v = series[i].values[k]; if (v == null) return; mk.setAttribute('cx', x(k)); mk.setAttribute('cy', y(v)); mk.classList.remove('hidden'); });
      tip.replaceChildren(h('div', { class: 'muted' }, new Date(xs[k]).toLocaleTimeString()),
        ...series.map((s, i) => h('div', { class: 'r' }, h('span', { class: 'dot', style: { background: SERIES[i % 4] } }), s.name, h('b', null, fmt(s.values[k] ?? 0) + unit))));
      tip.classList.remove('hidden');
      const left = (x(k) / W) * r.width;
      tip.style.left = `${Math.min(left + 12, r.width - tip.offsetWidth - 4)}px`;
      tip.style.top = '4px';
    });
    hit.addEventListener('mouseleave', () => { cross.classList.add('hidden'); marks.forEach((mk) => mk.classList.add('hidden')); tip.classList.add('hidden'); });
    svg.append(hit);
    host.replaceChildren(svg, tip);
  };
  requestAnimationFrame(draw);
  return wrap;
}

function niceMax(v) {
  const p = Math.pow(10, Math.floor(Math.log10(v)));
  for (const m of [1, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10]) if (m * p >= v) return m * p;
  return 10 * p;
}

// Horizontal bars for ranked counts (one hue: magnitude, not identity).
export function bars(rows, fmt = (v) => v) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  return h('div', { class: 'bars' }, rows.map((r) => h('div', { class: 'bar-row', title: `${r.label}: ${fmt(r.value)}` },
    h('span', { style: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, r.label),
    h('div', { class: 'track' }, h('div', { class: 'fill', style: { width: `${(r.value / max) * 100}%` } })),
    h('span', { class: 'v' }, fmt(r.value)))));
}
