package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/RahulMishra0722/GC/RpcServer"
	"github.com/RahulMishra0722/GC/Server"
)

func main() {
	server, err := Server.InItServer()
	if err != nil {
		log.Fatalf("Error occurred initializing server: %v", err)
	}
	// _, err = TestServer.InItServer()
	// if err != nil {
	// 	log.Fatalf("Error occurred initializing Test server: %v", err)
	// }

	rpc := RpcServer.NewRpcServer(&server, server.DBMan)

	router := http.NewServeMux()
	router.HandleFunc("/GetBalance", rpc.GetBalanceHandler)
	router.HandleFunc("/Send", rpc.SendTransaction)

	// Start HTTP server in a goroutine
	go func() {
		log.Println("Starting HTTP server on :8087")
		err := http.ListenAndServe(":8087", router)
		if err != nil {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	log.Println("Starting server application")

	fmt.Println("Sleeping for 5 seconds before calling")
	time.Sleep(2 * time.Second)
	server.Dial("127.0.0.1:8081")
	select {}
}
