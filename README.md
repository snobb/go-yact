# go-yact (yet another connection tunnel)

## Quick Start

### 1. Build

```sh
make build    # produces ./bin/yact
```

### 2. Generate mTLS certificates

```sh
make create-certs
# equivalent to:
# ./bin/yact make-certs --cert-dir ./config/certs
```

This creates a CA plus a server and a client certificate:

```
config/certs/
├── server/    ca.crt  server.crt  server.key
└── client1/   ca.crt  client.crt  client.key
```

Use `-n <count>` to generate more than one client certificate (e.g.
`./bin/yact make-certs --cert-dir ./config/certs -n 3`). If the server is reached over a
name or IP other than `localhost`, include it so the certificate is valid for that
endpoint:

```sh
./bin/yact make-certs --cert-dir ./config/certs \
  --domain tunnel.example.com --ip-address 203.0.113.10
```

### 3. Start the server (proxy)

`--cert-dir` is used as-is, so point it at the server certificate directory:

```sh
./bin/yact proxy --cert-dir ./config/certs/server --address :8001
```

Omit `--cert-dir` to use the default `~/.config/yact/certs/server`.

### 4. Start the client (tunnel)

Point the client at its own certificate directory and at the server's control port:

```sh
./bin/yact client --cert-dir ./config/certs/client1 --address <server-host>:8001 \
  -L :8080:3000
```

Omit `--cert-dir` to use the default `~/.config/yact/certs/client1`.

`-L` takes `bind_host:bind_port:local_port`:

- `:8080` — address/port the **server** listens on for external users
- `3000` — the **local** port on the client machine that traffic is forwarded to

So in this example, connecting to `<server-host>:8080` reaches `localhost:3000` on the
client. Repeat `-L` to expose multiple services:

```sh
./bin/yact client --cert-dir ./config/certs/client1 --address <server-host>:8001 \
  -L :8080:3000 \
  -L 127.0.0.1:8443:443
```

### Using config files

Both commands accept a config file. The `--config`/`-c` flag is a global flag, so it must
come **before** the subcommand:

```sh
./bin/yact -c server.yaml proxy
./bin/yact -c client.yaml client
```

In a config file `cert_dir` is used as-is (no `server`/`client1` suffix is added), so it
must point at the directory holding that role's `ca.crt`, `server.crt`/`client.crt` and
key. For example:

```yaml
# server.yaml
tls_config:
  cert_dir: ./config/certs/server
proxy_addr: ":8001"
```

```yaml
# client.yaml
tls_config:
  cert_dir: ./config/certs/client1
proxy_addr: "<server-host>:8001"
to_addrs:
  - bind_address: ":8080"
    local_port: 3000
```

Values from flags override the config file, which overrides the defaults. See
`config/yactrc.yaml.server-example` and `config/yactrc.yaml.client-example` for all
available options.

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
