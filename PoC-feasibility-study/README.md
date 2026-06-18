# Feasibility Study: Using Go for the Songbird Firewall System

## Executive Summary
This document evaluates the feasibility of using Go (Golang) as the primary programming language for the Songbird firewall's userspace components, including the Central API, Firewall Engine management, and orchestration of network services. Based on the requirements analysis and architectural goals, **Go is an exceptionally well-suited choice** for this project.

## 1. Feasibility Assessment

### 1.1 Core Components
| Component | Feasibility | Recommended Go Library |
| :--- | :--- | :--- |
| **Central API** | High | [Gin-Gonic](https://github.com/gin-gonic/gin) |
| **eBPF Management** | High | [cilium/ebpf](https://github.com/cilium/ebpf) |
| **Networking (Netlink)** | High | [vishvananda/netlink](https://github.com/vishvananda/netlink) |
| **GitOps (Git Sync)** | High | [go-git](https://github.com/go-git/go-git) |
| **DNS Server/Forwarder** | High | [miekg/dns](https://github.com/miekg/dns) |
| **DHCP Server** | High | [insomniacslk/dhcp](https://github.com/insomniacslk/dhcp) |

### 1.2 Analysis of Benefits
*   **Static Compilation:** Go can produce single, statically linked binaries (using `CGO_ENABLED=0`). This is critical for the **Kairos Hadron** immutable OS, as it eliminates dependencies on shared libraries and simplifies the A/B update process.
*   **Performance:** Go offers high performance with low memory overhead, suitable for a networking appliance. While the data plane (eBPF) handles packet processing at line rate, Go is more than capable of managing the control plane without introducing bottlenecks.
*   **Ecosystem:** The Go ecosystem for networking and system programming is mature. Libraries like `vishvananda/netlink` are industry standards for Linux network configuration.
*   **Developer Efficiency:** Consolidating on Go reduces the "mental context switching" for the team and simplifies the CI/CD pipeline.

## 2. Architectural Recommendations

### 2.1 Unified Songbird Binary
We recommend starting with a **single monolithic binary** that encapsulates:
1.  **Gin-based REST API:** Serving both the Web UI (via embedded static assets) and external CLI tools.
2.  **Firewall Engine:** Managing the lifecycle of eBPF programs and syncing rules from YAML/API to BPF maps.
3.  **Service Orchestrator:** Managing external daemons (Unbound, Kea) or hosting internal Go-based services for DNS/DHCP.
4.  **GitOps Worker:** A background routine that periodically syncs the local YAML state with a remote Git repository.

### 2.2 API Design (Gin)
Following the requested standard, the Gin API should implement a clear separation of concerns:
*   `/api/v1/policies`: CRUD operations for firewall rules.
*   `/api/v1/interfaces`: Management of physical and logical (VLAN) interfaces.
*   `/api/v1/services`: Configuration for DNS, DHCP, and NAT.
*   `/api/v1/git`: Triggering manual Git push/pull operations.
*   `/api/v1/health`: Real-time status of internal subsystems.

### 2.3 eBPF Interaction
The Go control plane should use the `cilium/ebpf` library to:
*   Load and attach XDP programs to interfaces.
*   Perform atomic map updates for firewall rules (using `BPF_MAP_TYPE_ARRAY` or `BPF_MAP_TYPE_HASH`).
*   Consume performance events from the kernel for real-time traffic logging.

## 3. Potential Challenges & Mitigations

| Challenge | Mitigation |
| :--- | :--- |
| **eBPF in C vs Go** | Keep the BPF data plane in C (as it is highly optimized and standard), but use Go for all map management and lifecycle control. |
| **Complex Orchestration** | For features like BGP (GoBGP) or IDS (Suricata), start by orchestrating external tools. If the footprint grows too large, consider moving these specific components to optional containers/modules. |
| **Netlink Complexity** | Use a high-level wrapper like `vishvananda/netlink` to handle the complexities of the netlink protocol while maintaining an idiomatic Go API. |

## 4. Conclusion
Using Go for the userspace of Songbird is highly feasible and recommended. It aligns perfectly with the requirements for an immutable OS (Hadron), provides the necessary performance for a control plane, and leverages a robust ecosystem of networking libraries. The transition to a Gin-based central API will provide a modern, scalable foundation for the WebUI and future integrations.
