# PoC — Web UI v2 (aligned with songbird-architecture)

A proof of concept of the Songbird Web UI **and of the Control Plane slice behind it**, built from
`songbird-architecture` (domain-model v0.4, components v0.1, reconciliation v0.1, ADR-0001) and
`songbird-requirements-analysis` v2.0.1.

Unlike `PoC-webui` (static mock-ups), every screen here is backed by a working pipeline:

```
Web UI ──REST/SSE──▶ API Server ──▶ Config Manager (YAML files in a local Git repo, 1 commit per save)
                                        │ new commit
                                        ▼
                     Reconciler: read HEAD → Validate (DM-xx) → Compile → Diff → Apply (atomic) → Status
                                                                              │
                                    simulated drivers: network, dataplane, dhcp, dns
```

One static Go binary (C-004), frontend embedded, no Node build step, no database.

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

On first start the factory-default configuration is committed to `./data/config` (a normal Git repository:
try `git -C data/config log -p`). Delete `./data` to start over. Use `-tls-cert/-tls-key` for HTTPS.

## What to try

| Try this | What it shows |
|---|---|
| Policies → `+ Policy` and leave Source empty | Save is disabled and the field is highlighted (R-009.1) |
| Pick sources in a Zone | Only objects allowed by DM-10/11/12 are offered |
| Reorder Policies (drag, or ↑ ↓) | R-009; the Zone file is committed with the new order |
| Save anything | Toast "Saved and applied": the API waits for the Reconciler (reconciliation.md §5) |
| Objects → edit a range to `10.0.0.9-10.0.0.1` | Live dry-run validation: `addresses/x.yaml spec.value … (DM-14)` (R-020.3) |
| Delete `nas` (used by a Policy) | Rejected with DM-02, file and path |
| Edit `HTTP` or the value of a predefined Colour | Rejected (DM-21, R-055.1); a Colour's meaning stays editable |
| Diagnostics → *Fail next apply* on Data Plane, then change a Policy | Transaction rolled back in reverse order, state *failed*, banner "desired ≠ applied", revert link (R-020.6, §6–§9) |
| Untick the fault | Treated as a fact change: HEAD is retried and applied |
| Changes → pick an old commit → *Restore* | Rollback is a new commit (components.md) |
| Edit the same object in two tabs, save both | Second save gets 409: optimistic locking on the revision, file-level merge (components.md, open issue #5) |
| Live traffic | SSE stream of FLOW_NEW/DROP/END with dataplane.md reasons, filters without reload (R-028, R-028.1) |
| Wait for the periodic port-scan burst | Drop events are rate-limited and `events_lost` grows (R-026.1) |
| Sign in as auditor | Everything visible, every write is 403 (R-043.9) |
| 5 wrong passwords | 6th attempt refused for 10 min, all audited (R-043.2, R-043.3) |
| Audit log → *Verify chain* | SHA-256 hash chain (R-035.1); edit `data/audit.log` by hand and verify again |
| Moon icon (every page) | Dark theme (R-044, R-044.1) |

## Layout

| Path | Component (components.md) |
|---|---|
| `internal/model` | Domain-model types; one CRD-style YAML file per object |
| `internal/validator` | Validator — pure, DM-01…DM-36 (IPv4 subset); unit tests |
| `internal/configstore` | Config Manager — Git repo, commits, optimistic locking, history, revert |
| `internal/reconciler` | Reconciler + Applier — level-triggered loop, coalesced triggers, ordered stages, rollback, last known good; simulated Compiler and drivers; tests |
| `internal/events` | Simulated Data Plane ring buffer + Event Collector (buffer, SSE fan-out, token bucket) |
| `internal/observer` | Observer — `/health` (R-032), CPU/memory from `/proc`, simulated throughput |
| `internal/auth`, `internal/audit` | Auth (bcrypt cost 12, signed 8 h session, lockout); Audit Log (hash-chained JSON lines) |
| `internal/api` | API Server — REST + SSE; never touches drivers |
| `web/` | Web UI — vanilla ES modules, no build step |

### REST API (v1)

`POST /api/v1/login|logout`, `GET /api/v1/me`, `GET /api/v1/config` (whole tree + `ETag`),
`PUT|DELETE /api/v1/objects/{kind}/{name}` (`If-Match: "<revision>"` required, `?create=1`),
`POST /api/v1/validate/{kind}/{name}` (dry run), `GET /api/v1/objects/{kind}/{name}/yaml`,
`PUT /api/v1/users/{name}`, `GET /api/v1/status`, `GET /api/v1/history`, `GET /api/v1/history/{rev}/diff`,
`POST /api/v1/history/{rev}/revert`, `GET /api/v1/events`, `GET /api/v1/events/stream` (SSE),
`GET /api/v1/metrics`, `GET /api/v1/audit`, `GET /api/v1/audit/verify`, `POST /api/v1/diagnostics/faults` (PoC only),
`GET /health` (no auth). Writes need `X-Requested-With: songbird` (CSRF), and 422 responses carry
`validation: [{file, path, reason, constraint}]`.

## Simulated or missing

- **No kernel work.** Drivers only record the hash of their Plan section; the Compiler only splits and hashes
  the tree and counts expanded match entries. Traffic and throughput are generated from the applied config.
- Not implemented: IPv6 (deferred in the domain model), setup wizard (R-031), TOTP (R-043.6), custom Roles,
  GitOps push/pull/conflict UI (R-040–R-042), alerting, DNS configuration (DnsConfig is not modelled yet),
  address pseudonymisation (R-037), audit-log encryption at rest, secrets encryption (ADR-0009), schema migration (ADR-0008).
- Live Interface events (R-015.1) need the Network Driver; the tree shows configured Interfaces only.

## Open points this PoC surfaced

1. **Interface naming.** The domain model names Interfaces logically (`LAN`, `WAN1`) but has no attribute
   for the kernel device (`eth0`, `eth0.10`, R-015). The PoC uses logical names; a `device` field is likely needed.
2. **YAML file layout** (ADR-0001 "to decide later"): the PoC uses one file per object, `<kind-dir>/<name>.yaml`,
   with Policies inside their Zone file. This makes optimistic locking per file natural, but two admins editing
   different Policies of the same Zone conflict.
3. **Renames** are not supported: references are by name, so a rename must rewrite every referencing file in
   one commit. To decide whether the API offers it.
4. **Validation of a stale HEAD**: the API rejects only *new* validation errors, so an invalid HEAD (e.g. after a
   manual edit) does not block unrelated fixes. Worth stating in the design.
5. **Dry-run endpoint**: live validation in forms (`/validate`) proved very useful for R-009.1/R-020.3; candidate for the real API.
