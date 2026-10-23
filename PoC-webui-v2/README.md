# PoC — Web UI v2 (schema v1alpha1, domain model 0.7)

| | |
|---|---|
| Question | Does the Control Plane designed in `songbird-fw/docs/architecture` work end to end behind a real UI, on the configuration format of `songbird-fw/schema`? |
| Status | **Active** |
| Based on | `songbird-fw` at commit `99bfafe`: schema v1alpha1 (`schema/`, decisions of 2026-10-19), domain model 0.7-draft, RA 2.0.6-draft, components v0.1, reconciliation v0.1, ADR-0001, ADR-0006, ADR-0010 |
| Result | Yes for the pipeline commit → validate (schema + semantic rules) → compile → apply (simulated drivers) → status, with the v1alpha1 files as the only store, in the canonical export layout. Every file the PoC writes passes the schema's own `validate.py`; the Go validator gives the same rule id as `validate.py` on all 48 invalid cases of the corpus. Seven points for the schema, the domain model and the RA surfaced: see [Findings](#findings). |
| Decisions | Feeds the schema/domain-model alignment (findings 1–5) and RA open issue #8 (finding 6). The findings of the previous version are closed by the 2026-10-19 decisions, see [Closed findings](#closed-findings). |

A proof of concept of the Songbird Web UI **and of the Control Plane slice behind it**. The configuration is
the v1alpha1 format of `songbird-fw/schema`, unchanged: `apiVersion: config.songbird-fw.com/v1alpha1`,
lowercase DNS-label names, Zones as sets of Interfaces, one `RuleSet` per `(from, to)` pair with member
Interfaces usable next to their Zone (evaluated first, DM-42), NAT as a section of the rule (`snat` / `dnat`,
matched before translation), flat groups, `any` / `any6` / Service `any` as predefined objects, `Color` objects
referenced by name, and the `System` singleton with its System Services (DNS, NTP and the DHCP pools).

```
Web UI ──REST/SSE──▶ API Server ──▶ Config Manager (v1alpha1 YAML in a local Git repo, 1 commit per save)
                         │ dry run          │ new commit
                         ▼                  ▼
                     Validator ◀──── Reconciler: read HEAD → Validate → Compile → Diff → Apply (atomic) → Status
              1. JSON Schema v1alpha1                                                 │
              2. semantic rules (validate.py ids + DM-xx)     simulated drivers: network, dataplane, dhcp, dns
```

One static Go binary (C-004), frontend, fonts and schema embedded, no Node build step, no database.

## Run

Requirements: Go 1.23+ and `git` on the PATH.

```bash
cd PoC-webui-v2
go run . -listen 127.0.0.1:8080 -data ./data
# or: CGO_ENABLED=0 go build -o songbird-webui . && ./songbird-webui
```

Open http://127.0.0.1:8080 and sign in as **admin / songbird** (admin) or **auditor / auditor** (read-only).
Without a local Go toolchain:

```bash
podman run --rm -v "$PWD":/src:Z -w /src docker.io/library/golang:1.23 \
  sh -c 'go test ./... && CGO_ENABLED=0 go build -o songbird-webui .'
```

On first start the factory default (`internal/defaults/factory/`, a small office: WAN, an 802.1Q trunk with office,
server, guest and DMZ VLANs, three Zones, eight RuleSets, three DHCP pools) is committed to `./data/config`, a
normal Git repository in the canonical export layout (`system.yaml`, `interfaces/`, `zones/`, `addresses/`, …):
try `git -C data/config log -p`. Delete `./data` to start over; a `./data` written by the previous version of the
PoC (old layout and schema) is refused at start with that hint. Use `-tls-cert/-tls-key` for HTTPS.

The same binary is also a prototype of `songbird validate` (R-052), with the rule ids of `validate.py`:

```bash
go run . validate ../../songbird-fw/schema/examples/valid        # any layout, multi-document files
go run . validate -appliance ./data/config                       # also requires the predefined objects and an admin User
```

## What to try

| Try this | What it shows |
|---|---|
| Hover the icons of the left rail | Labels appear on hover and on keyboard focus |
| Look at any object name | Its Color tints the name itself, as in FortiGate; the meaning is in the tooltip |
| Policies → right click a Policy row | Every row action is in the context menu: edit, enable/disable, duplicate, move (top, up, down, bottom), new Policy above, YAML, delete. Rows have no buttons. Keyboard: focus a row, `Shift+F10` or the menu key; `Enter` edits, `Alt+↑/↓` moves, `Del` deletes. On a touch screen a tap opens the menu |
| Policies, line under the title | `lan2 → wan` is checked against `lan2-to-untrusted`, then `internal-to-untrusted` (DM-42, R-046.3); the RuleSet headers say *before* / *after* |
| Live traffic, filter `printers`-range sources | A printer is dropped by `lan2-to-untrusted`; another lan2 host falls through to `internal-to-untrusted` and is accepted |
| `+ Policy` and leave Source empty | Save is disabled and the field is highlighted (R-009.1) |
| Pick a source in `internal-to-untrusted` | `any` / `any6` are predefined Addresses, listed first; Addresses bound elsewhere are not offered (DM-12) |
| `+ RuleSet` from `internal` to `internal` | Refused: from and to must be different objects (DM-06). `guest → wan` is accepted: a member next to its Zone (DM-38) |
| Edit `publish-web` (untrusted → dmz-zone) | A `dnat` rule: destination `wan-ip` is the public address **before** translation (ADR-0010); translate to `web-01` / `https-8443` |
| Set its destination to `offsite-backup` | Saved, with a DM-40 **warning**: warnings never block a save or an Apply |
| Network → `wan` → tick `https` | Refused by DM-35: `publish-web` translates the management port of an Interface that allows https |
| Objects → new Address `example.com` (fqdn), use it in an enabled Policy | The Address is accepted; the Policy is refused until it is disabled (DM-43, R-012.4) |
| Objects → Address group, add a group as member | Not offered: groups are flat (DM-15); in YAML it is `wrong-kind` |
| Objects → Service `any` or Address `any` | Predefined: only the comment is editable, delete is disabled (DM-21) |
| System → *Edit settings* → remove `guest` from *DNS listens on* | Refused by R-017.6: the `guest` pool advertises the firewall as DNS server, which would not answer there |
| Network → Interfaces | No role column: access to the firewall is the management access (https, ping, ssh) plus the System Services listening there (DM-34) |
| Any form → *Edit YAML* | The file itself, checked live: schema errors and semantic errors with **file:line** and path (R-020.3) |
| Objects → new Address `10.0.0.0/255.0.0.0` | Schema error, live: CIDR only (R-012) |
| Delete `web-01` (used by a group and a dnat rule) | Rejected with DM-02, file, line and path |
| Repaint `green`; change the Address `any` | Rejected (`predefined-hex`, `predefined-immutable`). The comment of a predefined object stays editable |
| Change only a comment or a color | Committed, and the Reconciler reports *no-op*: metadata never reaches a driver |
| Changes → *Import…* the schema's `examples/valid` (all files concatenated) | Accepted in one commit, multi-document file included; the repository keeps one object per file |
| Changes → *Export* | The repository as tar.gz (R-020.1); it reads back with 0 errors |
| Diagnostics → *Fail next apply* on Data Plane, then change a Policy | Transaction rolled back in reverse order, state *failed*, banner "desired ≠ applied" (R-020.6) |
| Changes → pick an old commit → *Restore* | Rollback is a new commit (components.md) |
| Edit the same object in two tabs, save both | Second save gets 409: optimistic locking on the revision, file-level merge (R-021.5) |
| Dashboard → *Real-time · 5 min · 1 h · 1 day · 1 week* | Raw samples every 2 s for 5 minutes, then averages per 30 s, 10 min and 1 h (`GET /api/v1/metrics?range=…`); history is in memory since the service started (R-027) |
| Live traffic | SSE stream of FLOW_NEW/DROP/END with RuleSet / Policy, DNAT and SNAT translations and the reasons of `dataplane.md` §11 (R-026, R-028, R-028.1) |
| Wait for the periodic port-scan burst | Drop events are rate-limited and `events_lost` grows (R-026.1) |
| System → *Edit settings* → session timeout `1h` | The next login gets a 1 h session: Auth reads `System.admin` (R-043.4, R-043.5, R-043.2) |
| Sign in as auditor | Everything visible, every write is 403 (R-043.9); the context menus offer *View* only |
| Audit log → *Verify chain* | SHA-256 hash chain (R-035.1); edit `data/audit.log` by hand and verify again |
| Sun / moon icon in the top bar | Dark theme (R-044, R-044.1); the default follows `System.admin.ui.theme` |

## Style

A clean technical drawing on white: graphite text, hairline rules in cool grey, one drafting-blue accent
reserved for the active state and focus, no gradients or decoration. **Instrument Sans** for all text, in
sentence case, with hierarchy carried by size and weight (the page title is the one large element);
**IBM Plex Mono** only for data such as addresses, ports and revisions. The fonts are served by the binary
(an appliance has no CDN), SIL OFL 1.1 (`web/fonts/`). The left rail shows icons only; labels appear on hover
or focus. Light and dark themes share the same tokens (`web/style.css`); the dark theme uses the Nord palette
(Polar Night grounds, Snow Storm text, Frost accent and primary buttons, Aurora states).

- **Colors.** As in FortiGate, an object's Color tints its name, not a swatch before it. Only Addresses,
  Services and their groups are coloured; Interfaces, Zones and RuleSets are not.
- **Kind icons.** A small icon before each reference tells the kind: one circle = Address, three circles =
  AddressGroup, one square = Service, two squares = ServiceGroup.
- **Policies.** RuleSets are sections separated by space, not boxes. In Policy rows, objects are plain coloured
  text, one per line; RuleSet comments are a tooltip on the name; the per-Policy `log` appears under the name only
  when it is not the default.

## Layout

| Path | Component (components.md) |
|---|---|
| `internal/model` | Go types of every v1alpha1 kind, one field per schema property; canonical YAML writer (schema decision 18); DM-42 evaluation order |
| `internal/schema` | Validator layer 1: `v1alpha1/` is a verbatim copy of `songbird-fw/schema` (`99bfafe`); `ext/` the PoC proposals (StaticRoute, User) |
| `internal/validator` | Validator layer 2 — pure; `validate.py` rule ids + DM-xx; loader for any layout; tests on the schema corpus |
| `internal/predefined` | Shipped objects (DM-21): the 16-colour palette, Addresses `any` / `any6`, Service `any`; embedded YAML |
| `internal/configstore` | Config Manager — Git repo, one file per object, commits, optimistic locking, uuids, history, revert, archive |
| `internal/reconciler` | Reconciler + Applier — level-triggered loop, ordered stages, rollback, last known good; simulated Compiler |
| `internal/events` | Simulated Data Plane with a first-match reference classifier (DNAT before translation) + Event Collector |
| `internal/observer` | Observer — `/health` (R-032), CPU/memory from `/proc`, simulated throughput, in-memory history for the dashboard ranges |
| `internal/auth`, `internal/audit` | Auth (bcrypt cost 12, signed sessions, lockout from `System.admin`); Audit Log (hash-chained JSON lines) |
| `internal/api` | API Server — REST + SSE; never touches drivers |
| `web/` | Web UI — vanilla ES modules, no build step; fonts in `web/fonts` |
| `testdata/schema-examples` | Copy of `songbird-fw/schema/examples` (57 valid objects, 48 invalid cases): the test corpus of the Validator (R-020.2) |

Repository layout: the canonical export layout of the schema (design decision 14), one object per file, file name
= `metadata.name`; any layout is accepted on import.

```
system.yaml   colors/   interfaces/   zones/   addresses/   address-groups/   services/   service-groups/   rulesets/
static-routes/*   users/*                                                                   (* PoC extension kinds)
```

### REST API (v1)

`POST /api/v1/login|logout`, `GET /api/v1/me`, `GET /api/v1/config` (whole tree, Validator findings, `ETag`),
`PUT|DELETE /api/v1/objects/{kind}/{name}` (`If-Match: "<revision>"` required, `?create=1`; JSON or
`Content-Type: application/yaml`), `POST /api/v1/validate/{kind}/{name}` (dry run), `GET /api/v1/objects/{kind}/{name}/yaml`,
`PUT /api/v1/users/{name}`, `GET /api/v1/export` (tar.gz), `POST /api/v1/import` (YAML, whole configuration),
`GET /api/v1/schema/{file}`, `GET /api/v1/status`, `GET /api/v1/history`, `GET /api/v1/history/{rev}/diff`,
`POST /api/v1/history/{rev}/revert`, `GET /api/v1/events`, `GET /api/v1/events/stream` (SSE), `GET /api/v1/metrics?range=rt|5m|1h|1d|1w`,
`GET /api/v1/audit`, `GET /api/v1/audit/verify`, `POST /api/v1/diagnostics/faults` (PoC only), `GET /health` (no auth).
Writes need `X-Requested-With: songbird` (CSRF). 422 responses carry
`validation: [{file, line, path, reason, rule, constraint, severity}]`; successful saves carry the file's `warnings`.

## Validation

| Layer | What | Where |
|---|---|---|
| 1. structural | every document against `songbird.schema.json` (or an `ext/` schema), before it is typed: unknown fields, types, patterns, `if/then` rules, `ENC[...]` secrets | `internal/schema` (santhosh-tekuri/jsonschema, as the schema README prescribes) |
| 2. semantic | all the rules of `validate.py` (`unique-name`, `dangling-ref`, `wrong-kind` (flat groups), `duplicate-hex`, `predefined-*`, `any-protocol`, `vlan-parent`, `zone-overlap`, `same-interface`, `duplicate-pair`, `dnat-*`, `unsupported-address-kind`, `mac-as-destination`, `system-singleton`, `dns64-without-nat64`, `dhcp-*`, `dhcp-advertises-silent-service`, …) | `internal/validator` |
| 2. semantic | the domain-model constraints the schema does not cover: DM-03/04 addressing, DM-06 (also Zone → same Zone), DM-12 binding, DM-17 port ranges and duplicate entries, DM-21 presence of the predefined objects, DM-22/23 routes, DM-28 DHCP overlap, DM-35 management port, DM-36 on the effective lists of DM-42, DM-40 / DM-41 DNAT overlap and shadowing, DM-45 `listenOn` | `internal/validator` |
| warnings | IPv6, NAT64 (deferred, R-057), host bits in a subnet, DM-40 destination, DM-41 shadowing, `schedule` | never block a save or an Apply |

On a plain tree (`songbird validate` without `-appliance`) the predefined Addresses and Service are injected
when missing, like `validate.py`; on the appliance they must be present.

Tests: the 57-object example and the 48 invalid cases of the schema, each expected with the rule id that
`validate.py` reports (`internal/validator/corpus_test.go`), one mutation per constraint on the factory default
(`validator_test.go`), the HTTP flows (`internal/api`), the reconciliation loop, and the simulated flows against
the classifier, including the DM-42 order (`internal/events`).

## Simulated or missing

- **No kernel work.** Drivers only record the hash of their Plan section; the Compiler builds the effective list
  of every Interface pair (DM-42), expands groups and hashes the result. Traffic is generated from the applied configuration and classified by a reference
  first-match classifier; throughput is synthetic.
- Not implemented: IPv6 (deferred in the domain model: kept in the files, not applied), Schedule kind, setup wizard (R-031),
  TOTP (R-043.6), custom Roles, GitOps push/pull/conflict UI (R-040–R-042), alerting, address pseudonymisation
  in the log (R-037: configured, not applied), audit-log encryption at rest, secrets encryption (ADR-0009), schema
  migration (ADR-0008), renames (references are by name; the uuid would allow them).
- Live Interface events (R-015.1) need the Network Driver; the tree shows configured Interfaces only.

## Findings

Against `songbird-fw` at `99bfafe`.

1. **DM-16 contradicts the schema and the Zones.** DM-16 requires all members of an AddressGroup to be bound to
   the same Interface; the schema has no `AddressGroup.interface`, and its valid example `grp-lan` mixes `lan1`
   and `lan2` members and is used from the Zone `internal`, which DM-12 allows. The PoC does not enforce DM-16
   and applies DM-12 to each expanded member. Proposal: withdraw DM-16 (or restate it per Zone).
2. **Predefined Services.** DM-21 lists `http`, `https`, `dns`, … as predefined Services; the schema allows
   `predefined: true` only on the Service `any` (`predefined-unknown`). The PoC follows the schema: the common
   Services ship as ordinary, editable objects. One of the two has to change.
3. **`validate.py` allows a RuleSet from a Zone to the same Zone** (`same-interface` checks Interfaces only);
   DM-06 and R-014.5 forbid it. Already listed in the domain model's *Schema alignment*; the PoC rejects it as
   `same-zone`.
4. **Reserved names.** The schema reserves only `none`; the domain model (0.7, *Names*) reserves `all` and
   `none`. The PoC follows the schema.
5. **DM-36 per effective list.** With member Interfaces next to their Zone, the limit has to hold on the
   effective list of each Interface pair (DM-42), not per RuleSet: a 600-rule Zone RuleSet plus a 600-rule member
   RuleSet is fine per file and over the limit for the shared pairs. The PoC checks the effective lists and
   reports the most specific RuleSet; ADR-0006 and DM-36 could say so explicitly.
6. **Policy `log`.** The RA (note v2.0.3, open issue #8) and the domain model remove the per-Policy log setting;
   the schema still has `log` (`all`, `none`, default `security`). The PoC stores it and the simulated Data Plane
   ignores it.
7. **R-046.3 in the editor.** The PoC shows the evaluation order on the Policies page and in each RuleSet header;
   R-046.3 also asks for it "when a RuleSet is created or edited". A preview inside the RuleSet form is still
   missing.

### Closed findings

The ten findings of the previous version (schema `55e63ac`) were settled by the team decisions of 2026-10-19:
member Interfaces next to their Zone (was 1, DM-38), `predefined` on Service (3), DHCP as a System Service (4),
one object per file as the export layout (6), Interface `role` removed (7), flat groups (8), DHCP overlap implied
(9, kept as a check), RA open issues #8 and #10 decided (10). Finding 5 (ECMAScript lookaheads) still holds for
any Go validator: the PoC uses `regexp2` in ECMAScript mode. Finding 2 is finding 3 above.
