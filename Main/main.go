package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	connman "github.com/RahulMishra0722/GC/Connman"
)

func main() {
	// Initialize the connection manager with a listening address
	cm := connman.Connman{
		ListenAddr: ":8080", // Set the desired port here
	}

	// Start the connection manager and handle errors
	if err := cm.Start(); err != nil {
		log.Fatalf("Error starting connection manager: %v", err)
	}

	fmt.Println("Connection manager started. Listening on", cm.ListenAddr)

	// Set up a channel to listen for OS interrupts for graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	<-stopChan

	fmt.Println("Shutting down connection manager...")

	// Stop the connection manager gracefully
	if err := cm.Stop(); err != nil {
		log.Printf("Error stopping connection manager: %v", err)
	}
	fmt.Println("Connection manager stopped.")
}
