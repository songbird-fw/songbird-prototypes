# songbird-prototypes

Proofs of concept for Songbird. A prototype is an **experiment that answers one question**: when it has answered, the result is recorded in the Requirements Analysis or in an ADR, and the code stops being a reference.

## Status

| Prototype | Question | Status | Result / decisions |
|---|---|---|---|
| [PoC-webui-v2](PoC-webui-v2) | Does the designed Control Plane work end to end behind a real UI? | **Active** | Works end to end with simulated drivers on the `songbird-fw` schema v1alpha1 (2026-10-19 decisions) and domain model v0.7; 7 findings for schema, model and RA |
| [PoC-hadron-os](PoC-hadron-os) | Can an immutable Hadron/Kairos image ship and run our eBPF binary? | **Active** | ISO builds; kernel, persistence, TPM and A/B upgrade still to verify |
| [archive/PoC-feasibility-study](archive/PoC-feasibility-study) | Is Go suitable for the Control Plane? | Concluded | Yes → RA C-004 |
| [archive/PoC-basic-xdp-firewall](archive/PoC-basic-xdp-firewall) | Can XDP filter with rules managed from Go? | Superseded | Yes, with the limits in `dataplane.md` §2 → new Data Plane design |
| [archive/PoC-interoperability](archive/PoC-interoperability) | XDP + nftables + PostgreSQL backend | Superseded | Rejected: contradicts C-001 and ADR-0001 |
| [archive/PoC-webui](archive/PoC-webui) | What could the UI look like? | Superseded | Replaced by PoC-webui-v2 |

| Status | Meaning | Where | Maintenance |
|---|---|---|---|
| **Active** | Experiment in progress, or basis for the next steps | repository root | Must build and pass its tests in CI; its README states which document versions it is based on |
| **Concluded** | Answered its question; the result is in the RA or an ADR | `archive/` | Frozen |
| **Superseded** | Replaced by another prototype, or explored an approach that was rejected | `archive/` | Frozen, with the reason at the top of its README |

## Rules

- **Every prototype starts with this header** in its README:

  ```markdown
  | | |
  |---|---|
  | Question | <the question it answers> |
  | Status | Active / Concluded / Superseded by … |
  | Based on | <RA, domain model, schema versions> |
  | Result | <the answer, once known> |
  | Decisions | <ADRs or requirements that follow from it> |
  ```

- **No binaries in Git.** Build outputs are produced from source or by CI (see `.gitignore`).
- **Closing a prototype**: record the result in the RA or an ADR, update its header and this table, then `git mv` it to `archive/` so its history is kept.
- **CI** (`.github/workflows/active-prototypes.yml`) builds and tests the active prototypes only.

## Quick start

```bash
cd PoC-webui-v2 && go run .          # http://127.0.0.1:8080 — admin / songbird
docker build -f PoC-hadron-os/Dockerfile -t songbird-hadron:0.1.1 --build-arg VERSION=0.1.1 .   # from the repository root
```
