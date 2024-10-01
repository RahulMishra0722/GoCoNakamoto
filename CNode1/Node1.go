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
)

// Server represents the server that listens for incoming connections
type Server struct {
	host     string
	port     string
	peers    []*CNode
	listener net.Listener
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
	// For now we'll use the machine's localhost address
	url := `http://localhost:8080/ReadyToServe`
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
		return
	}
	defer resp.Body.Close()
	// if resp.StatusCode == http.StatusOK {
	s.getActivePeers()
	//}

}

// getActivePeers sends a request to the seeder to get a list of active peers
func (s *Server) getActivePeers() {
	url := `http://localhost:8080/NeedPeers`
	jsonData := `{
		"servicesNeeded": {
			"doesEverything": true
		}
	}`

	resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		fmt.Println("Error getting active peers:", err)
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
	listener, err := net.Listen("tcp4", "127.0.0.1:8081")
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
	s.listener = listener
	defer listener.Close()

	log.Println("Server is running on 127.0.0.1:8081")
	go s.handleCallSeeder()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		log.Printf("Connected to %s\n", conn.RemoteAddr())
		client := CNode{
			conn:  conn,
			VSend: make(chan string),
		}
		s.peers = append(s.peers, &client)
		go client.write()
		go client.handler(conn)

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

// func (cn *CNode) handleConnection() {
// 	defer cn.conn.Close()

// 	var (
// 		buf = make([]byte, 1024)
// 		r   = bufio.NewReader(cn.conn)
// 	)

// ILOOP:
// 	for {
// 		n, err := r.Read(buf)
// 		if err != nil {
// 			if err == io.EOF {
// 				break ILOOP
// 			}
// 			log.Fatalf("Receive data failed: %s", err)
// 			return
// 		}

// 		data := string(buf[:n])
// 		log.Println("Receive:", data)

// 		// Check if the received message is a Ping
// 		if strings.TrimSpace(data) == "Ping" {
// 			log.Printf("Received Ping from %s, sending Pong", cn.conn.RemoteAddr())
// 			cn.VSend <- "Pong"

// 			log.Printf("Send: %s", Message)
// 		}

// 		// Check for transport closure
// 		if isTransportOver(data) {
// 			break ILOOP
// 		}
// 	}
// }

// func isTransportOver(data string) bool {
// 	return strings.HasSuffix(data, StopCharacter)
// }

// // write sends messages to the client
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
	server.Run()
}
