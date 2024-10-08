package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// Server represents the server that listens for incoming connections
type Server struct {
	host         string
	port         string
	peers        []*CNode
	listener     net.Listener
	maxPeerLimit int
}

const (
	Message       = "Pong"
	StopCharacter = "\r\n\r\n"
)

// CNode represents a client node in the network
type CNode struct {
	conn             net.Conn
	VSend            chan string
	ExchangedVersion bool
	LastServed       time.Time
}

func NewCNode() CNode {
	VSend := make(chan string)
	return CNode{
		conn:             &net.TCPConn{},
		VSend:            VSend,
		ExchangedVersion: false,
		LastServed:       time.Time{},
	}
}

// Config represents server configuration
type Config struct {
	Host string
	Port string
}

// New initializes a new server with the given configuration
func New(config *Config) *Server {
	return &Server{
		host: config.Host,
		port: config.Port,
	}
}

// handleCallSeeder sends a request to the seeder to inform it that this node is ready to serve
func (s *Server) handleCallSeeder() {
	url := `http://localhost:8082/ReadyToServe`
	jsonData := `{
		"address": "127.0.0.1:8081",
		"services": {
			"doesEverything": true
		}
	}`

	resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		fmt.Println("Error sending request to seeder:", err)
		s.Close()
		log.Fatal("Server stopped due to seeder communication failure")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Unexpected status code from seeder: %d\n", resp.StatusCode)
		s.Close()
		log.Fatal("Server stopped due to unexpected response from seeder")
	}

	go s.getActivePeers()
}

// getActivePeers sends a request to the seeder to get a list of active peers
func (s *Server) getActivePeers() {
	url := `http://localhost:8082/NeedPeers`
	jsonData := `{
		"servicesNeeded": {
			"doesEverything": true
		}
	}`

	resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		fmt.Println("Error getting active peers:", err)
		s.Close()
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	var addresses []string
	err = json.Unmarshal(body, &addresses)
	if err != nil {
		fmt.Print(err)
	}
	for _, addr := range addresses {
		//Make addr man and tell it to store it
		fmt.Print(addr)
	}

}

// Run starts the server and listens for incoming connections
func (s *Server) Run() {
	listener, err := net.Listen("tcp4", s.host+":"+s.port)
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
	s.listener = listener
	defer listener.Close()

	log.Println("Server is running on 127.0.0.1:8081")
	s.handleCallSeeder()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		log.Printf("Connected to %s\n", conn.RemoteAddr())
		newNode := NewCNode()
		newNode.conn = conn

		s.peers = append(s.peers, &newNode)
		go newNode.write()
		go newNode.handler(conn)

	}

}

func (cn *CNode) handler(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading from connection: %v", err)
			return
		}

		message = strings.TrimSpace(message)
		log.Printf("Received: %s", message)

		if isTransportOver(message) {
			break
		}

		if message == "Ping" {
			response := "Pong\n"
			cn.VSend <- response
			log.Printf("Sent: %s", strings.TrimSpace(response))
		}

	}

	log.Println("Connection closed")
}

func isTransportOver(data string) bool {
	return strings.HasSuffix(data, StopCharacter)
}

// write sends messages to the client
func (cn *CNode) write() {
	writer := bufio.NewWriter(cn.conn)

	for {
		msg, ok := <-cn.VSend
		if !ok {
			fmt.Println("Channel closed, stopping message sending")
			return
		}

		fmt.Printf("Writing message %s", msg)
		_, err := writer.Write([]byte(msg))
		if err != nil {
			fmt.Println("Error writing to client:", err)
			return
		}

		err = writer.Flush()
		if err != nil {
			fmt.Println("Error flushing writer:", err)
			return
		}
		fmt.Println("Message flushed to client")
	}
}

// Close stops the server listener
func (s *Server) Close() {
	s.listener.Close()
}

func main() {
	config := Config{
		Host: "127.0.0.1",
		Port: "8081",
	}

	server := New(&config)
	server.maxPeerLimit = 20
	server.Run()
}
