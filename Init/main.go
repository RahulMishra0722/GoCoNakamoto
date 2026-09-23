package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/RahulMishra0722/GoCoNakamoto/DBMan"
	"github.com/RahulMishra0722/GoCoNakamoto/RpcServer"
	"github.com/RahulMishra0722/GoCoNakamoto/Server"
)

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	server, err := Server.InItServer()
	if err != nil {
		log.Fatalf("Error occurred initializing server: %v", err)
	}

	rpc := RpcServer.NewRpcServer(server, server.DBMan, make(map[string]DBMan.Transaction))

	router := http.NewServeMux()
	router.HandleFunc("/GetBalance", rpc.GetBalanceHandler)
	router.HandleFunc("/Send", rpc.SendTransaction)
	router.HandleFunc("/SubmitBlock", rpc.SubmitBlock)
	router.HandleFunc("/GetBlockTemplate", rpc.GetBlockTemplate)

	corsRouter := CORSMiddleware(router)

	go func() {
		log.Println("Starting HTTP server on :8087")
		err := http.ListenAndServe(":8087", corsRouter)
		if err != nil {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()
	go server.SendRetryHandler()
	go server.HandleCallSeeder()

	addrMap, err := server.AddrMan.GetAddr()
	if err != nil {
		fmt.Printf("error getting addr %v", err)
	}
	for _, addr := range addrMap {
		go server.Dial(addr)
	}
	go server.DBMan.LogAllTheBlocks()

	select {}
}
