// package types

// import (
// 	"crypto/ecdsa"
// 	"time"
// )

// type GetBalance struct {
// 	Address string `json:"address"`
// }
// type GetBalanceResponse struct {
// 	Balance float64 `json:"balance"`
// }
// type CoinbaseTransaction struct {
// 	ID        string  // Unique identifier for the coinbase transaction
// 	Amount    float64 // Amount awarded to the miner
// 	Miner     string  // Address of the miner (newly generated)
// 	Signature string  // Signature for the coinbase transaction
// }

// // BlockHeader represents the structure of a simplified block header

// type BlockHeader struct {
// 	Version       int
// 	PrevBlockHash string
// 	MerkleRoot    string
// 	Timestamp     time.Time
// 	Bits          int   // Difficulty target
// 	Nonce         int64 // The value we are trying to find
// }

// // Transaction represents a Bitcoin transaction
// type Transaction struct {
// 	hash     string
// 	Version  int32
// 	Inputs   []TxInput
// 	Outputs  []TxOutput
// 	LockTime uint32
// }

// // TxInput represents an input of a Bitcoin transaction
// type TxInput struct {
// 	PreviousOutput OutPoint
// 	publickey      ecdsa.PublicKey
// }

// // TxOutput represents an output of a Bitcoin transaction
// type TxOutput struct {
// 	Value     int64
// 	publickey ecdsa.PublicKey
// }

// // OutPoint references a specific output in a previous transaction
// type OutPoint struct {
// 	Hash  []byte
// 	Index uint32
// }
// type Block struct {
// 	Header       BlockHeader         // The block header
// 	Coinbase     CoinbaseTransaction // The coinbase transaction
// 	Hash         string              // Hash of the block
// 	Transactions []Transaction       // List of other transactions in the block
// }
