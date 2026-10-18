# PoC — Web UI v2 (songbird-yaml-schema v1alpha1)

| | |
|---|---|
| Question | Does the Control Plane designed in `songbird-architecture` work end to end behind a real UI, on the configuration format of `songbird-yaml-schema`? |
| Status | **Active** |
| Based on | `songbird-yaml-schema` v1alpha1 (commit `55e63ac`), domain model v0.6-draft, RA v2.0.2-draft, components v0.1, reconciliation v0.1, ADR-0001, ADR-0006, ADR-0010 |
| Result | Yes for the pipeline commit → validate (schema + semantic rules) → compile → apply (simulated drivers) → status, with the v1alpha1 files as the only store. Every file the PoC writes passes the schema's own `validate.py`. Ten points for the schema, the domain model and the RA surfaced: see [Findings](#findings). |
| Decisions | Feeds the schema review (findings 1–6), the domain model open questions (7–9) and RA open issues #6, #8, #10. |

A proof of concept of the Songbird Web UI **and of the Control Plane slice behind it**. The configuration is
the v1alpha1 format of `songbird-yaml-schema`, unchanged: `apiVersion: config.songbird-fw.com/v1alpha1`,
lowercase DNS-label names, Zones as sets of Interfaces, one `RuleSet` per `(from, to)` pair, NAT as a section of
the rule (`snat` / `dnat`, matched before translation), `Color` objects referenced by name, the `System` singleton.

```
Web UI ──REST/SSE──▶ API Server ──▶ Config Manager (v1alpha1 YAML in a local Git repo, 1 commit per save)
                         │ dry run          │ new commit
                         ▼                  ▼
                     Validator ◀──── Reconciler: read HEAD → Validate → Compile → Diff → Apply (atomic) → Status
              1. JSON Schema v1alpha1                                                 │
              2. semantic rules (validate.py ids + DM-xx)     simulated drivers: network, dataplane, dhcp, dns
```

One static Go binary (C-004), frontend and schema embedded, no Node build step, no database.

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
server, guest and DMZ VLANs, three Zones, seven RuleSets) is committed to `./data/config`, a normal Git repository:
try `git -C data/config log -p`. Delete `./data` to start over. Use `-tls-cert/-tls-key` for HTTPS.

The same binary is also a prototype of `songbird validate` (R-052), with the rule ids of `validate.py`:

```bash
go run . validate ../../songbird-yaml-schema/examples/valid      # any layout, multi-document files
go run . validate -appliance ./data/config                       # also requires an admin User
```

## What to try

| Try this | What it shows |
|---|---|
| Policies → `+ Policy` and leave Source empty | Save is disabled and the field is highlighted (R-009.1) |
| Pick a source in `internal-to-untrusted` | Only Addresses bound to `lan1`/`lan2` (or unbound) are offered (DM-12); `any` is the schema keyword `["any"]` |
| `+ RuleSet` from `guest` to `wan` | Refused: `wan` belongs to Zone `untrusted`, reference the Zone (DM-38, R-046.1). `lan1-to-lan2` is the allowed intra-zone case |
| Edit `publish-web` (untrusted → dmz-zone) | A `dnat` rule: destination `wan-ip` is the public address **before** translation (ADR-0010); translate to `web-01` / `https-8443` |
| Set its destination to `offsite-backup` | Saved, with a DM-40 **warning**: warnings never block a save or an Apply |
| Network → `wan` → tick `https` | Refused by DM-35: `publish-web` translates the management port of an Interface that allows https |
| Reorder Policies (drag, or ↑ ↓) | R-009; the RuleSet file is committed with the new order; hits follow the first match |
| Any form → *Edit YAML* | The file itself, checked live: schema errors and semantic errors with **file:line** and path (R-020.3) |
| Objects → new Address `10.0.0.0/255.0.0.0` | Schema error, live: CIDR only (R-012) |
| Delete `web-01` (used by a group and a dnat rule) | Rejected with DM-02, file, line and path |
| Delete or repaint `green`; change `https` | Rejected (DM-21, R-055.1). The comment of a predefined Color — its meaning — stays editable |
| Change only a comment or a color | Committed, and the Reconciler reports *no-op*: metadata never reaches a driver |
| Changes → *Import…* the schema's `examples/valid` | Refused with the DM-38 error on `untrusted-to-lan1` (finding 1); change `to: "lan1"` to `"internal"` and it is accepted, in one commit |
| Changes → *Export* | The repository as tar.gz (R-020.1); it reads back with 0 errors |
| Diagnostics → *Fail next apply* on Data Plane, then change a Policy | Transaction rolled back in reverse order, state *failed*, banner "desired ≠ applied" (R-020.6) |
| Changes → pick an old commit → *Restore* | Rollback is a new commit (components.md) |
| Edit the same object in two tabs, save both | Second save gets 409: optimistic locking on the revision, file-level merge (R-021.5) |
| Live traffic | SSE stream of FLOW_NEW/DROP/END with RuleSet / Policy, DNAT and SNAT translations and the reasons of `dataplane.md` §11 (R-026, R-028, R-028.1) |
| Wait for the periodic port-scan burst | Drop events are rate-limited and `events_lost` grows (R-026.1) |
| System → *Edit settings* → session timeout `1h` | The next login gets a 1 h session: Auth reads `System.admin` (R-043.4, R-043.5, R-043.2) |
| Sign in as auditor | Everything visible, every write is 403 (R-043.9) |
| Audit log → *Verify chain* | SHA-256 hash chain (R-035.1); edit `data/audit.log` by hand and verify again |
| Moon icon (every page) | Dark theme (R-044, R-044.1); the default follows `System.admin.ui.theme` |

## Layout

| Path | Component (components.md) |
|---|---|
| `internal/model` | Go types of every v1alpha1 kind, one field per schema property; canonical YAML writer (schema decision 12) |
| `internal/schema` | Validator layer 1: `v1alpha1/` is a verbatim copy of `songbird-yaml-schema/schema` (`55e63ac`); `ext/` the PoC proposals |
| `internal/validator` | Validator layer 2 — pure; `validate.py` rule ids + DM-xx; loader for any layout; tests on the schema corpus |
| `internal/predefined` | Shipped palette and Services (DM-21), embedded YAML |
| `internal/configstore` | Config Manager — Git repo, one file per object, commits, optimistic locking, uuids, history, revert, archive |
| `internal/reconciler` | Reconciler + Applier — level-triggered loop, ordered stages, rollback, last known good; simulated Compiler |
| `internal/events` | Simulated Data Plane with a first-match reference classifier (DNAT before translation) + Event Collector |
| `internal/observer` | Observer — `/health` (R-032), CPU/memory from `/proc`, simulated throughput |
| `internal/auth`, `internal/audit` | Auth (bcrypt cost 12, signed sessions, lockout from `System.admin`); Audit Log (hash-chained JSON lines) |
| `internal/api` | API Server — REST + SSE; never touches drivers |
| `web/` | Web UI — vanilla ES modules, no build step |
| `testdata/schema-examples` | Copy of `songbird-yaml-schema/examples`: the test corpus of the Validator (R-020.2) |

Repository layout (schema open question 3: the PoC writes one object per file, and imports any layout):

```
system/system.yaml        network/interfaces/  network/zones/  network/routes/*  network/dhcp/*
objects/colors/           objects/addresses/   objects/addressgroups/  objects/services/  objects/servicegroups/
firewall/rulesets/        users/*                                   (* PoC extension kinds)
```

### REST API (v1)

`POST /api/v1/login|logout`, `GET /api/v1/me`, `GET /api/v1/config` (whole tree, Validator findings, `ETag`),
`PUT|DELETE /api/v1/objects/{kind}/{name}` (`If-Match: "<revision>"` required, `?create=1`; JSON or
`Content-Type: application/yaml`), `POST /api/v1/validate/{kind}/{name}` (dry run), `GET /api/v1/objects/{kind}/{name}/yaml`,
`PUT /api/v1/users/{name}`, `GET /api/v1/export` (tar.gz), `POST /api/v1/import` (YAML, whole configuration),
`GET /api/v1/schema/{file}`, `GET /api/v1/status`, `GET /api/v1/history`, `GET /api/v1/history/{rev}/diff`,
`POST /api/v1/history/{rev}/revert`, `GET /api/v1/events`, `GET /api/v1/events/stream` (SSE), `GET /api/v1/metrics`,
`GET /api/v1/audit`, `GET /api/v1/audit/verify`, `POST /api/v1/diagnostics/faults` (PoC only), `GET /health` (no auth).
Writes need `X-Requested-With: songbird` (CSRF). 422 responses carry
`validation: [{file, line, path, reason, rule, constraint, severity}]`; successful saves carry the file's `warnings`.

## Validation

| Layer | What | Where |
|---|---|---|
| 1. structural | every document against `songbird.schema.json` (or an `ext/` schema), before it is typed: unknown fields, types, patterns, `if/then` rules, `ENC[...]` secrets | `internal/schema` (santhosh-tekuri/jsonschema, as the schema README prescribes) |
| 2. semantic | all the rules of `validate.py` (`unique-name`, `dangling-ref`, `wrong-kind`, `duplicate-hex`, `predefined-*`, `vlan-parent`, `zone-overlap`, `group-cycle`, `duplicate-pair`, `dnat-*`, `system-singleton`, `dns64-without-nat64`, …) | `internal/validator` |
| 2. semantic | the domain-model constraints the schema does not cover: DM-03/04 addressing, DM-06 (also Zone → same Zone), DM-12 binding, DM-17 port ranges, DM-21 predefined Services, DM-22/23 routes, DM-24…29 DHCP, DM-35 management port, DM-36 and ADR-0006 limits, DM-38 Zone members, DM-40 / DM-41 DNAT overlap and shadowing | `internal/validator` |
| warnings | IPv6, NAT64 (deferred, R-057), FQDN/MAC Addresses (not in the Data Plane yet), host bits in a subnet, DM-40 destination, DM-41 shadowing, `schedule` | never block a save or an Apply |

Tests: the 43-object example and the 38 invalid files of the schema (`internal/validator/corpus_test.go`), one
mutation per constraint on the factory default (`validator_test.go`), the HTTP flows (`internal/api`), the
reconciliation loop, and the simulated flows against the classifier.

## Simulated or missing

- **No kernel work.** Drivers only record the hash of their Plan section; the Compiler expands Zones and groups
  and hashes the result. Traffic is generated from the applied configuration and classified by a reference
  first-match classifier; throughput is synthetic.
- Not implemented: IPv6 (deferred in the domain model: kept in the files, not applied), setup wizard (R-031),
  TOTP (R-043.6), custom Roles, GitOps push/pull/conflict UI (R-040–R-042), alerting, address pseudonymisation
  in the log (R-037: configured, not applied), audit-log encryption at rest, secrets encryption (ADR-0009), schema
  migration (ADR-0008), renames (references are by name; the uuid would allow them).
- Live Interface events (R-015.1) need the Network Driver; the tree shows configured Interfaces only.

## Findings

For **songbird-yaml-schema**:

1. **The valid example breaks DM-38 / R-046.1.** `untrusted-to-lan1` references `lan1`, a member of Zone
   `internal`; `validate.py` has no such rule. The PoC rejects it; with `to: "internal"` the example is accepted.
2. **`validate.py` allows a RuleSet from a Zone to the same Zone** (`same-interface` checks Interfaces only);
   DM-06 and R-014.5 forbid it.
3. **No `predefined` field on Service**, while DM-21 marks predefined Services with `predefined: true`. The PoC keeps
   the list in an embedded YAML (`internal/predefined/services.yaml`), like the palette (open question 5).
4. **Kinds not covered yet.** The PoC proposes schemas for `StaticRoute`, `DhcpServer` and `User`
   (`internal/schema/ext`), in the same style (`additionalProperties: false`, common `$defs`). DHCP as its own kind
   keeps the Interface exactly as in v1alpha1.
5. **The patterns use ECMAScript lookaheads** (`fqdn`, `fqdnOrLabel`), which Go's RE2 cannot compile: the Go
   validator needs `regexp2` in ECMAScript mode, or the patterns must be rewritten without lookaheads.
6. **One file per object** works well with Git and per-file optimistic locking; the UI and the CLI still accept the
   multi-document files of the examples, so the choice can stay open for `songbird export` only.

For the **domain model**:

7. **Interface `role` values**: DM-24 says "dhcp only on `role: lan`"; the schema roles are `internal`, `external`,
   `dmz`, `transit`. The PoC forbids DHCP on `external` only.
8. **Nested groups and multi-protocol Services** (schema, RA) make DM-15, DM-16 and DM-18 inapplicable as written.
   The PoC applies DM-12 to the expanded members of a group, which covers what DM-16 wanted; DM-18 is dropped.
   **InterfaceAddresses** has no syntax in the schema and is not offered.
9. **DM-28 is implied by DM-04 + DM-25**: DHCP ranges lie in Interface subnets, which cannot overlap.

For the **RA**:

10. Per-rule `log` (open issue #8) is stored and shown but the simulated Data Plane emits every Flow (R-026);
    intra-zone traffic without a RuleSet (R-046.2, open issue #10) is implemented with the `intraZoneTraffic`
    verdict and produces events with reason `intra-zone`, a reason `dataplane.md` §11 does not list yet.
