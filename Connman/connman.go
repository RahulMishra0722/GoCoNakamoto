package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// Connect to the server
	conn, err := net.Dial("tcp", "127.0.0.1:8081")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Automatically read the version message sent by the server
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading version message from server:", err)
		return
	}
	fmt.Printf("Received version message from server: %s", response)

	for {
		// Read input from the terminal
		fmt.Print("Enter a message to send to the server (or 'quit' to exit): ")
		message, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return
		}

		// Check if the user wants to quit
		if strings.TrimSpace(message) == "quit" {
			fmt.Println("Closing connection...")
			return
		}

		// Send the message to the server
		_, err = conn.Write([]byte(message)) // Directly writing to the connection
		if err != nil {
			fmt.Println("Error sending message:", err)
			return
		}
		fmt.Println("Message sent to server")

		// Wait for server response
		fmt.Println("Waiting for server response...")

		// Read the response from the server
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading response from server:", err)
			return
		}

		// Print server's response
		fmt.Printf("Server response: %s", response)
	}
}
