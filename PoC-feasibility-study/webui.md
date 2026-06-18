# Feasibility Study: WebSocket-Only WebUI Architecture

## 1. Overview
This study evaluates the feasibility and implications of moving to a WebSocket-only communication model between the React frontend and the Go backend for the Songbird Firewall WebUI. In this model, all interactions—including real-time logs, dashboard metrics, and CRUD (Create, Read, Update, Delete) operations for firewall rules—are handled over a single, persistent WebSocket connection.

## 2. Benefits
*   **Real-Time by Default**: Eliminates the need for separate logic for polling (REST) and streaming (WebSockets). The UI always reflects the current state of the backend.
*   **Reduced Overhead**: Avoids the repetitive handshake and header overhead of multiple HTTP requests for frequent small updates (e.g., rule priority shifts).
*   **Simplified State Management**: Since every client is connected to a "live" stream of events, complex frontend cache invalidation (like React Query or SWR) becomes simpler: if a change happens (by any user), the backend broadcasts the new state, and the UI updates automatically.
*   **Lower Latency**: Persistent connections offer lower latency for configuration changes compared to establishing new TCP/TLS connections for every REST call.

## 3. Challenges & Mitigations

### 3.1 Request-Response Correlation
**Problem**: Unlike HTTP, WebSockets are inherently asynchronous. When the UI sends a "Create Rule" message, it needs a way to know when *that specific* request has succeeded or failed.
**Recommendation**: Implement a **Correlation ID** pattern. Every message sent from the client should include a unique ID (e.g., UUID). The server must respond with the same ID in its acknowledgement or error message.

### 3.2 Error Handling & Status Codes
**Problem**: You lose standard HTTP status codes (400, 401, 403, 500).
**Recommendation**: Define a standardized JSON envelope that includes a status field.
```json
{
  "type": "RULE_CREATE_RESPONSE",
  "correlation_id": "v1-123",
  "status": "error",
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Invalid CIDR format"
  }
}
```

### 3.3 Connection Lifecycle & Reliability
**Problem**: WebSockets can drop due to network instability or service restarts.
**Recommendation**:
*   **Frontend**: Implement exponential backoff for reconnection.
*   **State Sync**: Upon successful (re)connection, the backend must immediately send a "FULL_STATE_SYNC" message to ensure the UI isn't displaying stale data.

### 3.4 Persistence Race Conditions
**Problem**: If two users modify rules simultaneously via WebSockets, one might overwrite the other's changes in `rules.yaml`.
**Recommendation**: Use a **Single-Writer Lock** in the Go backend. All rule modifications should pass through a central controller that performs atomic writes to the YAML file (write to temp file then rename) to prevent corruption.

## 4. Recommended Message Protocol

We recommend a structured JSON envelope format:

### Client -> Server (Action)
```json
{
  "action": "CREATE_RULE",
  "id": "msg-001",
  "payload": {
    "priority": 10,
    "src_cidr": "192.168.1.0/24",
    "action": "DROP"
  }
}
```

### Server -> Client (Ack/Response)
```json
{
  "type": "ACK",
  "ref_id": "msg-001",
  "status": "success",
  "timestamp": 1715000000
}
```

### Server -> All Clients (Broadcast)
```json
{
  "type": "STATE_UPDATE",
  "topic": "FIREWALL_RULES",
  "payload": [ ... full list of rules ... ]
}
```

## 5. Persistence Strategy
To fulfill the requirement of writing changes to `rules.yaml`:
1.  **Incoming WS Message**: Backend receives `UPDATE_RULE`.
2.  **Validation**: Backend validates the new rule set.
3.  **Atomic Write**: Backend serializes the entire rule set to `rules.yaml.tmp` and renames it to `rules.yaml`.
4.  **BPF Sync**: Backend updates the eBPF Hash Maps.
5.  **Broadcast**: Backend sends the new rule list to all connected WebSocket clients.

## 6. Conclusion
Going "all in" on WebSockets is a highly effective strategy for a low-latency, high-sync environment like a firewall dashboard. While it requires more effort to implement robust request tracking and error handling than standard REST, the UX benefits of a truly real-time interface significantly outweigh the implementation complexity in the context of the Songbird project.

**Final Recommendation**: Proceed with the WebSocket-only model using the **JSON Correlation Envelope** pattern and **Atomic YAML persistence**.
