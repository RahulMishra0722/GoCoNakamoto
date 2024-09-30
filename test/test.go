package main

import (
	"fmt"
	"log"
	"net"
	"strings"
)

const (
	message       = "Ping"
	StopCharacter = "\r\n\r\n"
)

func SocketClient(ip string, port int) {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Error connecting to server: %s", err)
	}
	defer conn.Close()

	// Send Ping message
	_, err = conn.Write([]byte(message + "\n")) // Send message with a newline
	if err != nil {
		log.Fatalf("Error sending message: %s", err)
	}
	log.Printf("Sent: %s", message)

	// Wait for response
	buff := make([]byte, 1024)
	n, err := conn.Read(buff)
	if err != nil {
		log.Fatalf("Error reading response: %s", err)
	}

	response := strings.TrimSpace(string(buff[:n])) // Trim whitespace from the response
	if response == "Pong" {
		fmt.Println("Received: Pong")
	} else {
		fmt.Printf("Unexpected response: %s\n", response)
	}
}

func main() {
	ip := "127.0.0.1"
	port := 3333

	SocketClient(ip, port)
}
