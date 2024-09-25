package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Message types
const (
	MsgTypeVersion = iota
	MsgTypeVerack
	MsgTypePing
	MsgTypePong
	MsgTypeAddr
	MsgTypeTx
)

// Peer represents a connected peer
type Peer struct {
	conn       net.Conn
	addr       string
	inbound    bool
	disconnect chan struct{}
	msgChan    chan []byte
}

// Node represents the main node structure
type Node struct {
	listenAddr string
	peers      map[string]*Peer
	addrBook   map[string]time.Time
	txMempool  map[string][]byte
	mu         sync.RWMutex
}

// NewNode creates a new Node instance
func NewNode(listenAddr string) *Node {
	return &Node{
		listenAddr: listenAddr,
		peers:      make(map[string]*Peer),
		addrBook:   make(map[string]time.Time),
		txMempool:  make(map[string][]byte),
	}
}

// Start initializes and starts the Node
func (n *Node) Start() error {
	ln, err := net.Listen("tcp", n.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	go n.acceptLoop(ln)
	go n.maintainConnections()
	return nil
}

func (n *Node) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		go n.handleConnection(conn, true)
	}
}

func (n *Node) handleConnection(conn net.Conn, inbound bool) {
	peer := &Peer{
		conn:       conn,
		addr:       conn.RemoteAddr().String(),
		inbound:    inbound,
		disconnect: make(chan struct{}),
		msgChan:    make(chan []byte, 100),
	}

	n.mu.Lock()
	n.peers[peer.addr] = peer
	n.mu.Unlock()

	go n.readLoop(peer)
	go n.writeLoop(peer)

	// Perform handshake
	n.sendVersion(peer)
}

func (n *Node) readLoop(peer *Peer) {
	defer func() {
		n.mu.Lock()
		delete(n.peers, peer.addr)
		n.mu.Unlock()
		close(peer.disconnect)
		peer.conn.Close()
	}()

	for {
		msgType, payload, err := n.readMessage(peer.conn)
		if err != nil {
			fmt.Printf("read error from %s: %v\n", peer.addr, err)
			return
		}

		switch msgType {
		case MsgTypeVersion:
			n.handleVersion(peer, payload)
		case MsgTypeVerack:
			fmt.Printf("Received verack from %s\n", peer.addr)
		case MsgTypePing:
			n.handlePing(peer, payload)
		case MsgTypePong:
			fmt.Printf("Received pong from %s\n", peer.addr)
		case MsgTypeAddr:
			n.handleAddr(payload)
		case MsgTypeTx:
			n.handleTx(payload)
		default:
			fmt.Printf("Unknown message type %d from %s\n", msgType, peer.addr)
		}
	}
}

func (n *Node) writeLoop(peer *Peer) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-peer.disconnect:
			return
		case msg := <-peer.msgChan:
			_, err := peer.conn.Write(msg)
			if err != nil {
				fmt.Printf("write error to %s: %v\n", peer.addr, err)
				return
			}
		case <-ticker.C:
			n.sendPing(peer)
		}
	}
}

func (n *Node) maintainConnections() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		n.mu.RLock()
		peerCount := len(n.peers)
		n.mu.RUnlock()

		if peerCount < 10 {
			n.connectToPeers()
		}
	}
}

func (n *Node) connectToPeers() {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for addr, lastSeen := range n.addrBook {
		if _, ok := n.peers[addr]; !ok && time.Since(lastSeen) < 3*time.Hour {
			go n.connectToPeer(addr)
		}
	}
}

func (n *Node) connectToPeer(addr string) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("Failed to connect to %s: %v\n", addr, err)
		return
	}
	n.handleConnection(conn, false)
}

func (n *Node) sendMessage(peer *Peer, msgType byte, payload []byte) {
	msg := make([]byte, len(payload)+5)
	binary.LittleEndian.PutUint32(msg[0:4], uint32(len(payload)+1))
	msg[4] = msgType
	copy(msg[5:], payload)
	peer.msgChan <- msg
}

func (n *Node) readMessage(conn net.Conn) (byte, []byte, error) {
	lenBuf := make([]byte, 4)
	_, err := conn.Read(lenBuf)
	if err != nil {
		return 0, nil, err
	}

	length := binary.LittleEndian.Uint32(lenBuf)
	if length > 1024*1024 {
		return 0, nil, fmt.Errorf("message too large")
	}

	buf := make([]byte, length)
	_, err = conn.Read(buf)
	if err != nil {
		return 0, nil, err
	}

	return buf[0], buf[1:], nil
}

func (n *Node) sendVersion(peer *Peer) {
	payload := []byte("version:0.1.0")
	n.sendMessage(peer, MsgTypeVersion, payload)
}

func (n *Node) handleVersion(peer *Peer, payload []byte) {
	fmt.Printf("Received version %s from %s\n", string(payload), peer.addr)
	n.sendMessage(peer, MsgTypeVerack, nil)
}

func (n *Node) sendPing(peer *Peer) {
	nonce := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonce, uint64(time.Now().UnixNano()))
	n.sendMessage(peer, MsgTypePing, nonce)
}

func (n *Node) handlePing(peer *Peer, payload []byte) {
	n.sendMessage(peer, MsgTypePong, payload)
}

func (n *Node) handleAddr(payload []byte) {
	// Simple implementation: assume payload is a list of IP:Port strings
	addrs := string(payload)
	for _, addr := range strings.Split(addrs, ",") {
		n.mu.Lock()
		n.addrBook[addr] = time.Now()
		n.mu.Unlock()
	}
}

func (n *Node) handleTx(payload []byte) {
	txHash := fmt.Sprintf("%x", sha256.Sum256(payload))
	n.mu.Lock()
	if _, exists := n.txMempool[txHash]; !exists {
		n.txMempool[txHash] = payload
		n.mu.Unlock()
		n.relayTx(txHash, payload)
	} else {
		n.mu.Unlock()
	}
}

func (n *Node) relayTx(txHash string, payload []byte) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, peer := range n.peers {
		n.sendMessage(peer, MsgTypeTx, payload)
	}
}

func main() {
	node := NewNode(":8333")
	err := node.Start()
	if err != nil {
		fmt.Printf("Failed to start node: %v\n", err)
		return
	}

	// Keep the main goroutine running
	select {}
}
