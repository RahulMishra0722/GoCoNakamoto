package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/RahulMishra0722/GC/DBMan"
	"github.com/RahulMishra0722/GC/RpcServer"
	"github.com/RahulMishra0722/GC/Server"
)

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

func main() {
	server, err := Server.InItServer()
	if err != nil {
		log.Fatalf("Error occurred initializing server: %v", err)
	}
	prevHash, err := server.DBMan.GetPrevHash()

	fmt.Println("Previous Block Hash %s\n  error %w\n", prevHash, err)
	rpc := RpcServer.NewRpcServer(&server, server.DBMan, make(map[string]DBMan.Transaction))

	router := http.NewServeMux()
	router.HandleFunc("/GetBalance", rpc.GetBalanceHandler)
	router.HandleFunc("/Send", rpc.SendTransaction)
	router.HandleFunc("/SubmitBlock", rpc.SubmitBlock)
	router.HandleFunc("/GetBlockTemplate", rpc.GetBlockTemplate)

	// Wrap the router with the CORS middleware
	corsRouter := CORSMiddleware(router)

	// Start HTTP server in a goroutine
	go func() {
		log.Println("Starting HTTP server on :8087")
		err := http.ListenAndServe(":8087", corsRouter)
		if err != nil {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	log.Println("Starting server application")

	fmt.Println("Sleeping for 2 seconds before calling")
	time.Sleep(2 * time.Second)
	server.Dial("127.0.0.1:8081")
	rpc.Connman.DBMan.LogAllTheBlocks()
	select {}
}
