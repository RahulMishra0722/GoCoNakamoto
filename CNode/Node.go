package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/RahulMishra0722/GC/DBMan"
)

type MessageType int

const (
	RequestLatestBlock   MessageType = 0x01
	SendLatestBlock      MessageType = 0x02
	BroadcastTransaction MessageType = 0x03
	PeerDiscovery        MessageType = 0x04
	Version              MessageType = 0x05
)

const (
	VERSION_BYTE = 1
	CMD_BYTE     = 1
	LENGTH_BYTE  = 2
)

// Server represents the server that listens for incoming connections
type Server struct {
	DBMan        *DBMan.DBman
	host         string
	port         string
	peers        []*CNode
	listener     net.Listener
	done         chan struct{}
	maxPeerLimit int
}
type VersionMessage struct {
	Id          int       //Random nonce for self-connection detection
	StartHeight int32     //Last block height known by the node
	Version     int       //Software version idk how i would manage this
	TimeStamp   time.Time // When did we do the handshake
	Addr        string    //Sending node’s IP and port we would do reciving if we were lisning on multiple addresses
	Relay       bool      //Flag to indicate if the node wants transaction relays
}

func NewVersionMessage() VersionMessage {
	Id := GenerateRandomId()
	// db, err := DBMan.GetDB()
	// if err != nil {
	// 	fmt.Print(err)
	// }

	return VersionMessage{
		Id:          Id,
		Version:     1,
		TimeStamp:   time.Now(),
		Addr:        "127.0.0.1:8082",
		StartHeight: 0,
		Relay:       true,
	}
}

const (
	StopCharacter = "\r\n\r\n"
	Service       = "FullNode" //* This node / all the nodes will only do fullNode since it does everything
)

type Message struct {
	Version byte
	Cammand byte
	Length  uint16
	Data    []byte
}

// CNode represents a client node in the network
type CNode struct {
	Id               int
	conn             net.Conn
	done             chan struct{}
	VSend            chan []byte
	Addr             string
	ExchangedVersion bool
	LastServed       time.Time
}

func (cn *CNode) Close() {
	close(cn.done)
	close(cn.VSend)
	cn.conn.Close()
}
func NewCNode() CNode {
	VSend := make(chan []byte)
	return CNode{
		Id:               GenerateRandomId(),
		conn:             &net.TCPConn{},
		VSend:            VSend,
		ExchangedVersion: false,
		Addr:             "",
		done:             make(chan struct{}),
		LastServed:       time.Time{},
	}
}

// Config represents server configuration
type Config struct {
	Host  string
	Port  string
	DBMan *DBMan.DBman
}

// New initializes a new server with the given configuration
func New(config *Config) *Server {
	return &Server{
		host:  config.Host,
		port:  config.Port,
		DBMan: config.DBMan,
		done:  make(chan struct{}),
	}
}

// func SendMessageEnd() {
// 	endMessage := Message{}
// 	endMessage.Cammand
// }

var ServerAddr string // Run starts the server and listens for incoming connections
func (s *Server) Run() {
	listener, err := net.Listen("tcp4", s.host+":"+s.port)
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
	s.listener = listener
	defer listener.Close()

	log.Printf("Server is running on %s:%s", s.host, s.port)

	for {
		select {
		case <-s.done:
			log.Println("Server is shutting down")
			return
		default:
			conn, err := listener.Accept()

			if err != nil {
				log.Println("Error accepting connection:", err)
				continue
			}
			log.Printf("New connection accepted from %s", conn.RemoteAddr())

			newNode := NewCNode()
			newNode.conn = conn

			s.peers = append(s.peers, &newNode)
			log.Printf("New peer added. Total peers: %d", len(s.peers))

			go newNode.write()
			go newNode.handler()
		}
	}
}
func (s *Server) Close() {
	close(s.done)
	s.listener.Close()
	for _, peer := range s.peers {
		peer.Close()
	}
}
func (s *Server) Dial(addr string) bool {
	log.Printf("Attempting to dial %s", addr)
	conn, err := net.Dial("tcp4", addr)
	if err != nil {
		log.Printf("Error dialing %s: %v", addr, err)
		return false
	}
	log.Printf("Successfully connected to %s", addr)

	newCNode := NewCNode()
	newCNode.conn = conn
	go newCNode.write()

	go newCNode.handler()
	newCNode.SendVersionMessage()

	s.peers = append(s.peers, &newCNode)

	log.Printf("New outgoing peer added. Total peers: %d", len(s.peers))
	return true
}
func GenerateRandomId() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(10000) //generates an between 1 - 10000
}
func GetVersionMessageSize(VersionMessage VersionMessage) int {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	encoder.Encode(VersionMessage)
	return len(buff.Bytes())
}

func (cn *CNode) SendVersionMessage() {
	log.Printf("Sending Version message to %s", cn.conn.RemoteAddr())
	//!New message protocol
	//*[Version | Cammand | Length | Data ]
	if !cn.ExchangedVersion {

		version := NewVersionMessage()
		var buff bytes.Buffer

		// Encode the message type
		err := binary.Write(&buff, binary.BigEndian, uint16(Version))
		if err != nil {
			log.Printf("Error encoding message type: %v", err)
			return
		}

		// Encode the version message
		versionEnc := gob.NewEncoder(&buff)
		err = versionEnc.Encode(version)
		if err != nil {
			log.Printf("Error encoding Version message: %v", err)
			return
		}

		// Get the length of the encoded message (excluding the message type)
		messageLength := uint16(buff.Len() - 2)

		// Create a new buffer with the correct order: type, length, message
		var finalBuff bytes.Buffer
		binary.Write(&finalBuff, binary.BigEndian, uint16(Version))
		binary.Write(&finalBuff, binary.BigEndian, messageLength)
		finalBuff.Write(buff.Bytes()[2:]) // Skip the message type we wrote earlier

		// Append the stop character
		finalBuff.WriteString(StopCharacter)

		// Send the message
		_, err = cn.conn.Write(finalBuff.Bytes())
		if err != nil {
			log.Printf("Error sending Version message: %v", err)
			return
		}
		cn.ExchangedVersion = true
		log.Printf("Version message sent to %s", cn.conn.RemoteAddr())
	}
}
func (cn *CNode) ReadVersionMessage(VersionMessage VersionMessage) {
	log.Printf("Received Version message from %s", cn.conn.RemoteAddr())
	log.Printf("Version details - ID: %d, Addr: %v, Relay: %v, StartHeight: %d, Timestamp: %v",
		VersionMessage.Id, VersionMessage.Addr, VersionMessage.Relay, VersionMessage.StartHeight, VersionMessage.TimeStamp)
	cn.Addr = string(VersionMessage.Addr)

	cn.SendVersionMessage()
}

func (cn *CNode) handler() {
	defer cn.conn.Close()
	log.Printf("Starting handler for connection from %s", cn.conn.RemoteAddr())

	reader := bufio.NewReader(cn.conn)

	for {

		messageTypeBytes := make([]byte, 2)
		_, err := io.ReadFull(reader, messageTypeBytes)
		if err != nil {
			log.Printf("Error reading message type from %s: %v", cn.conn.RemoteAddr(), err)
			return
		}
		messageType := binary.BigEndian.Uint16(messageTypeBytes)

		// Read message length (2 bytes)
		messageLengthBytes := make([]byte, 2)
		_, err = io.ReadFull(reader, messageLengthBytes)
		if err != nil {
			log.Printf("Error reading message length from %s: %v", cn.conn.RemoteAddr(), err)
			return
		}
		messageLength := binary.BigEndian.Uint16(messageLengthBytes)

		log.Printf("Received message of type %d with length %d from %s", messageType, messageLength, cn.conn.RemoteAddr())

		// Read message data
		messageData := make([]byte, messageLength)
		_, err = io.ReadFull(reader, messageData)
		if err != nil {
			log.Printf("Error reading message data from %s: %v", cn.conn.RemoteAddr(), err)
			return
		}
		if isTransportOver(string(messageData)) {
			break
		}
		//
		// Check for stop character
		stopChar := make([]byte, len(StopCharacter))
		_, err = io.ReadFull(reader, stopChar)
		if err != nil {
			log.Printf("Error reading stop character from %s: %v", cn.conn.RemoteAddr(), err)
			return
		}

		if string(stopChar) != StopCharacter {
			log.Printf("Invalid stop character received from %s", cn.conn.RemoteAddr())
			return
		}

		switch MessageType(messageType) {
		case Version:
			log.Printf("Received Version message from %s", cn.conn.RemoteAddr())
			var versionMessage VersionMessage
			decoder := gob.NewDecoder(bytes.NewReader(messageData))
			err := decoder.Decode(&versionMessage)
			if err != nil {
				log.Printf("Error decoding Version message from %s: %v", cn.conn.RemoteAddr(), err)
				return
			}
			cn.ReadVersionMessage(versionMessage)
		// ... (handle other message types)
		default:
			log.Printf("Unknown MessageType %d received from %s", messageType, cn.conn.RemoteAddr())
		}
	}
}
func (s *Server) LogPeers() {
	time.Sleep(10 * time.Second)
	for _, p := range s.peers {
		fmt.Print("Peers:\n")
		fmt.Printf("%s\n", p.Addr)
		fmt.Printf("%d\n", p.Id)
		fmt.Printf("%v\n", p.LastServed)
	}
}

func isTransportOver(data string) bool {
	return strings.HasSuffix(data, StopCharacter)
}

func (cn *CNode) write() {
	writer := bufio.NewWriter(cn.conn)
	log.Printf("Starting write routine for connection to %s", cn.conn.RemoteAddr())

	for {
		msg, ok := <-cn.VSend
		if !ok {
			log.Printf("Channel closed for %s, stopping message sending", cn.conn.RemoteAddr())
			return
		}

		log.Printf("Writing message of length %d to %s", len(msg), cn.conn.RemoteAddr())
		_, err := writer.Write(msg)
		if err != nil {
			log.Printf("Error writing to %s: %v", cn.conn.RemoteAddr(), err)
			return
		}

		err = writer.Flush()
		if err != nil {
			log.Printf("Error flushing writer for %s: %v", cn.conn.RemoteAddr(), err)
			return
		}
		log.Printf("Message flushed to %s", cn.conn.RemoteAddr())
	}
}

// Close stops the server listener
func main() {
	log.Println("Starting server application")

	config := Config{
		Host: "127.0.0.1",
		Port: "8082",
	}

	server := New(&config)
	server.maxPeerLimit = 20

	log.Printf("Server configured with max peer limit: %d", server.maxPeerLimit)

	go server.Run()

	log.Println("Waiting 5 seconds before dialing")
	time.Sleep(5 * time.Second)

	go server.Dial("127.0.0.1:8081")
	server.LogPeers()
	// Keep the main goroutine running
	select {}
}
