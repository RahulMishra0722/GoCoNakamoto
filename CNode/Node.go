package CNode

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Services int64

const (
	FullNode Services = iota
	TransactionAndRelay
)

type CNode struct {
	ConnectedAt      time.Time
	Socket           net.Conn
	addr             net.Addr
	LastSentTo       time.Time
	vRecv            []byte
	vSend            []byte
	LastReceivedFrom time.Time
	HaveHandShaked   bool
	Services         Services
	LastPinged       time.Time
	fDisconnect      bool
	muSend           sync.Mutex
	muRecv           sync.Mutex
}

var (
	vNodes             []*CNode
	cs_vNodes          sync.Mutex
	vNodesDisconnected []*CNode
	nPrevNodeCount     int
	vfThreadRunning    [1]bool
	addrLocalHost      net.IP
	vRecv, vSend       []byte
)

func ThreadSocketHandler(hListenSocket net.Listener) {
	// Listen for incoming connections
	for {
		conn, err := hListenSocket.Accept()
		if err != nil {
			fmt.Printf("Accept failed: %v\n", err)
			continue
		}
		fmt.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())

		// Initialize a new CNode with the accepted connection
		pNode := &CNode{
			ConnectedAt: time.Now(),
			Socket:      conn,
			addr:        conn.RemoteAddr(),
		}

		// Locking for thread safety while modifying vNodes
		cs_vNodes.Lock()
		vNodes = append(vNodes, pNode)
		cs_vNodes.Unlock()

		go handleNode(pNode) // Handle each node in a separate goroutine
	}
}

func handleNode(pNode *CNode) {
	// Create a buffer for reading data
	buf := make([]byte, 4096)

	for {
		// Read from the socket
		n, err := pNode.Socket.Read(buf)
		if err != nil {
			fmt.Printf("Read error from %s: %v\n", pNode.addr.String(), err)
			pNode.fDisconnect = true
			break
		}

		// Update received data
		pNode.muRecv.Lock()
		pNode.vRecv = append(pNode.vRecv, buf[:n]...)
		pNode.LastReceivedFrom = time.Now()
		pNode.muRecv.Unlock()

		// Process send queue
		pNode.muSend.Lock()
		if len(pNode.vSend) > 0 {
			n, err := pNode.Socket.Write(pNode.vSend)
			if err != nil {
				fmt.Printf("Write error to %s: %v\n", pNode.addr.String(), err)
				pNode.fDisconnect = true
			} else {
				pNode.vSend = pNode.vSend[n:] // Remove sent bytes
				pNode.LastSentTo = time.Now()
			}
		}
		pNode.muSend.Unlock()

		// Sleep to prevent busy waiting
		time.Sleep(10 * time.Millisecond)
	}

	// Cleanup on disconnect
	cleanupNode(pNode)
}

func cleanupNode(pNode *CNode) {
	cs_vNodes.Lock()
	defer cs_vNodes.Unlock()

	// Remove the node from vNodes
	for i, node := range vNodes {
		if node == pNode {
			vNodes = append(vNodes[:i], vNodes[i+1:]...)
			break
		}
	}

	pNode.Socket.Close() // Close the connection
	fmt.Printf("Disconnected from %s\n", pNode.addr.String())
}
