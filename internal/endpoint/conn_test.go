package endpoint

import (
	"net"
	"testing"
)

func TestConnRegistry(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(r *ConnRegistry, pipe net.Conn)
		connID     string
		clientID   string
		wantConn   net.Conn
		wantErr    error
	}{
		{
			name:     "add and claim",
			setup:    func(r *ConnRegistry, c net.Conn) { r.AddConn(c, "conn-1", "client-a") },
			connID:   "conn-1",
			clientID: "client-a",
			wantErr:  nil,
		},
		{
			name:     "ownership violation",
			setup:    func(r *ConnRegistry, c net.Conn) { r.AddConn(c, "conn-1", "client-a") },
			connID:   "conn-1",
			clientID: "client-b",
			wantErr:  ErrPermissionDenied,
		},
		{
			name:     "claim not found",
			setup:    func(r *ConnRegistry, c net.Conn) {},
			connID:   "nonexistent",
			clientID: "client-a",
			wantErr:  ErrConnectionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewConnRegistry()
			pipe, _ := net.Pipe()

			tt.setup(r, pipe)

			conn, err := r.ConnByConnectionID(tt.connID, tt.clientID)
			if err != tt.wantErr {
				t.Fatalf("got err %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && conn != pipe {
				t.Fatal("returned conn does not match")
			}
		})
	}
}

func TestConnRegistry_DoubleClaim(t *testing.T) {
	r := NewConnRegistry()
	pipe, _ := net.Pipe()

	r.AddConn(pipe, "conn-1", "client-a")

	conn, err := r.ConnByConnectionID("conn-1", "client-a")
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if conn != pipe {
		t.Fatal("returned conn does not match")
	}

	_, err = r.ConnByConnectionID("conn-1", "client-a")
	if err != ErrConnectionNotFound {
		t.Fatalf("second claim: got err %v, want ErrConnectionNotFound", err)
	}
}

func TestConnRegistry_MultipleConns(t *testing.T) {
	r := NewConnRegistry()
	pipe1, _ := net.Pipe()
	pipe2, _ := net.Pipe()

	r.AddConn(pipe1, "conn-1", "client-a")
	r.AddConn(pipe2, "conn-2", "client-b")

	c1, err := r.ConnByConnectionID("conn-1", "client-a")
	if err != nil || c1 != pipe1 {
		t.Fatalf("conn1: got %v, %v; want pipe1, nil", c1, err)
	}

	c2, err := r.ConnByConnectionID("conn-2", "client-b")
	if err != nil || c2 != pipe2 {
		t.Fatalf("conn2: got %v, %v; want pipe2, nil", c2, err)
	}
}
