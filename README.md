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

The server may optionally start with a secret parameter, in which case all clients are required to
provide the same secret in the hello message.

gRPC with mTLS is used for the control connection.