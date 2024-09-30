package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

// Server ...
type Server struct {
	host string
	port string
}

// Client ...
type CNode struct {
	conn             net.Conn
	VSend            chan string
	ExchangedVersion bool
}

// Config ...
type Config struct {
	Host string
	Port string
}

//   - - This is the json structure  our seeder expects DoesEverything DoesValidateAndRelay these are optional you only have to send one
//     var req struct {
//     Addr     string `json:"address"`
//     Services struct {
//     DoesEverything       bool `json:"doesEverything"`
//     DoesValidateAndRelay bool `json:"doesValidateAndRelay"`
//     } `json:"services"`
//     }
//
// New ...
func New(config *Config) *Server {
	return &Server{
		host: config.Host,
		port: config.Port,
	}
}
func handleCallSeeder() {

	// get your public facing ip
	// for now we'll use machine's
	url := `http://localhost:8080/ReadyToServe`
	jsonData := `{
	address::8081
	"services": {
	  "doesEverything": true
	}
  }`
	resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	// body, err := io.ReadAll(resp.Body)
}

// Run ...
func (server *Server) Run() {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", server.host, server.port))
	if err != nil {
		log.Fatal(err)
	}
	handleCallSeeder()

	defer listener.Close()
	//Call Seeder and let them know that we are available for work
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		client := &CNode{
			conn:  conn,
			VSend: make(chan string),
		}
		go client.sendVersionMessage()
		go client.handleRequest()
		go client.write()
	}
}

func (cn *CNode) sendVersionMessage() {
	version := "0.1"
	service := "FullNode"
	when := time.Now()
	formattedString := fmt.Sprintf("Version: %s, Service: %s, when: %v", version, service, when)
	cn.VSend <- formattedString
}

func (client *CNode) handleRequest() {
	defer client.conn.Close()
	reader := bufio.NewReader(client.conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from client:", err)
			return
		}

		fmt.Printf("Message incoming: %s", message)
	}
}

func (cn *CNode) write() {
	writer := bufio.NewWriter(cn.conn)

	for {
		msg, ok := <-cn.VSend
		if !ok {
			fmt.Println("Channel closed, stopping message sending")
			return
		}

		messageToSend := msg + "\n"
		_, err := writer.WriteString(messageToSend)
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

func main() {
	config := Config{
		Host: "127.0.0.1",
		Port: "8081",
	}
	server := New(&config)
	server.Run()
}
