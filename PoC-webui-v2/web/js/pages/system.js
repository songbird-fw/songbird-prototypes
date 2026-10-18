// System: the singleton `kind: System` (system.schema.json), the global
// settings structured after FortiGate's "config system global / admin / ntp
// / dns / session-ttl" and "config log". The form covers the settings the
// PoC uses; every other field is editable in YAML. Secrets are never shown
// in clear: they are ENC[...] envelopes (R-020.5).
import { h, icon, mount, objs, find, isAdmin, refChip, objectForm, showYAML, yamlEditor, findingBadge } from '../core.js';

const lines = (v) => (v || []).join('\n');
const splitLines = (s) => s.split(/[\n,]+/).map((x) => x.trim()).filter(Boolean);
const netOptions = () => [...objs('Zone').map((z) => ({ value: z.metadata.name, label: `${z.metadata.name} (Zone)`, group: 'Zones' })),
  ...objs('Interface').map((i) => ({ value: i.metadata.name, label: i.metadata.name, group: 'Interfaces' }))];
const addrOptions = () => [...objs('Address').map((a) => ({ value: a.metadata.name, label: a.metadata.name, group: 'Addresses' })),
  ...objs('AddressGroup').map((g) => ({ value: g.metadata.name, label: g.metadata.name, group: 'Address groups' }))];

function editSystem(o) {
  objectForm({
    kind: 'System', obj: o, title: 'System settings',
    intro: h('p', { class: 'sub small', style: { margin: 0 } }, 'The main settings. Certificates, syslog, GitOps, updates and the other sections are edited in YAML.'),
    fields: [
      { key: 'spec.hostname', label: 'Hostname', required: true, placeholder: 'fw01.example.lan' },
      { key: 'spec.timezone', label: 'Time zone', placeholder: 'UTC', help: 'IANA name, e.g. Europe/Rome' },
      { key: 'spec.admin.httpsPort', label: 'Management HTTPS port', type: 'number', placeholder: '443', help: 'A dnat rule cannot translate it on an Interface that allows https (DM-35)' },
      { key: 'spec.admin.sessionTimeout', label: 'UI session timeout', placeholder: '8h', help: 'Default 8h (R-043.5)' },
      { key: 'spec.admin.lockout.threshold', label: 'Lockout after failed logins', type: 'number', placeholder: '5', help: 'Per source address (R-043.2)' },
      { key: 'spec.admin.lockout.duration', label: 'Lockout duration', placeholder: '10m' },
      { key: 'spec.admin.trustedHosts', label: 'Trusted hosts for the UI', type: 'multi', options: addrOptions, placeholder: 'any address…', full: true,
        help: 'Empty = any address on Interfaces that allow https' },
      { key: 'spec.dns.mode', label: 'DNS server mode', type: 'select', options: [{ value: '', label: '— not configured —' }, 'forwarder', 'recursive'], rerender: true, help: 'R-018.1, R-018.3' },
      { key: 'spec.dns.localDomain', label: 'Local domain', show: (s) => !!s.spec.dns?.mode, placeholder: 'example.lan' },
      { key: 'spec.dns.upstreams', label: 'Upstream DNS servers', type: 'textarea', show: (s) => s.spec.dns?.mode === 'forwarder', format: lines, parse: splitLines, required: true,
        help: 'In order: the first is primary; failover after 3 timeouts (R-034)' },
      { key: 'spec.dns.serveToInterfaces', label: 'DNS served to', type: 'multi', show: (s) => !!s.spec.dns?.mode, options: netOptions, placeholder: 'add…', help: 'Opens UDP/TCP 53 there (DM-34)' },
      { key: 'spec.ntp.enabled', label: 'NTP', type: 'checkbox', checkLabel: 'Synchronise the clock', rerender: true },
      { key: 'spec.ntp.servers', label: 'NTP servers', type: 'textarea', show: (s) => s.spec.ntp?.enabled, format: lines, parse: splitLines, placeholder: 'pool.ntp.org' },
      { key: 'spec.ntp.serveToInterfaces', label: 'NTP served to', type: 'multi', show: (s) => s.spec.ntp?.enabled, options: netOptions, placeholder: 'add…' },
      { key: 'spec.logging.rotation.retentionDays', label: 'Traffic Log retention (days)', type: 'number', placeholder: '30', help: 'R-029' },
      { key: 'spec.logging.anonymizeSourceIp.enabled', label: 'Pseudonymise source addresses', type: 'checkbox', checkLabel: 'In the Traffic Log (GDPR, R-037)', rerender: true },
      { key: 'spec.logging.anonymizeSourceIp.interfaces', label: 'Only on', type: 'multi', show: (s) => s.spec.logging?.anonymizeSourceIp?.enabled,
        options: () => objs('Interface').map((i) => i.metadata.name), placeholder: 'everywhere', help: 'Per Interface (R-037.1); empty = everywhere' },
      { key: 'spec.session.onRuleChange', label: 'Flows a new rule no longer allows', type: 'select', options: [{ value: '', label: 'terminate (default)' }, 'terminate', 'expire'], help: 'R-054, ADR-0004' },
      { key: 'spec.session.ipv4NonInitialFragments', label: 'IPv4 non-initial fragments', type: 'select', options: [{ value: '', label: 'drop (default)' }, 'drop', 'track'], help: 'R-053, ADR-0005' },
      { key: 'spec.threatFeeds.enabled', label: 'Threat feeds', type: 'checkbox', checkLabel: 'Drop traffic from and to listed addresses (R-039)' },
    ],
  });
}

const kv = (rows) => h('table', null, h('tbody', null, rows.filter(Boolean).map(([k, v]) => h('tr', null, h('td', { class: 'sub', style: { width: '42%' } }, k), h('td', null, v ?? h('span', { class: 'muted' }, '—'))))));
const yes = (b) => (b ? h('span', { class: 'badge good' }, 'on') : h('span', { class: 'badge' }, 'off'));
const chips = (l) => (l?.length ? h('div', { class: 'row wrap' }, l.map((x) => refChip(x))) : null);
const card = (title, body, note) => h('section', { class: 'card' }, h('div', { class: 'head' }, h('h2', null, title), h('span', { class: 'spacer' }), note && h('span', { class: 'muted small' }, note)), h('div', { class: 'body' }, body));

export default {
  render(root) {
    const o = find('System', 'system');
    if (!o) { mount(root, h('div', { class: 'card empty' }, 'No System object: the configuration is invalid (system-missing).')); return; }
    const s = o.spec;
    const a = s.admin || {};
    mount(root,
      h('div', { class: 'page-head' },
        h('div', null, h('h1', null, 'System'), h('p', null, 'Global settings: one System object, named "system", in system/system.yaml.')),
        h('span', { class: 'spacer' }), findingBadge('System', 'system'),
        h('button', { class: 'ghost', onclick: () => showYAML('System', 'system') }, icon('code'), 'YAML'),
        isAdmin() && h('button', { onclick: () => yamlEditor('System', 'system') }, icon('edit'), 'Edit YAML'),
        isAdmin() && h('button', { class: 'primary', onclick: () => editSystem(o) }, icon('cog'), 'Edit settings')),
      h('div', { class: 'grid cols-2' },
        card('Identity', kv([['Hostname', h('code', null, s.hostname)], ['Time zone', s.timezone || 'UTC'], ['Comment', o.metadata.comment]])),
        card('Management access', kv([
          ['HTTPS port', a.httpsPort || 443], ['Redirect HTTP to HTTPS', yes(a.redirectHttp !== false)],
          ['Session timeout', a.sessionTimeout || '8h'], ['Lockout', `${a.lockout?.threshold || 5} failures → ${a.lockout?.duration || '10m'}`],
          ['Trusted hosts', chips(a.trustedHosts) || 'any address on Interfaces with https'],
          ['Certificate', a.certificate?.mode || 'self-signed'], ['UI theme', a.ui?.theme || 'auto'],
        ]), 'per Interface: Network → allowAccess'),
        card('DNS', s.dns ? kv([
          ['Mode', s.dns.mode], s.dns.mode === 'forwarder' && ['Upstreams', h('span', { class: 'mono' }, (s.dns.upstreams || []).join(', '))],
          ['Local domain', s.dns.localDomain], ['Local records', (s.dns.localRecords || []).length ? h('div', null, s.dns.localRecords.map((r) => h('div', { class: 'mono small' }, `${r.name} → ${r.address}`))) : null],
          ['Served to', chips(s.dns.serveToInterfaces)], s.dns.dns64 && ['DNS64', h('span', { class: 'badge outline' }, 'on (IPv6 deferred)')],
        ]) : h('div', { class: 'muted' }, 'Not configured.'), 'R-018'),
        card('NTP', kv([['Enabled', yes(s.ntp?.enabled)], ['Servers', s.ntp?.servers?.join(', ')], ['Served to', chips(s.ntp?.serveToInterfaces)]])),
        card('Logging', kv([
          ['Retention', `${s.logging?.rotation?.retentionDays || 30} days, ${s.logging?.rotation?.maxFileSizeMb || 100} MB per file`],
          ['Pseudonymise source IP', s.logging?.anonymizeSourceIp?.enabled ? h('span', null, yes(true), ' ', (s.logging.anonymizeSourceIp.interfaces || []).join(', ') || 'everywhere') : yes(false)],
          ['Syslog', s.logging?.syslog?.enabled ? `${s.logging.syslog.server}:${s.logging.syslog.port || 6514} (${s.logging.syslog.transport || 'tls'})` : yes(false)],
        ]), 'R-029, R-037, R-050'),
        card('Sessions', kv([
          ['Max sessions', s.session?.maxSessions || 1000000], ['On rule change', s.session?.onRuleChange || 'terminate'],
          ['IPv4 non-initial fragments', s.session?.ipv4NonInitialFragments || 'drop'],
          ['Timeouts', s.session?.timeouts ? Object.entries(s.session.timeouts).map(([k, v]) => `${k} ${v}`).join(', ') : 'defaults (R-045.2)'],
        ])),
        card('GitOps', kv([['Remote', s.git?.enabled ? h('code', null, s.git.remote) : yes(false)], s.git?.enabled && ['Branch', s.git.branch || 'main'],
          s.git?.enabled && ['Auth', s.git.auth?.type]]), 'push/pull not in the PoC'),
        card('Updates and threat feeds', kv([
          ['Update channel', `${s.updates?.channel || 'stable'}${s.updates?.automatic ? ', automatic ' + (s.updates.window || '') : ', manual'}`],
          ['Threat feeds', s.threatFeeds?.enabled ? h('div', null, (s.threatFeeds.sources || []).map((f) => h('div', { class: 'small' }, f.name, ' ', f.enabled === false ? h('span', { class: 'muted' }, '(off)') : ''))) : yes(false)],
        ]))));
  },
};
