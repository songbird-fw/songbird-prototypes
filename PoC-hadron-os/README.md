# PoC — Hadron OS image

| | |
|---|---|
| Question | Can we build an immutable Hadron/Kairos image that ships and runs our eBPF binary? |
| Status | **Active** |
| Based on | RA 2.0.2 (C-002, R-019), ADR-0007, ADR-0008, ADR-0009 |
| Result | An ISO is produced with auroraboot. Still to verify on it: kernel version and BTF/XDP support, init system, `/usr/local` persistence, TPM, A/B upgrade. |
| Decisions | Inputs for the deployment view (not written yet). |

## Build

The Dockerfile builds the payload from source (multi-stage), so no binary is committed. Run it **from the repository root**, because the payload lives elsewhere in the repo:

```bash
docker build -f PoC-hadron-os/Dockerfile -t songbird-hadron:0.1.1 --build-arg VERSION=0.1.1 .
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v $PWD/build/:/output \
        quay.io/kairos/auroraboot:v0.20.1 build-iso --output /output/ oci:songbird-hadron:0.1.1
```

The ISO is written to `build/` (ignored by Git). To check only that the payload compiles: `docker build -f PoC-hadron-os/Dockerfile --target payload .`

The payload is the archived XDP PoC (`archive/PoC-basic-xdp-firewall`), the only eBPF program available. Change `PAYLOAD_DIR` when the real Control Plane binary exists.

## To do

- Pin the base images by digest, e.g. `docker buildx imagetools inspect ghcr.io/kairos-io/hadron:v0.0.1-beta2`, and replace the tags in the Dockerfile.
- Decide what `VERSION` means: the image version or the Songbird version (they should match).
