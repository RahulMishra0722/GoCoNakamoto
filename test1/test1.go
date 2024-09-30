package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	Message       = "Pong"
	StopCharacter = "\r\n\r\n"
)

func SocketServer(port int) {
	listener, err := net.Listen("tcp4", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("Socket listen on port %d failed: %s", port, err)
		os.Exit(1)
	}
	defer listener.Close()
	log.Printf("Listening on port: %d", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept failed:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		data, err := reader.ReadString('\n') // Read until a newline
		if err != nil {
			if err == io.EOF {
				log.Println("Connection closed by client")
				return
			}
			log.Println("Read error:", err)
			return
		}

		data = strings.TrimSpace(data) // Remove trailing newline and spaces
		log.Printf("Received: %s", data)

		if data == "Ping" {
			_, err := conn.Write([]byte(Message + "\n")) // Respond with Pong
			if err != nil {
				log.Println("Write error:", err)
				return
			}
			log.Printf("Sent: %s", Message)
		}

		// Check if the end of the transmission is reached
		if strings.HasSuffix(data, StopCharacter) {
			log.Println("End of transmission detected.")
			break
		}
	}
}

func main() {
	port := 3333
	SocketServer(port)
}
