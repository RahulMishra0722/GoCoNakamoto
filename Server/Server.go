package Server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
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
	VERSION   CMD = 0x01 // version
	VERACK    CMD = 0x02 // version acknowledgement
	GETBLOCKS CMD = 0x03 // request blocks
	INV       CMD = 0x04 // inventory
	GETDATA   CMD = 0x05 // request data
	BLOCK     CMD = 0x06 // block
	TX        CMD = 0x07 // transaction
	PING      CMD = 0x08 // ping
	PONG      CMD = 0x09 // pong
	REJECT    CMD = 0x0A // reject
	ADDR      CMD = 0x0B // addresses
	GETADDR   CMD = 0x0C // get addresses
	MEMPOOL   CMD = 0x0D // mempool request
	//ALERT       CMD = 0x0E // alert
	//FEEFILTER   CMD = 0x0F // fee filter
	//SENDHEADERS CMD = 0x10 // send headers
	//SENDCMPCT   CMD = 0x11 // send compact
	//GETHEADERS  CMD = 0x12 // get headers
	HEADERS         CMD = 0x13 // headers
	BLOCKS          CMD = 0x14 // headers
	REQUESTEDBLOCKS CMD = 0x15
)

const (
	VERSION_BYTE = 1
	CMD_BYTE     = 1
	LENGTH_BYTES = 2
	version      = 0.1
	Difficulty   = 0x1e00ffff
)

//! 0x1e00ffff
/*
 Let's break it down:
 0001 1110 0000 000 1111 1111 1111 1111
 The difficulty is represented in compact form as 0x1e00ffff.
 This compact format splits into two parts:
 1. The **exponent**: 0x1e, which is 30 in decimal.
 2. The **coefficient**: 0x00ffff, which is 65535 in decimal.

 To calculate the target value for mining, we need to convert this compact representation.
 We shift the coefficient left by 8 * (exponent - 3) bits.

 In this case:
 Exponent = 30
 Left shift by 8 * (30 - 3) = 8 * 27 = 216 bits

 This means the coefficient (65535) is multiplied by 2^216.

 The result is a large target value, making it easier to find a valid hash and since our mining function finds hashes target lower difficulty target
 having a bigger number will increase our chances of getting a valid block hash
 In other words, this is a **low difficulty**, meaning mining will be relatively **easy**.
*/
func CompactToBig(compact uint32) *big.Int {
	// !Mental Note
	// The current difficulty is represented in compact form as 0x1c1fffff.
	// In binary, this is: 0001 1100 0001 1111 1111 1111 1111 1111
	// The first 8 bits represent the exponent, and the last 24 bits represent the coefficient.

	// Extract the exponent by right-shifting the compact value by 24 bits.
	// This gives us the first byte (0x1c) as the exponent.
	exponent := uint(compact >> 24)

	// Extract the coefficient using a bitwise AND operation with 0x007fffff.
	// This masks out the upper 8 bits, leaving only the lower 24 bits (the coefficient).
	coefficient := compact & 0x007fffff // we use this to zero out the exponent and extract coefficient
	// In binary, 0x007fffff is: 0000 0111 1111 1111 1111 1111 1111
	// Performing the AND operation results in:
	// 0001 1100 0001 1111 1111 1111 1111 1111 (compact)
	// AND
	// 0000 0000 0111 1111 1111 1111 1111 1111 (mask)
	// --------------------------------------------
	// Resulting in:
	// 0 0 0 0    0 0 0 0   0 0 0 1   1 1 1 1  1 1 1 1   1 1 1 1   1 1 1 1  1 1 1 1
	//   0          1          1         f        f         f         f        f
	// This binary value corresponds to the coefficient 0x001fffff or 0x1fffff in hexadecimal.

	// Create a new big.Int and set its value to the coefficient.
	target := new(big.Int).SetUint64(uint64(coefficient))
	// This converts the coefficient (e.g., 2097151 for 0x1fffff) into a big integer.

	// Shift the target to the left by (8 * (exponent - 3)) bits.
	// This is equivalent to multiplying the target by 2 raised to the power of (8 * (exponent - 3)).
	// The exponent is 28, so we calculate:
	// 8 * (28 - 3) = 8 * 25 = 200
	// Thus, we are effectively multiplying the target by 2^200.
	target.Lsh(target, 8*(exponent-3))

	// The result is a big integer that represents the mining difficulty target.
	// A higher value for the target indicates a lower difficulty in mining,
	// while a smaller value (like this one, due to the left shift) indicates a high difficulty.

	return target
}

type ErrorCode int

const (
	// Network errors (1000-1999)
	ErrConnectionFailed   ErrorCode = 1000
	ErrPeerDisconnected   ErrorCode = 1001
	ErrMessageEncoding    ErrorCode = 1002
	ErrMessageDecoding    ErrorCode = 1003
	ErrConnectionTimeout  ErrorCode = 1004
	ErrNetworkUnreachable ErrorCode = 1005

	// Database errors (2000-2999)
	ErrDBConnectionFailed ErrorCode = 2000
	ErrDBRead             ErrorCode = 2001
	ErrDBWrite            ErrorCode = 2002
	ErrDBDelete           ErrorCode = 2003
	ErrDBNotFound         ErrorCode = 2004
	ErrDBCorruption       ErrorCode = 2005

	// Protocol errors (3000-3999)
	ErrInvalidMessage       ErrorCode = 3000
	ErrVersionMismatch      ErrorCode = 3001
	ErrInvalidBlock         ErrorCode = 3002
	ErrInvalidTransaction   ErrorCode = 3003
	ErrDuplicateBlock       ErrorCode = 3004
	ErrDuplicateTransaction ErrorCode = 3005

	// Validation errors (4000-4999)
	ErrInvalidSignature  ErrorCode = 4000
	ErrInvalidUTXO       ErrorCode = 4001
	ErrInsufficientFunds ErrorCode = 4002
	ErrInvalidBlockchain ErrorCode = 4003

	// Internal errors (5000-5999)
	ErrInternalFailure   ErrorCode = 5000
	ErrResourceExhausted ErrorCode = 5001
	ErrStateCorruption   ErrorCode = 5002
)

const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	Purple    = "\033[35m"
)

type ErrorCategory int

const (
	CategoryNetwork ErrorCategory = iota
	CategoryDatabase
	CategoryProtocol
	CategoryValidation
	CategoryInternal
)

func (em *ErrorMan) ErrorCategory(code ErrorCode) ErrorCategory {
	switch {
	case code >= 1000 && code < 2000:
		return CategoryNetwork
	case code >= 2000 && code < 3000:
		return CategoryDatabase
	case code >= 3000 && code < 4000:
		return CategoryProtocol
	case code >= 4000 && code < 5000:
		return CategoryValidation
	case code >= 5000 && code < 6000:
		return CategoryInternal
	default:
		return CategoryInternal
	}
}

type ErrorType int

const (
	ErrorNetwork ErrorType = iota
	ErrorDatabase
	ErrorProtocol
	ErrorInternal
	ErrorNeedsRetry
)

type ServerError struct {
	Type    ErrorType
	Message string
	Err     error
}

// RetryContext holds information needed for retry attempts
type RetryContext struct {
	PeerId   int
	Metadata []DBMan.Block
}

type Server struct {
	DBMan        *DBMan.DBman
	host         string
	port         string
	peers        []*CNode
	listener     net.Listener
	done         chan struct{}
	maxPeerLimit int
	ErrorMan     *ErrorMan
	cancel       context.CancelFunc
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

func NewMessage() Message {
	return Message{}
}
func (e *ServerError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
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
	block            bool
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
		block:            false,
	}
}

// Todo Frame messages
// Todo when a tx is requested back from the peer it will send it to to its know peers and one of them is us and
// * thus itll send back what we sent them thats redundunt we wont let it in ofc because we check if its in the mempool already
// Todo [add another check if it has already been mined been mined in a block already]
// Config represents server configuration
type Config struct {
	Host     string
	Port     string
	DBMan    *DBMan.DBman
	ErrorMan *ErrorMan
}

// New initializes a new server with the given configuration
func New(config *Config) *Server {
	return &Server{
		host:     config.Host,
		port:     config.Port,
		DBMan:    config.DBMan,
		ErrorMan: config.ErrorMan,
		done:     make(chan struct{}),
	}
}

var ServerAddr string // Run starts the server and listens for incoming connections
func (s *Server) Run() {
	listener, err := net.Listen("tcp4", s.host+":"+s.port)
	if err != nil {
		s.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "error starting server Err", err)
	}
	s.listener = listener
	defer listener.Close()

	log.Printf("Server is running on %s:%s", s.host, s.port)

	for {
		select {
		case <-s.ErrorMan.ctx.Done():
			s.ShutDown()
		case <-s.done:
			log.Println("Server is shutting down")
			return
		default:
			conn, err := listener.Accept()

			if err != nil {
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "Error accepting connection:", err)
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
		s.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, message, err)
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

func (cn *CNode) SendVerack(height int) error {
	log.Printf("Sending Version Acknowledgement to %s", cn.conn.RemoteAddr())
	Verack := NewMessage()
	Verack.Cammand = byte(VERACK)
	Verack.Length = 0
	encodedBuff := encodeMessage(Verack)
	if _, err := cn.conn.Write(encodedBuff); err != nil {
		message := fmt.Sprintf("error occured sending Verack Message: Err:%v", err)
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, message, err)
		return fmt.Errorf("error occured sending Verack Message: Err:%v", err)
	}
	if err := cn.SendVersionMessage(); err != nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "Error Sending VersionMessage", err)
	}
	curHeight, err := cn.server.DBMan.GetBlockHeight()
	if err != nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "Error getting block height from db", err)
	}
	fmt.Printf("My HEIGHT %d", curHeight)

	if curHeight < height {

		cn.RequestBlocks()
	}

	cn.ExchangedVersion = true

	return nil
}
func (cn *CNode) RequestBlocks() {
	message := NewMessage()

	hash, err := cn.server.DBMan.GetPrevHash()
	if err != nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "Error trying to get Prev Hash from DBMan", err)
	}
	length := dataByteLen([]byte(hash))
	message.Length = length
	message.Cammand = byte(GETBLOCKS)
	message.Data = []byte(hash)
	encodedRequest := encodeMessage(message)
	cn.VSend <- encodedRequest

}
func (cn *CNode) SendVersionMessage() error {
	log.Printf("Sending Version message to %s", cn.conn.RemoteAddr())
	//!New message protocol
	//*[Version | Cammand | Length | Data ]
	if !cn.ExchangedVersion {
		var data = NewVersionMessage()
		var buff bytes.Buffer
		currentBlockHeight, GetHighterr := cn.server.DBMan.GetBlockHeight()
		if GetHighterr != nil {
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "Error getting Block Height From DBMan", GetHighterr)
		}
		data.StartHeight = int32(currentBlockHeight)

		endr := gob.NewEncoder(&buff)
		err := endr.Encode(data)
		if err != nil {
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "error encoding Version Message", err)
		}

		version := NewMessage()
		version.Cammand = byte(VERSION)
		version.Length = dataByteLen(buff.Bytes())
		version.Data = buff.Bytes()
		encodedbuff := encodeMessage(version)
		_, err = cn.conn.Write(encodedbuff)
		if err != nil {
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "error Writing Version Message", err)
			return err
		}

	}
	return nil
}
func (s *Server) BlockTemplate() (DBMan.BlockTemplate, error) {

	prevHash, err := s.DBMan.GetPrevHash()
	if err != nil {
		fmt.Print(err)
	}
	height, GetBlockHeightError := s.DBMan.GetBlockHeight()
	fmt.Printf("Height %d", height)
	if GetBlockHeightError != nil {
		fmt.Print(GetBlockHeightError)
	}
	transactions, err := s.DBMan.GetMempoolTxns()
	fmt.Println(transactions)
	if err != nil {
		fmt.Println(err)
	}

	template := DBMan.BlockTemplate{
		Version:       version,
		Height:        height + 1,
		Transactions:  transactions,
		PrevBlockHash: prevHash,
		MerkleRoot:    "",
		Bits:          Difficulty,
		Timestamp:     time.Now().Unix(),
	}
	return template, nil
}
func HashBlock(header DBMan.BlockHeader) string {
	headerData := fmt.Sprintf("%f%s%s%d%d%d",
		header.Version,
		header.PrevBlockHash,
		header.MerkleRoot,
		header.Timestamp,
		header.Bits,
		header.Nonce,
	)
	// Double SHA-256 hash
	hash := sha256.Sum256([]byte(headerData))
	finalHash := sha256.Sum256(hash[:])

	return hex.EncodeToString(finalHash[:])
}

func (cn *CNode) OnGetDataReqTX(hash string) error {
	fmt.Printf("INCOMMING-HASH:%s\n", (hash))
	txn, err := cn.server.DBMan.GetTxnFromMempool(hash)

	fmt.Printf("Transaction thats being sent %v", txn)
	if err != nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "error occured trying to get txn from DB", err)
	}

	encoded, err := cn.Utils.Encode(txn)

	if err != nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorInternal, "OnGetDataReqTX error encoding transaction", err)

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
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, message, err)
		}

		cn.Addr = cn.conn.RemoteAddr()
		if msg.Cammand == byte(VERSION) {
			var buff VersionMessage

			decoder := gob.NewDecoder(bytes.NewReader(msg.Data))
			decoder.Decode(&buff)

			if !cn.ExchangedVersion {
				if err := cn.SendVerack(int(buff.StartHeight)); err != nil {
					errMessage := fmt.Sprintf("Error sending version ackowledgement message to peer %v", cn.conn.RemoteAddr())
					cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, errMessage, err)
				}

			}
		} else if msg.Cammand == byte(VERACK) {
			fmt.Printf("The Verack \n")
			if !cn.ExchangedVersion {
				cn.Addr = cn.conn.RemoteAddr()
			}
		} else if msg.Cammand == byte(GETBLOCKS) {
			blocks, err := cn.server.DBMan.FromNToLatestHash(string(msg.Data))

			if err != nil {
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "error occured Trying to get Blocks from Db for Transaport", err)
			}
			for _, hash := range blocks {
				fmt.Println(string(hash))
			}
			var buff bytes.Buffer

			errorEncode := gob.NewEncoder(&buff).Encode(blocks)
			if errorEncode != nil {
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "Error Encoding Hashes", err)
			}

			length := dataByteLen(buff.Bytes())
			msg := NewMessage()
			msg.Cammand = byte(HEADERS)
			msg.Data = buff.Bytes()
			msg.Length = length
			encoded := encodeMessage(msg)
			cn.VSend <- encoded
		} else if msg.Cammand == byte(INV) {
			// * check in db if we already have this txn

			exists, err := cn.server.DBMan.IsInMempool(string(msg.Data))

			if err != nil {
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "error occured Trying to get Blocks from Db for Transaport", err)
			}
			HasAlreadybeenMiner, err := cn.server.DBMan.HasTxnAlreadyBeenMined(string(msg.Data))
			errMessage := fmt.Sprintf("error checking if the block has already been miner %v", err)
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, errMessage, err)
			if !exists && !HasAlreadybeenMiner {
				cn.RequestTxn(msg.Data)
			}
		} else if msg.Cammand == byte(GETDATA) {
			go cn.OnGetDataReqTX(string(msg.Data))
		} else if msg.Cammand == byte(TX) {

			// var txn DBMan.Transaction
			// decodedTxn, err := cn.Utils.Decode(msg.Data, &txn)
			// if err != nil {
			// 	cn.server.handleError(NewServerError(ErrorProtocol, "error decoding message from peer", err))
			// }
			//	cn.server.StoreInMempoolAndRelay(decodedTxn)
		} else if msg.Cammand == byte(BLOCK) {
			// Make sure to register the Block type before using gob
			if !cn.block {
				gob.Register(&DBMan.Block{})

				var block DBMan.Block
				var buff bytes.Buffer
				buff.Write(msg.Data)

				err := gob.NewDecoder(&buff).Decode(&block)
				if err != nil {
					cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorInternal, "Error decoding Block", err)
				} else {
					cn.server.ProcessAndStoreNewMinedBlock(block)
				}

			}
		} else if msg.Cammand == byte(HEADERS) {
			// Decode the incoming hashes
			var hashes []string
			decoder := gob.NewDecoder(bytes.NewReader(msg.Data))
			if err := decoder.Decode(&hashes); err != nil {
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "Error decoding hashes", err)
				continue
			}

			var blocksToRequest []string
			for _, hash := range hashes {
				exists, err := cn.server.DBMan.IsBlockAlreadyPresent(hash)
				if err != nil {
					cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "Error checking block existence", err)
					continue
				}
				if !exists {
					blocksToRequest = append(blocksToRequest, hash)
				}
			}

			if len(blocksToRequest) > 0 {
				cn.RequestBlocksForGivenHases(blocksToRequest)
			}
		} else if msg.Cammand == byte(BLOCKS) {
			var hashes []string
			var buff bytes.Buffer
			buff.Write(msg.Data)
			// Create a new decoder from the message data
			decoder := gob.NewDecoder(&buff)
			if err := decoder.Decode(&hashes); err != nil {
				fmt.Printf("failed to decode hashes: %v", err)
			}
			//! Debug loggs
			for _, hash := range hashes {
				fmt.Printf("Received hash: %s\n", hash)
			}

			blocks, err := cn.server.DBMan.GetBlocksForGivenHashes(hashes)
			if err != nil {
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorDatabase, "failed to get blocks: %v", err)
			}

			errorBlocks := cn.PrepareAndSendBlocks(blocks)
			if errorBlocks != nil {
				retryctx := RetryContext{
					PeerId:   cn.Id,
					Metadata: blocks,
				}
				cn.server.ErrorMan.SetBlocksCtxForRetry(retryctx) //this will do the retry if there is an error when this is doing its thing
				// it will log it no no need to catch anything here but just for fun we'll log
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "Error Preparing Blocks for Transacport", errorBlocks)

			}
		} else if msg.Cammand == byte(REQUESTEDBLOCKS) {

			var blocks []DBMan.Block
			var buff bytes.Buffer
			buff.Write(msg.Data)
			err := gob.NewDecoder(&buff).Decode(&blocks)
			if err != nil {
				cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "Error Decoding Recived Blocks", err)
			}
		}

	}
}
func (cn *CNode) PrepareAndSendBlocks(blocks []DBMan.Block) error {
	if blocks == nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "", fmt.Errorf("cannot send nil blocks"))
	}

	var buff bytes.Buffer
	if err := gob.NewEncoder(&buff).Encode(blocks); err != nil {
		return fmt.Errorf("failed to encode blocks: %w", err)
	}

	encodedData := buff.Bytes()
	if len(encodedData) == 0 {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "encoded blocks resulted in empty data", nil)
	}

	blocksResponseMessage := NewMessage()

	blocksResponseMessage.Cammand = byte(REQUESTEDBLOCKS)
	blocksResponseMessage.Length = dataByteLen(encodedData)
	blocksResponseMessage.Data = encodedData

	encodedMessage := encodeMessage(blocksResponseMessage)
	if encodedMessage == nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "", fmt.Errorf("failed to encode message"))
	}

	if cn.VSend == nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "send channel is nil", nil)
	}
	select {
	case cn.VSend <- encodedMessage:
		return nil
	case <-time.After(5 * time.Second):
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "", fmt.Errorf("timeout sending blocks to channel"))
	}
	return nil
}
func (cn *CNode) RequestBlocksForGivenHases(hashes []string) {

	var buff bytes.Buffer

	err := gob.NewEncoder(&buff).Encode(hashes)

	if err != nil {
		cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, "Error Encoding Hashes", err)
	}
	// Create the message
	newBlockRequestMessage := NewMessage()
	newBlockRequestMessage.Cammand = byte(BLOCKS)
	newBlockRequestMessage.Data = buff.Bytes()
	newBlockRequestMessage.Length = dataByteLen(newBlockRequestMessage.Data)

	// Send the encoded message
	encodedMessage := encodeMessage(newBlockRequestMessage)
	cn.VSend <- encodedMessage
}
func (s *Server) IsBlockValid(block *DBMan.BlockHeader) bool {

	target := CompactToBig(uint32(block.Bits))

	blockHash := HashBlock(*block)

	blockHashInt := new(big.Int)
	blockHashInt.SetString(blockHash, 16)
	return blockHashInt.Cmp(target) == -1
}
func (s *Server) ProcessAndStoreNewMinedBlock(block DBMan.Block) (bool, error) {
	if s.IsBlockValid(&block.Header) {
		fmt.Println(true)
		for _, txn := range block.Transactions {
			//Todo check if any of the utxo is absent if so reject
			for _, inputs := range txn.Inputs {
				exist, err := s.DBMan.UTXOExists(inputs.Id)
				if !exist {
					fmt.Println(err)
					return false, fmt.Errorf("rejecting Block Txn With Invalid Utxo Inputs error occured ON UTXO %s", inputs.Id)
				}
			}
		}
		for _, txn := range block.Transactions {
			//Todo check if any of the utxo is absent if so reject
			for _, inputs := range txn.Inputs {
				err := s.DBMan.DeleteUtxo(inputs.Id)
				if err != nil {
					return false, fmt.Errorf("error deleting Inputs %w", err)
				}
			}

			for _, outputSenderUtxo := range txn.Outputs {
				s.DBMan.StoreUTXO(outputSenderUtxo.Id, outputSenderUtxo)
			}
			err := s.DBMan.StoreTransaction(txn.TxnHash, txn)
			if err != nil {
				return false, fmt.Errorf("error Storing txns %w", err)
			}
		}
		err := s.DBMan.StoreBlock(block)

		if err != nil {
			return false, fmt.Errorf("error storing block with hash %s %w", block.Hash, err)
		}

		s.BroadCasteBlock(&block)
		return true, nil
	}
	return false, fmt.Errorf("Error Invalid block")
}

func (s *Server) BroadCasteBlock(block *DBMan.Block) error {
	newBlock := NewMessage()

	var buff bytes.Buffer
	err := gob.NewEncoder(&buff).Encode(block)
	if err != nil {
		s.ErrorMan.ErrorChan <- *(NewServerError(ErrorNetwork, "Error Encoding Block", err))
	}

	length := dataByteLen(buff.Bytes())
	newBlock.Cammand = byte(BLOCK)
	newBlock.Length = length
	newBlock.Data = buff.Bytes()

	encodedBlock := encodeMessage(newBlock)
	for _, peer := range s.peers {
		peer.VSend <- encodedBlock
		peer.block = true
	}
	return nil
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
	if len(s.peers) > 0 {
		for _, peer := range s.peers {
			peer.VSend <- encoded
		}
	}

}

// ! if added call a relay function and send the txn hashes in mempool and they will check in their db do they already have any of the given txn that maches the hashes provided.
// ! or give them all and let em handle theyll just drop it if they already have it i takes one call so i think its faster ill still go with first this one will end up sending alot of redundant data across and the overhead will be insane
func (s *Server) StoreInMempoolAndRelay(ctx context.Context, transaction DBMan.Transaction) (error, bool) {
	log.Println("StoreInMempoolAndRelay: Starting")
	added, err := s.DBMan.VerifyTransaction(ctx, transaction)
	if err != nil {
		log.Printf("StoreInMempoolAndRelay: Error verifying transaction: %v", err)
		return err, false
	}
	log.Printf("StoreInMempoolAndRelay: Transaction verified. Added: %v", added)
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
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, errMessage, fmt.Errorf("channel closed for %s, stopping message sending", cn.conn.RemoteAddr()))
			return
		}

		log.Printf("Writing message of length %d to %s", len(msg), cn.conn.RemoteAddr())
		_, err := writer.Write(msg)
		if err != nil {
			errMessage := fmt.Sprintf("error writing to : %v", cn.conn.RemoteAddr())
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, errMessage, err)
		}

		err = writer.Flush()
		if err != nil {
			errMessage := fmt.Sprintf("Error flushing writer for %s: %v", cn.conn.RemoteAddr(), err)
			cn.server.ErrorMan.ErrorChan <- *NewServerError(ErrorNetwork, errMessage, err)
		}
		log.Printf("Message flushed to %s", cn.conn.RemoteAddr())
	}
}
func (s *Server) ShutDown() {
	if s.cancel != nil {
		fmt.Printf("Closing Down Ctx\n")
		s.cancel()
	}
	if s.listener != nil {
		fmt.Printf("Closing Down Listners\n")
		s.listener.Close()
	}
	if s.DBMan != nil {
		fmt.Printf("Closing Down DBMan\n")
		s.DBMan.Close()
	}

}
func InItServer() (Server, error) {
	log.Println("Starting server application")
	//! Problem we are intialising db too many times in handler its ugly after test fix it
	//Todo
	db, err := DBMan.InitDB("../../testDB")
	if err != nil {
		return Server{}, fmt.Errorf("error occurred initializing db: %w", err)
	}
	//ctx, _ := context.WithCancel(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	ErrMan := &ErrorMan{
		Connman: &Server{},
		ctx:     ctx,
	}
	config := Config{
		Host:     "127.0.0.1",
		Port:     "8082",
		DBMan:    db,
		ErrorMan: ErrMan,
	}

	server := New(&config)
	ErrMan.Connman = server
	server.maxPeerLimit = 20
	server.ErrorMan = ErrMan
	server.cancel = cancel
	go func() {
		server.Run()
	}()
	return *server, nil
}

// Custom key type for context values to avoid string collisions
type contextKey string

const retryContextKey contextKey = "RetryCtx"

type ErrorMan struct {
	ctx               context.Context
	RetryRequestQueue chan context.Context
	RetrySendQueue    chan context.Context
	ErrorChan         chan ServerError
	Connman           *Server
}

func NewServerError(errType ErrorType, message string, err error) *ServerError {
	return &ServerError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}
func NewErrorMan(ctx context.Context, connMan *Server) *ErrorMan {
	if ctx == nil {
		ctx = context.Background()
	}
	return &ErrorMan{
		ctx:               ctx,
		RetryRequestQueue: make(chan context.Context, 100),
		RetrySendQueue:    make(chan context.Context, 100),
		Connman:           connMan,
		ErrorChan:         make(chan ServerError, 1000),
	}
}
func (em *ErrorMan) Log(err error) {
	serverErr, ok := err.(*ServerError)
	if !ok {
		fmt.Println("Error is not of type *ServerError:", err)
		return
	}
	var prefix string
	switch serverErr.Type {
	case ErrorNetwork:
		prefix = "Network Error"
		fmt.Printf("%s%s%s : %s%v%s\n", Magenta, prefix, Reset, Green, serverErr, Reset)
	case ErrorDatabase:
		prefix = "Datebase Error"
		fmt.Printf("%s%s%s : %s%v%s\n", Green, prefix, Reset, Blue, serverErr, Reset)
	case ErrorProtocol:
		prefix = "Protocol Error"
		fmt.Printf("%s%s%s : %s%v%s\n", Cyan, prefix, Reset, Yellow, serverErr, Reset)
	case ErrorInternal:
		prefix = "Internal Error"
		fmt.Printf("%s%s%s : %s%v%s\n", Red, prefix, Reset, Magenta, serverErr, Reset)
	case ErrorNeedsRetry:
		prefix = "Needs Retry"
		fmt.Printf("%s%s%s : %s%v%s\n", Yellow, prefix, Reset, Blue, serverErr, Reset)
	default:
		prefix = "Unknown Error Type"
		fmt.Printf("%s%s%s : %s%v%s\n", Red, prefix, Reset, Yellow, serverErr, Reset)
	}

}
func (rc RetryContext) String() string {
	// Convert to JSON for readable output
	data, err := json.Marshal(rc)
	if err != nil {
		return fmt.Sprintf("PeerId: %d, Metadata: %v", rc.PeerId, rc.Metadata)
	}
	return string(data)
}

func (em *ErrorMan) MaybeRetryGetBlocksFromPeers(ctx context.Context) {
	for {
		ctx, ok := <-em.RetryRequestQueue
		if !ok {
			em.ErrorChan <- *NewServerError(ErrorInternal, "Error Occurred while reading chan", nil)
			return
		}

		if retryCtx, ok := ctx.Value(retryContextKey).(RetryContext); ok {
			fmt.Printf("%sRetry Context:%s %s%v%s\n",
				Yellow,
				Reset,
				Green,
				retryCtx,
				Reset)
		} else {
			fmt.Printf("No retry context found in context\n")
		}
	}
}

func (em ErrorMan) RetrySendingBlockToPeer(ctx context.Context) {
	for {
		ctx, ok := <-em.RetrySendQueue
		if !ok {
			em.ErrorChan <- *NewServerError(ErrorInternal, "Error Occurred while reading chan", nil)
			return
		}

		if retryCtx, ok := ctx.Value(retryContextKey).(RetryContext); ok {
			for _, peer := range em.Connman.peers {
				if peer.Id == retryCtx.PeerId {
					peer.PrepareAndSendBlocks(retryCtx.Metadata)
				}
			}
			// fmt.Printf("%sRetry Send Context:%s %s%v%s\n",
			// 	Cyan,
			// 	Reset,
			// 	Magenta,
			// 	retryCtx,
			// 	Reset)

		} else {
			fmt.Printf("No retry context found in context\n")
		}
	}
}

func (em *ErrorMan) SetGetBlockErrorRetryCtx(ctx context.Context, retCtx RetryContext) {
	newCtx := context.WithValue(ctx, retryContextKey, retCtx)
	em.RetryRequestQueue <- newCtx
}
func (em *ErrorMan) SetBlocksCtxForRetry(retCtx RetryContext) {
	newCtx := context.WithValue(em.ctx, retryContextKey, retCtx)
	em.RetryRequestQueue <- newCtx
}

func (em ErrorMan) HandleErrors() {
	for {
		err := <-em.ErrorChan
		em.Log(&err)
	}
}
