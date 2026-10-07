# go-yact (yet another connection tunnel)

## Architecture Proposal

There is an control port at 8001. To initialise, the client sends a "Hello" message to the server on
the TCP control port, asking to create a proxy listener on the provided port. The server acks and
starts listening for external TCP connections on the provided port.

On new connection to proxy, the proxy generates a connection ID and sends it to the client via the
control channel, so that a client can create a separate connection and sending the provided
connection ID in the "accept" message (via control message). From that point on the server proxies
the payloads between those connections. If the client failed to establish the connection within the
timeout, the server connection is dropped.

gRPC with mTLS is used for the control connection.

## Tunnel Control & Data Plane Architecture

### Overview:
This service defines a secure, mTLS-authenticated TCP tunneling protocol
over gRPC. It allows a client (behind a NAT/firewall) to expose a local TCP
service through a public proxy server listening on a designated port.

### Protocol Lifecycle:
1. Registration (Unary RPC):
   The client calls `RegisterProxy` providing the desired port and optional
   secret. The server starts an external TCP listener on the requested port
   and responds with confirmation.

2. Control Plane / Event Notification (Server-Streaming RPC):
   The client invokes `ListenEvents` to maintain a long-lived, server-to-client
   event stream. Whenever an external user connects to the proxy listener:
   a) The server generates a unique, cryptographically secure `connection_id`.
   b) The server temporarily holds the incoming user socket.
   c) The server emits a `NewConnectionEvent(connection_id)` to the client
      over `ListenEvents`.

3. Data Plane / Payload Proxying (Bi-Directional Streaming RPC):
   Upon receiving a `NewConnectionEvent`:
   a) The client dials its local target service (e.g., localhost:8080).
   b) The client invokes `OpenDataPipe` to open a new, dedicated bi-directional
      stream to the proxy server.
   c) In the VERY FIRST `DataPacket` sent over `OpenDataPipe`, the client includes
      the `connection_id` token to claim the waiting socket.
   d) The server verifies the token, bridges the external TCP socket to the gRPC
      stream, and raw byte proxying begins.

### Design Rationale & Trade-offs:
- Decoupled RPC Isolation: Control events (`ListenEvents`) are separated from
  data streams (`OpenDataPipe`). A failure or disconnect in a single proxy data
  stream never impacts the primary event stream or other active connections.

- Security & NAT Traversal: The client initiates all connections outbound to
  the server on a single public gRPC port (8901), eliminating the need for extra
  open server ports, complex port-mapping, or NAT holes. Token validation in
  `OpenDataPipe` prevents connection hijacking across shared egress IPs/CGNATs.

- Built-in Transport Management: Uses gRPC mTLS for transport encryption and
  mutual identity verification, while relying on HTTP/2 Keep-Alives for dead
  connection detection instead of custom application-level ping/pongs.

```
[ Local Client ]                                          [ Tunnel Server ]
  |                                                               |
  |--- 1. RegisterProxy(HandshakeRequest{port: 8080}) ----------->|
  |<-- 2. HandshakeResponse{success: true} -----------------------|
  |                                                               |
  |--- 3. ListenEvents(EventStreamRequest) ---------------------->| (Keeps stream open)
  |                                                               |
  |              (External TCP user connects to 8080)             |
  |                                                               |
  |<-- 4. NewConnectionEvent{id: "uuid-123"} ---------------------|
  |                                                               |
  |--- 5. OpenDataPipe() ---------------------------------------->|
  |       First Packet: DataPacket{connection_id: "uuid-123"}     |-- Matches "uuid-123" to user
  |       Subsequent:   DataPacket{payload: [...] }               |   socket & bridges pipes
  |<================ Bi-directional Data Pipe ===================>|
```
