# Songbird Architecture Analysis: Monolithic vs. Multi-Daemon

## 1. Introduction
This document evaluates two primary architectural patterns for the Songbird firewall system: a **Monolithic** approach where all userspace logic resides in a single process, and a **Multi-Daemon** approach where functionality is split across specialized, isolated processes.

Given Songbird's requirements for **Security**, **Resilience**, and **Modular Extensibility**, this analysis concludes that a **Multi-Daemon approach is the superior choice**.

---

## 2. Comparison Table

| Feature | Monolithic | Multi-Daemon (Recommended) |
| :--- | :--- | :--- |
| **Fault Isolation** | Low: UI/API crash kills the firewall engine. | High: UI crash does not affect packet filtering. |
| **Security** | Low: Entire codebase must run as `root`. | High: Only the Engine runs as `root`. |
| **Complexity** | Low: Easier to develop, test, and deploy. | Medium: Requires IPC and process management. |
| **Upgradeability** | All-or-nothing updates. | Individual services can be updated/restarted. |
| **Memory/CPU** | Slightly lower overhead. | Slightly higher due to multiple processes. |
| **Developer Experience** | Fast initial progress. | Better for long-term modularity. |

---

## 3. Monolithic Analysis
In this model, the Gin/Echo API, the BPF Map manager, the GitOps worker, and the DNS/DHCP controllers all run inside one Go binary.

### Pros
- **Simplicity:** No need for IPC (Inter-Process Communication); shared state is just memory variables.
- **Single Binary:** Easier to package and distribute (though not a requirement for Hadron OS).
- **Lower Resource Footprint:** Single Go runtime overhead.

### Cons
- **Single Point of Failure:** A memory leak in a dashboard chart or a panic in the API layer will stop the firewall control plane, preventing rule updates or log processing.
- **Excessive Privileges:** The entire process must run as `root` to manage eBPF, even code that handles untrusted Web UI input.
- **Tight Coupling:** Difficult to swap or disable components (e.g., swapping the internal DNS for an external one).

---

## 4. Multi-Daemon Analysis (The Recommended Approach)
Functions are split into distinct processes: `songbird-api`, `songbird-engine`, `songbird-gitops`, etc.

### Pros
- **Enhanced Security:** Only the small, hardened `songbird-engine` requires `CAP_BPF` and `CAP_NET_ADMIN`. The API can run as a low-privileged user.
- **Resilience:** If the Web UI/API crashes, the `songbird-engine` continues to process eBPF ring-buffer events and maintain kernel state.
- **Clean Extensibility:** New features (VPN, BGP, IDS) can be added as optional daemons that the API interacts with.
- **Independent Life-cycles:** The API can be "hot-reloaded" for a UI update without touching the firewall state.

### Cons
- **IPC Overhead:** Requires a mechanism for daemons to communicate.
- **Deployment complexity:** Requires a process manager (like `systemd` or a Go-based supervisor).

---

## 5. Proposed IPC & Shared State Strategy

To manage the complexity of a multi-daemon setup, we recommend the following:

### Inter-Process Communication (IPC)
- **gRPC over Unix Domain Sockets (UDS):** Provides high-performance, type-safe communication between the API and the Engine. UDS ensures that the API is local to the machine and allows for standard Linux file permissions to control access to the socket.

### Shared State (Source of Truth)
- **File-based GitOps:** The YAML configuration (e.g., `/etc/songbird/config.yaml`) should be the single source of truth.
- **The API** acts as a "Config Editor" that validates and writes the YAML.
- **The Engine** acts as a "Config Consumer" that watches the file (via `inotify`) and applies changes to eBPF maps.
- **The GitOps Daemon** handles the synchronization between the local file and the remote repository.

---

## 6. Proposed Daemon Breakdown

1.  **`songbird-engine` (Root):**
    - Loads eBPF programs.
    - Syncs YAML configuration to BPF maps.
    - Reads ring-buffer events and writes to local logs.
2.  **`songbird-api` (Non-Root):**
    - Serves the React Web UI.
    - Provides the Gin-based REST API.
    - Handles Auth/JWT and Session Management.
    - Communicates with other daemons via gRPC.
3.  **`songbird-gitops` (Non-Root):**
    - Periodically pulls/pushes the YAML config to the Git repository.
    - Signals the Engine or API when a remote change is detected.
4.  **`songbird-services` (Optional/Modular):**
    - Lightweight managers for DNS (Unbound/miekg) and DHCP.

---

## 7. Conclusion
For a security-critical product like Songbird, **the Multi-Daemon approach is the only choice that provides the necessary fault tolerance and privilege separation.** While it requires more initial work in setting up IPC (gRPC/UDS), it creates a robust foundation that can scale from a simple PoC to a production-grade enterprise firewall.
