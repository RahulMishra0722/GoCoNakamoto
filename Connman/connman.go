package connman

import (
	"fmt"
	"net"
	"sync"
)

// CNode represents a connection node
type CNode struct {
	addr    string
	service string
}

// SendMessage sends a message to the specified address
func (cn *CNode) SendMessage(addr string) {
	// TODO: Implement message sending using a goroutine
	go func() {
		// Add implementation here
	}()
}

// NewCNode creates a new CNode
func NewCNode(addr string) *CNode {
	return &CNode{
		addr: addr,
	}
}

// Connman represents a connection manager
type Connman struct {
	listenAddr string
	ln         net.Listener
	peers      []CNode
	mu         sync.Mutex
}

// Start initializes and starts the Connman
func (cm *Connman) Start() error {
	ln, err := net.Listen("tcp", cm.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	cm.ln = ln
	go cm.acceptLoop()
	return nil
}

// Stop closes the listener
func (cm *Connman) Stop() error {
	if cm.ln != nil {
		return cm.ln.Close()
	}
	return nil
}

func (cm *Connman) acceptLoop() {
	for {
		conn, err := cm.ln.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		go cm.handleConnection(conn)
	}
}

func (cm *Connman) handleConnection(conn net.Conn) {
	defer conn.Close()
	// TODO: Implement handshake and other connection setup steps

	cm.readLoop(conn)
}

func (cm *Connman) readLoop(conn net.Conn) {
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("read loop error:", err)
			return
		}

		msg := buf[:n]
		parsed := string(msg)
		// TODO: Handle the parsed message
		fmt.Println("Received message:", parsed)
	}
}

// AddPeer adds a new peer to the Connman
func (cm *Connman) AddPeer(peer CNode) {
	cm.mu.Lock()
	defer cm.mu.Lock()
	cm.peers = append(cm.peers, peer)
}
