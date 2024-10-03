package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/RahulMishra0722/GC/DBMan"
	"go.uber.org/zap/buffer"
)

type Miner struct {
	DBman *DBMan.DBman
}

const (
	Difficulty = 10
	Subsidy    = 50 // will get halved every 250000 blocks
	CurVersion = 0.1
)

type MinerKeys struct {
	PublicKey  ecdsa.PublicKey
	Privatekey ecdsa.PrivateKey
}

// CoinbaseTransaction represents the special coinbase transaction
type CoinbaseTransaction struct {
	Amount    float64   // Amount awarded to the miner
	Miner     MinerKeys // Address of the miner (newly generated)
	BlockHash string
	Outputs   []TxOutput
}

// BlockHeader represents the structure of a simplified block header

type BlockHeader struct {
	Version       float64
	PrevBlockHash string
	MerkleRoot    string
	Timestamp     time.Time
	Bits          int   // Difficulty target
	Nonce         int64 // The value we are trying to find
}
type ProofOfWork struct {
	block  *Block
	Target *big.Int
}

// Transaction represents a Bitcoin transaction
type Transaction struct {
	Hash      string
	Sender    ecdsa.PublicKey
	Reciever  ecdsa.PublicKey
	Signature string
	Amount    float64
	Inputs    []TxInput
	Outputs   []TxOutput
	LockTime  uint32
}

// TxInput represents an input of a Bitcoin transaction
type TxInput struct {
	PreviousOutput OutPoint
	Publickey      ecdsa.PublicKey
}

// TxOutput represents an output of a Bitcoin transaction
type TxOutput struct {
	TxnId     string          // curr txn thats creating it
	Value     int64           // how much does this coin worth
	PublicKey ecdsa.PublicKey // who cam spend this
}

// OutPoint references a specific output in a previous transaction
type OutPoint struct {
	Hash  string // Identifies which transaction created the UTXO
	Index uint32 // Refers to where the UTXO lies in the list of outputs in that transaction
}

type Block struct {
	Header       BlockHeader         // The block header
	Coinbase     CoinbaseTransaction // The coinbase transaction
	Hash         string              // Hash of the block
	Transactions []string            // List of other transactions in the block
}

func int64ToBytes(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func float64ToBytes(num float64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func doubleSHA256(data []byte) string {
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])
	return hex.EncodeToString(hash2[:])
}

// func (m *Miner) NewUtxo(hash string) TxOutput {

// }
func (m *Miner) CreateCoinbaseTransaction() CoinbaseTransaction {
	privateKey, publicKey, err := GenerateNewKey()
	if err != nil {
		fmt.Print("Error Generating Keys CoinBase %s", err)
	}
	minerKeys := &MinerKeys{
		PublicKey:  *publicKey,
		Privatekey: *privateKey,
	}
	outPut := TxOutput{
		TxnId:     "",
		Value:     Subsidy,
		PublicKey: *publicKey,
	}
	coinBaseTxn := CoinbaseTransaction{
		Amount: 50,
		Miner:  *minerKeys,
		Outputs: []TxOutput{
			outPut,
		},
	}

	return coinBaseTxn
}
func calculateMerkleRoot(Transactions [][]byte) string {

	// Initial Hashing
	Hashes := make([]string, len(Transactions))
	for i, txn := range Transactions {
		Hashes[i] = doubleSHA256(txn)
	}

	for len(Hashes) > 1 {
		if len(Hashes)%2 != 0 {
			Hashes = append(Hashes, Hashes[len(Hashes)-1])
		}
		newHash := make([]string, 0, len(Hashes)/2)
		for i := 0; i < len(Hashes); i += 2 {
			combined := Hashes[i] + Hashes[i+1]

			txnHash := doubleSHA256([]byte(combined))
			newHash = append(newHash, txnHash)
		}
		Hashes = newHash
	}
	return Hashes[0]
}
func HashBlock(header BlockHeader, coinbase CoinbaseTransaction) string {
	headerData := fmt.Sprintf("%f%s%s%d%d%d%f",
		header.Version,
		header.PrevBlockHash,
		header.MerkleRoot,
		header.Timestamp.Unix(),
		header.Bits,
		header.Nonce,
		coinbase.Amount,
	)
	// Double SHA-256 hash
	hash := sha256.Sum256([]byte(headerData))
	finalHash := sha256.Sum256(hash[:])

	return hex.EncodeToString(finalHash[:])
}

// CalculateMerkleRoot calculates the Merkle root of the transactions
func CalculateMerkleRoot(transactions []Transaction) string {
	if len(transactions) == 0 {
		return ""
	}

	var hashes []string
	for _, tx := range transactions {
		hashes = append(hashes, tx.Hash) // Using transaction ID as hash for simplicity
	}

	// Construct the Merkle tree
	for len(hashes) > 1 {
		var newHashes []string
		for i := 0; i < len(hashes); i += 2 {
			if i+1 == len(hashes) {
				newHashes = append(newHashes, hashes[i]) // Odd number of hashes, carry forward
			} else {
				combinedHash := hashes[i] + hashes[i+1] // Combine two hashes
				hash := sha256.Sum256([]byte(combinedHash))
				newHashes = append(newHashes, hex.EncodeToString(hash[:]))
			}
		}
		hashes = newHashes // Replace with new level of hashes
	}

	return hashes[0] // The last remaining hash is the Merkle root
}

// GenerateNewKey generates a new ECDSA key pair
func GenerateNewKey() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pubKey := &privKey.PublicKey
	return privKey, pubKey, nil
}

// SignTransaction signs a coinbase transaction with the provided private key
func SignTransaction(privKey *ecdsa.PrivateKey, txnhash string, transaction Transaction, amount float64) (string, error) {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%f", txnhash, transaction.Hash, amount)))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		return "", err
	}
	signature := append(r.Bytes(), s.Bytes()...)
	return hex.EncodeToString(signature), nil
}

// ValidateSignature verifies the signature of the coinbase transaction
func ValidateSignature(pubKey *ecdsa.PublicKey, txnHash string, transaction DBMan.Transaction, amount float64, signature string) bool {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%f", txnHash, transaction.Hash, amount)))

	// Decode the signature
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	if len(sigBytes) != 64 {
		return false // Invalid signature length
	}

	// Extract r and s from the signature
	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	// Verify the signature
	return ecdsa.Verify(pubKey, hash[:], r, s)
}

func (m *Miner) ConvertTransactionsToByteArray(transactions []DBMan.Transaction) [][]byte {

	var transactionInBytes [][]byte
	for _, txn := range transactions {
		var buff buffer.Buffer
		encoder := gob.NewEncoder(&buff)
		encoder.Encode(txn)
		transactionInBytes = append(transactionInBytes, buff.Bytes())
	}
	return transactionInBytes
}
func (m *Miner) NewBlock(merkelRoot string, Transactions []string) *Block {
	fmt.Println("Mining starts")

	cbaseTxn := m.CreateCoinbaseTransaction()
	Prevhash, err := m.DBman.GetPrevHash()
	if err != nil {
		fmt.Println(err)
	}

	blockHeader := BlockHeader{
		Version:       CurVersion,
		PrevBlockHash: Prevhash,
		MerkleRoot:    merkelRoot,
		Timestamp:     time.Now(),
		Bits:          Difficulty,
		Nonce:         0,
	}

	block := &Block{
		Coinbase:     cbaseTxn,
		Header:       blockHeader,
		Transactions: Transactions,
		Hash:         "",
	}

	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()

	block.Header.Nonce = nonce

	block.Hash = hex.EncodeToString(hash[:])
	cbaseTxn.BlockHash = block.Hash
	fmt.Print(cbaseTxn.BlockHash)

	if !pow.Validate() {
		fmt.Println("Invalid block: Proof of Work validation failed")
		os.Exit(1)
	}

	fmt.Printf("Block mined with nonce: %d\n", nonce)
	return block
}

func NewProofOfWork(block *Block) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-block.Header.Bits))
	return &ProofOfWork{block: block, Target: target}
}
func (pow *ProofOfWork) prepareData(nonce int64) []byte {
	data := bytes.Join(
		[][]byte{
			float64ToBytes(pow.block.Header.Version),
			[]byte(pow.block.Header.PrevBlockHash),
			[]byte(pow.block.Header.MerkleRoot),
			int64ToBytes(pow.block.Header.Timestamp.Unix()),
			int64ToBytes(int64(pow.block.Header.Bits)),
			int64ToBytes(nonce),
		},
		[]byte{},
	)
	return data
}

func (pow *ProofOfWork) Run() (int64, []byte) {
	var hashInt big.Int
	var hash [32]byte
	nonce := int64(0)
	maxNonce := int64(math.MaxInt64)

	fmt.Println("Mining a new block")
	for nonce < maxNonce {
		data := pow.prepareData(nonce)
		hash = sha256.Sum256(data)
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.Target) == -1 {
			fmt.Printf("\rNonce found: %d", nonce)
			fmt.Println()
			return nonce, hash[:]
		} else {
			nonce++
		}
	}
	fmt.Println("Failed to find nonce, consider adjusting difficulty")
	return nonce, hash[:]
}

func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int
	data := pow.prepareData(pow.block.Header.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.Target) == -1
	return isValid
}

func onTransactionEmpty() string {
	return strings.Repeat("0", 64)
}

func (m *Miner) MineBlock() {
	// maybe we'll call all the nodes and ask for txns and also get it from mempool if there arent any meaning we'd have to mine an empty block which is fine in newwer blockchains
	// we gotta distribute they keys somehow
	db := DBMan.GetDB()
	transactions, err := db.GetMempoolTxns()
	if err != nil {
		// maybe here we will need to make empty blocks
		fmt.Print(err)
	}

	var txnHashes []string
	for _, txn := range transactions {

		txnHashes = append(txnHashes, txn.Hash)
	}
	for _, txn := range transactions {

		go ValidateSignature(&txn.Sender, txn.Hash, txn, txn.Amount, txn.Signature)
	}
	//before this for each transaction get
	if len(transactions) == 0 {
		root := onTransactionEmpty()
		block := m.NewBlock(root, txnHashes)
		fmt.Printf("Empty block mined: %+v\n", block)
		return
	}

	tran := m.ConvertTransactionsToByteArray(transactions)

	root := calculateMerkleRoot(tran)
	block := m.NewBlock(root, txnHashes)
	fmt.Print(block)
}

func main() {
	db := DBMan.GetDB()
	mine := Miner{
		DBman: db,
	}
	mine.MineBlock()
}
