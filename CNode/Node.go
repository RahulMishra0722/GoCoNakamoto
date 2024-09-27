package cnode

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Constants
const (
	Version           = 0.1
	DefaultBufferSize = 4096
)

// Services represents the services offered by a node
type Services uint64

const (
	FullNode Services = 1 << iota
	TransactionAndRelay
)

// MessageType represents different types of messages
type MessageType uint32

const (
	Addr MessageType = iota
	Inv
	GetData
	GetBlocks
	Tx
	Block
	GetAddr
	CheckOrder
	SubmitOrder
	Reply
)

// CNode represents a connection to a peer in the network
type CNode struct {
	ConnectedAt             time.Time
	Socket                  net.Conn
	InBound                 bool
	Addr                    net.Addr
	LastSentTo              time.Time
	LastReceivedFrom        time.Time
	HaveHandShaked          bool
	Services                Services
	LastPinged              time.Time
	DisconnectRequested     bool
	ExchangedVersionMessage bool
	sendChan                chan []byte
	recvChan                chan []byte
	done                    chan struct{}
}

// MessageWithHeader represents a message with its header
type MessageWithHeader struct {
	Type       MessageType
	DataLength uint32
	Services   Services
	Data       []byte
	Time       time.Time
	Addr       net.Addr
}

// VersionHeaders represents the headers for a version message
type VersionHeaders struct {
	Version  float64
	Services Services
	Time     time.Time
	Addr     net.Addr
}

var (
	nodes             []*CNode
	nodesMutex        sync.RWMutex
	disconnectedNodes []*CNode
)

// NewCNode creates a new CNode
func NewCNode(conn net.Conn, inbound bool) *CNode {
	return &CNode{
		ConnectedAt: time.Now(),
		Socket:      conn,
		Addr:        conn.RemoteAddr(),
		InBound:     inbound,
		sendChan:    make(chan []byte, 100),
		recvChan:    make(chan []byte, 100),
		done:        make(chan struct{}),
	}
}

// Start begins processing for the CNode
func (cn *CNode) Start() {
	go cn.readLoop()
	go cn.writeLoop()
}

func (cn *CNode) readLoop() {
	defer close(cn.recvChan)
	buf := make([]byte, DefaultBufferSize)
	for {
		n, err := cn.Socket.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Read error from %s: %v\n", cn.Addr, err)
			}
			cn.requestDisconnect()
			return
		}
		select {
		case cn.recvChan <- buf[:n]:
			cn.LastReceivedFrom = time.Now()
		case <-cn.done:
			return
		}
	}
}

func (cn *CNode) writeLoop() {
	for {
		select {
		case data := <-cn.sendChan:
			_, err := cn.Socket.Write(data)
			if err != nil {
				fmt.Printf("Write error to %s: %v\n", cn.Addr, err)
				cn.requestDisconnect()
				return
			}
			cn.LastSentTo = time.Now()
		case <-cn.done:
			return
		}
	}
}

func (cn *CNode) SendMessage(msg []byte) {
	select {
	case cn.sendChan <- msg:
	default:
		fmt.Printf("Send buffer full for %s\n", cn.Addr)
	}
}

func (cn *CNode) SendVersionMessage() {
	message := VersionHeaders{
		Version:  Version,
		Services: FullNode,
		Time:     time.Now(),
		Addr:     cn.Addr,
	}
	buffer := make([]byte, 40)
	binary.BigEndian.PutUint64(buffer[0:8], uint64(message.Version*100))
	binary.BigEndian.PutUint64(buffer[8:16], uint64(message.Services))
	binary.BigEndian.PutUint64(buffer[24:32], uint64(message.Time.Unix()))
	if tcpAddr, ok := message.Addr.(*net.TCPAddr); ok {
		// Write IP address
		copy(buffer[20:36], tcpAddr.IP.To16()) // IPv4 or IPv6

		// Write port
		binary.BigEndian.PutUint16(buffer[36:38], uint16(tcpAddr.Port))
	} else {
		// If it's not a TCP address, fill with zeros
		fmt.Printf("Warning: Non-TCP address for node %s\n", cn.Addr.String())
	}
	cn.SendMessage(buffer)

}

func (cn *CNode) requestDisconnect() {
	cn.DisconnectRequested = true
	close(cn.done)
}

func ThreadSocketHandler(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Accept failed: %v\n", err)
			continue
		}
		fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr())

		node := NewCNode(conn, true)
		node.Start()

		nodesMutex.Lock()
		nodes = append(nodes, node)
		nodesMutex.Unlock()
	}
}

func StartNode(port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	go ThreadSocketHandler(ln)
	return nil
}

func CleanupNodes() {
	nodesMutex.Lock()
	defer nodesMutex.Unlock()

	for i := len(nodes) - 1; i >= 0; i-- {
		if nodes[i].DisconnectRequested {
			nodes[i].Socket.Close()
			disconnectedNodes = append(disconnectedNodes, nodes[i])
			nodes = append(nodes[:i], nodes[i+1:]...)
		}
	}
}
