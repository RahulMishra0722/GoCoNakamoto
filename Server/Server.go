package Server

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
	"github.com/RahulMishra0722/GC/Utils"
)

type CMD uint8

// make the node send its lastBlockHash nvm i was gonna say with height but thats like one sec look in db
const (
	VERSION     CMD = 0x01 // version
	VERACK      CMD = 0x02 // version acknowledgement
	GETBLOCKS   CMD = 0x03 // request blocks
	INV         CMD = 0x04 // inventory
	GETDATA     CMD = 0x05 // request data
	BLOCK       CMD = 0x06 // block
	TX          CMD = 0x07 // transaction
	PING        CMD = 0x08 // ping
	PONG        CMD = 0x09 // pong
	REJECT      CMD = 0x0A // reject
	ADDR        CMD = 0x0B // addresses
	GETADDR     CMD = 0x0C // get addresses
	MEMPOOL     CMD = 0x0D // mempool request
	ALERT       CMD = 0x0E // alert
	FEEFILTER   CMD = 0x0F // fee filter
	SENDHEADERS CMD = 0x10 // send headers
	SENDCMPCT   CMD = 0x11 // send compact
	GETHEADERS  CMD = 0x12 // get headers
	HEADERS     CMD = 0x13 // headers
)

// // CommandToString maps command identifiers to their string representations
// var CommandToString = map[CMD]string{
// 	VERSION:     "version",
// 	VERACK:      "verack",
// 	GETBLOCKS:   "getblocks",
// 	INV:         "inv",
// 	GETDATA:     "getdata",
// 	BLOCK:       "block",
// 	TX:          "tx",
// 	PING:        "ping",
// 	PONG:        "pong",
// 	REJECT:      "reject",
// 	ADDR:        "addr",
// 	GETADDR:     "getaddr",
// 	MEMPOOL:     "mempool",
// 	ALERT:       "alert",
// 	FEEFILTER:   "feefilter",
// 	SENDHEADERS: "sendheaders",
// 	SENDCMPCT:   "sendcmpct",
// 	GETHEADERS:  "getheaders",
// 	HEADERS:     "headers",
// }

const (
	VERSION_BYTE = 1
	CMD_BYTE     = 1
	LENGTH_BYTES = 2
	version      = 0.1
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
	Version     float64
	Services    uint64
	Timestamp   time.Time
	AddrFrom    net.TCPAddr
	Nonce       uint64
	UserAgent   string
	StartHeight int32
	Relay       bool
}
type ErrorType int

const (
	ErrorNetwork ErrorType = iota
	ErrorDatabase
	ErrorProtocol
	ErrorInternal
)

type ServerError struct {
	Type    ErrorType
	Message string
	Err     error
}

func NewMessage() Message {
	return Message{}
}
func (e *ServerError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func NewServerError(errType ErrorType, message string, err error) *ServerError {
	return &ServerError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}
func NewVersionMessage() VersionMessage {

	return VersionMessage{
		Version:     version,
		Services:    Service,
		StartHeight: 0,
		Timestamp:   time.Now(),
		AddrFrom:    net.TCPAddr{},
		Nonce:       uint64(GenerateRandomId()),
		Relay:       true,
	}
}

const (
	StopCharacter = "\r\n\r\n"
	Service       = 0x012 //* This node / all the nodes will only do fullNode since it does everything
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
	Addr             net.Addr
	ExchangedVersion bool
	LastServed       time.Time
	Utils            *Utils.Utils
	server           *Server
}

func (cn *CNode) Close() {
	close(cn.done)
	close(cn.VSend)
	cn.conn.Close()
}
func NewCNode(server *Server) CNode {
	VSend := make(chan []byte)
	return CNode{
		Id:               GenerateRandomId(),
		conn:             &net.TCPConn{},
		VSend:            VSend,
		ExchangedVersion: false,
		Addr:             &net.TCPAddr{},
		done:             make(chan struct{}),
		LastServed:       time.Time{},
		Utils:            &Utils.Utils{},
		server:           server,
	}
}

// Todo Frame messages
// Todo when a tx is requested back from the peer it will send it to to its know peers and one of them is us and
// * thus itll send back what we sent them thats redundunt we wont let it in ofc because we check if its in the mempool already
// Todo [add another check if it has already been mined been mined in a block already]
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

var ServerAddr string // Run starts the server and listens for incoming connections
func (s *Server) Run() {
	listener, err := net.Listen("tcp4", s.host+":"+s.port)
	if err != nil {
		s.handleError(NewServerError(ErrorNetwork, "error starting server Err", err))
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
				s.handleError(NewServerError(ErrorNetwork, "Error accepting connection:", err))
				continue
			}
			log.Printf("New connection accepted from %s", conn.RemoteAddr())

			newNode := NewCNode(s)
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
	conn, err := net.DialTimeout("tcp4", addr, 5*time.Second)
	if err != nil {
		message := fmt.Sprintf("error dialing %s: %v", addr, err)
		s.handleError(NewServerError(ErrorNetwork, message, err))
		return false
	}
	log.Printf("Successfully connected to %s", addr)

	newCNode := NewCNode(s)
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

// ! have to shift these to utils
func dataByteLen(data []byte) uint16 {
	return uint16(len(data))
}

func encodeMessage(Message Message) []byte {
	totalLength := CMD_BYTE + Message.Length + VERSION_BYTE + LENGTH_BYTES
	buffer := make([]byte, totalLength)
	buffer[0] = Message.Version
	buffer[1] = Message.Cammand // Make sure this is correct
	binary.BigEndian.PutUint16(buffer[2:4], Message.Length)
	copy(buffer[4:], Message.Data)
	fmt.Printf("Encoded Command: %d\n", Message.Cammand)
	return buffer
}

func decodeMessage(reader io.Reader) (Message, error) {
	header := make([]byte, VERSION_BYTE+CMD_BYTE+LENGTH_BYTES)
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return Message{}, err
	}
	length := binary.BigEndian.Uint16(header[2:4])
	data := make([]byte, length)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		Version: header[0],
		Cammand: header[1],
		Length:  length,
		Data:    data,
	}
	fmt.Printf("Decoded Command: %d\n", msg.Cammand)
	return msg, nil
}
func (cn *CNode) SendVerack() error {
	log.Printf("Sending Version Acknowledgement to %s", cn.conn.RemoteAddr())
	Verack := NewMessage()
	Verack.Cammand = byte(VERACK)
	Verack.Length = 0
	encodedBuff := encodeMessage(Verack)
	if _, err := cn.conn.Write(encodedBuff); err != nil {
		message := fmt.Sprintf("error occured sending Verack Message: Err:%v", err)
		cn.server.handleError(NewServerError(ErrorNetwork, message, err))
		return fmt.Errorf("error occured sending Verack Message: Err:%v", err)
	}
	return nil
}
func (cn *CNode) SendVersionMessage() error {
	log.Printf("Sending Version message to %s", cn.conn.RemoteAddr())
	//!New message protocol
	//*[Version | Cammand | Length | Data ]
	if !cn.ExchangedVersion {
		var data = NewVersionMessage()
		var buff bytes.Buffer

		endr := gob.NewEncoder(&buff)
		err := endr.Encode(data)
		if err != nil {
			cn.server.handleError(NewServerError(ErrorNetwork, "error encoding Version Message", err))
		}

		version := NewMessage()
		version.Cammand = byte(VERSION)
		version.Length = dataByteLen(buff.Bytes())
		version.Data = buff.Bytes()
		encodedbuff := encodeMessage(version)
		_, err = cn.conn.Write(encodedbuff)
		if err != nil {
			cn.server.handleError(NewServerError(ErrorNetwork, "error Writing Version Message", err))
			return err
		}
		cn.ExchangedVersion = true
	}
	return nil
}
func (cn *CNode) OnGetDataReqTX(hash string) error {
	fmt.Printf("INCOMMING-HASH:%s\n", (hash))
	txn, err := cn.server.DBMan.GetTxnFromMempool(hash)

	fmt.Printf("Transaction thats being sent %v", txn)
	if err != nil {
		cn.server.handleError(NewServerError(ErrorDatabase, "error occured trying to get txn from DB", err))
	}

	encoded, err := cn.Utils.Encode(txn)

	if err != nil {
		cn.server.handleError(NewServerError(ErrorInternal, "OnGetDataReqTX error encoding transaction", err))

	}
	if len(encoded) == 0 {
		fmt.Printf("encoded transaction is empty")

	}
	length := dataByteLen(encoded)
	txnData := NewMessage()
	txnData.Cammand = byte(TX)
	txnData.Data = encoded
	txnData.Length = length
	txnData.Version = 10 // gotta figure out ho to manage this dynamically also on what premise
	data := encodeMessage(txnData)

	cn.VSend <- data
	return nil
}
func (s *Server) handleError(err error) {
	serverErr, ok := err.(*ServerError)
	if !ok {
		serverErr = NewServerError(ErrorInternal, "Unknown Error Ocuured", err)
	}
	//Todo Figure out when to restart the proccess and on what premise
	switch serverErr.Type {
	case ErrorNetwork:
		log.Printf("Network error : %v", serverErr)
	case ErrorDatabase:
		log.Printf("Database Error : %v", serverErr)
	case ErrorProtocol:
		log.Printf("Protocol Error : %v", serverErr)
	case ErrorInternal:
		log.Printf("Internal Error : %v", serverErr)
	}
}
func (cn *CNode) handler() {
	defer cn.conn.Close()
	log.Printf("Starting handler for connection from %s", cn.conn.RemoteAddr())
	for {

		msg, err := decodeMessage(cn.conn)
		if err == io.EOF {
			break
		}

		fmt.Printf("Decoded Command: %d\n", msg.Cammand)

		if isTransportOver(string(msg.Data)) {
			break
		}
		if err != nil {
			message := fmt.Sprintf("error Decoding Message From Peer %v", cn.conn.RemoteAddr())
			cn.server.handleError(NewServerError(ErrorNetwork, message, err))
		}

		cn.Addr = cn.conn.RemoteAddr()
		if msg.Cammand == byte(VERSION) {
			var buff VersionMessage

			decoder := gob.NewDecoder(bytes.NewReader(msg.Data))
			decoder.Decode(&buff)

			if !cn.ExchangedVersion {
				if err := cn.SendVerack(); err != nil {
					errMessage := fmt.Sprintf("Error sending version ackowledgement message to peer %v", cn.conn.RemoteAddr())
					cn.server.handleError(NewServerError(ErrorNetwork, errMessage, err))
				}

			}
		} else if msg.Cammand == byte(VERACK) {
			fmt.Printf("The Verack \n")
			if !cn.ExchangedVersion {
				cn.Addr = cn.conn.RemoteAddr()
			}
		} else if msg.Cammand == byte(GETBLOCKS) {
			// gotta find a way to call this through server not re intialise it it looks ugly

			blocks, err, _ := cn.server.DBMan.GetBlocksFromGivenHashTillLatest(string(msg.Data))
			if err != nil {
				cn.server.handleError(NewServerError(ErrorDatabase, "error occured Trying to get Blocks from Db for Transaport", err))
			}
			length := dataByteLen(blocks)
			msg := NewMessage()
			msg.Cammand = byte(BLOCK)
			msg.Data = blocks
			msg.Length = length
			encoded := encodeMessage(msg)
			cn.VSend <- encoded
		} else if msg.Cammand == byte(INV) {
			// * check in db if we already have this txn

			exists, err := cn.server.DBMan.IsInMempool(string(msg.Data))

			if err != nil {
				cn.server.handleError(NewServerError(ErrorDatabase, "error occured Trying to get Blocks from Db for Transaport", err))
			}
			HasAlreadybeenMiner, err := cn.server.DBMan.HasTxnAlreadyBeenMined(string(msg.Data))
			errMessage := fmt.Sprintf("error checking if the block has already been miner %v", err)
			cn.server.handleError(NewServerError(ErrorDatabase, errMessage, err))
			if !exists && !HasAlreadybeenMiner {
				cn.RequestTxn(msg.Data)
			}
		} else if msg.Cammand == byte(GETDATA) {
			go cn.OnGetDataReqTX(string(msg.Data))
		} else if msg.Cammand == byte(TX) {

			var txn DBMan.Transaction
			decodedTxn, err := cn.Utils.Decode(msg.Data, &txn)
			if err != nil {
				cn.server.handleError(NewServerError(ErrorProtocol, "error decoding message from peer", err))
			}
			cn.server.StoreInMempoolAndRelay(decodedTxn)
		}

	}
}

func (cn *CNode) RequestTxn(txn []byte) error {
	getDataRequest := NewMessage()
	length := dataByteLen(txn)
	getDataRequest.Cammand = byte(GETDATA)
	getDataRequest.Data = txn
	getDataRequest.Length = length
	encodedGetDataRequest := encodeMessage(getDataRequest)

	cn.VSend <- encodedGetDataRequest
	return nil
}
func (s *Server) RelayToKnowPeers(transaction DBMan.Transaction) {
	// first send the hash
	// Todo need more logs this sucks
	newRelay := NewMessage()
	data := []byte(transaction.TxnHash)
	length := dataByteLen(data)
	newRelay.Cammand = byte(INV)
	newRelay.Data = data
	newRelay.Length = length
	encoded := encodeMessage(newRelay)
	for _, peer := range s.peers {
		peer.VSend <- encoded
	}

}

// ! if added call a relay function and send the txn hashes in mempool and they will check in their db do they already have any of the given txn that maches the hashes provided.
// ! or give them all and let em handle theyll just drop it if they already have it i takes one call so i think its faster ill still go with first this one will end up sending alot of redundant data across and the overhead will be insane
func (s *Server) StoreInMempoolAndRelay(transaction DBMan.Transaction) (error, bool) {

	added, err := s.DBMan.VerifyTransaction(transaction)
	if err != nil {
		return NewServerError(ErrorDatabase, "error verifying transaction", err), false
	}
	if !added {
		return err, false
	}
	if added {
		s.RelayToKnowPeers(transaction)
	}
	return nil, added
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
			errMessage := fmt.Sprintf("Channel closed for %s, stopping message sending", cn.conn.RemoteAddr()) //* redundant
			cn.server.handleError(NewServerError(ErrorNetwork, errMessage, fmt.Errorf("channel closed for %s, stopping message sending", cn.conn.RemoteAddr())))
			return
		}

		log.Printf("Writing message of length %d to %s", len(msg), cn.conn.RemoteAddr())
		_, err := writer.Write(msg)
		if err != nil {
			errMessage := fmt.Sprintf("error writing to : %v", cn.conn.RemoteAddr())
			cn.server.handleError(NewServerError(ErrorNetwork, errMessage, err))
		}

		err = writer.Flush()
		if err != nil {
			errMessage := fmt.Sprintf("Error flushing writer for %s: %v", cn.conn.RemoteAddr(), err)
			cn.server.handleError(NewServerError(ErrorNetwork, errMessage, err))
		}
		log.Printf("Message flushed to %s", cn.conn.RemoteAddr())
	}
}

func InItServer() (Server, error) {
	log.Println("Starting server application")
	//! Problem we are intialising db too many times in handler its ugly after test fix it
	//Todo
	db, err := DBMan.GetDB("../../testDB")
	if err != nil {
		return Server{}, fmt.Errorf("error occurred initializing db: %w", err)
	}

	config := Config{
		Host:  "127.0.0.1",
		Port:  "8082",
		DBMan: db,
	}
	server := New(&config)
	server.maxPeerLimit = 20

	log.Printf("Server configured with max peer limit: %d", server.maxPeerLimit)
	go func() {
		server.Run()
	}()
	return *server, nil
}
