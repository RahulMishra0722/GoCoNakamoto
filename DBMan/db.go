package DBMan

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"

	"go.etcd.io/bbolt"
)

type DBman struct {
	db   *bbolt.DB
	rwMu sync.RWMutex
}

var (
	instance *DBman
	once     sync.Once
	dbMutex  sync.Mutex
)

// itob converts an int to a byte slice.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// btoi converts a byte slice to an int.
func btoi(b []byte) int {
	return int(binary.BigEndian.Uint64(b))
}

type BlockHeader struct {
	Version       float64
	PrevBlockHash string
	MerkleRoot    string
	Timestamp     int64
	Bits          int  // Difficulty target
	Nonce         uint // The value we are trying to find
}

type Transaction struct {
	BlockHash  string //Block Hash what block this transaction was formed in
	TxnHash    string // Transaction hash
	Sender     string // Sender's public key (not used in coinbase)
	Receiver   string // Coinbase transactions for rewarding miner
	Signature  string // Signature of the transaction
	Amount     int64  // Total amount, in satoshis (1 GXC = 100,000,000 satoshis) — never floats
	Inputs     []Utxo // Inputs for the transaction
	Outputs    []Utxo // Outputs for the transaction
	LockTime   uint32 // Lock time for the transaction
	IsCoinbase bool   // Indicates if the transaction is a coinbase transaction
	Height     uint32 // Block height for the coinbase transaction
	Subsidy    int64  // Mining subsidy, in satoshis
	Fee        int64
}

// SatoshisPerCoin: the smallest unit is the satoshi; 1 GXC = 100,000,000 satoshis.
// All monetary amounts anywhere in the codebase are satoshis (int64).
const SatoshisPerCoin int64 = 100_000_000

type BlockTemplate struct {
	PrevBlockHash string        `json:"previousBlockHash"`
	Bits          int           `json:"bits"`
	Height        int           `json:"height"`
	MerkleRoot    string        `json:"merkleRoot"`
	Timestamp     int64         `json:"timeStamp"`
	Transactions  []Transaction `json:"transactions"`
	Version       float64       `json:"version"`
}

// TxInput represents an input of a Bitcoin transaction
type OutPoint struct {
	Hash  string // Identifies which transaction created the UTXO
	Index uint32 // Refers to where the UTXO lies in the list of outputs in that transaction
}

type TxInput struct {
	Value          int64
	PreviousOutput OutPoint
	Publickey      ecdsa.PublicKey
}
type Utxo struct {
	Id             string
	PreviousOutput OutPoint
	Output         TxOutput
}

type TxOutput struct {
	Value        int64  // Amount in satoshis (1 GXC = 100,000,000 satoshis). Integers only — floats lose precision and make equality/arithmetic unreliable.
	ScriptPubKey string // Locking script, setting conditions to spend this output (e.g., requires a signature)
}
type Blocks []Block
type Block struct {
	Height       int
	Header       BlockHeader   // The block header
	Hash         string        // Hash of the block
	Transactions []Transaction // List of other transactions in the block
	Difficulty   int
}

func (b *Blocks) Serialize() []byte {
	var buff bytes.Buffer
	gob.NewEncoder(&buff).Encode(b)
	return buff.Bytes()
}
func Deserialize(data []byte) (Blocks, error) {
	var blocks Blocks
	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&blocks)
	return blocks, err
}
func (b *Block) Serialize() []byte {
	var buff bytes.Buffer
	gob.NewEncoder(&buff).Encode(b)
	return buff.Bytes()
}

func SerializeUnserializableData(input interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(input)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
func DeSeralizeUnserialisableData(input []byte, output interface{}) error {
	buf := bytes.NewBuffer(input)
	decoder := gob.NewDecoder(buf)
	err := decoder.Decode(output)
	if err != nil {
		return err
	}
	return nil
}

// Todo Come back to this one filet is missing

func DeserializeTxn(data []byte) ([]Transaction, error) {
	var retrievedTransactions []Transaction
	err := json.Unmarshal(data, &retrievedTransactions)
	return retrievedTransactions, err
}
func DeserializeUTXO(data []byte) (Utxo, error) {
	var retrievedUXTO Utxo
	err := json.Unmarshal(data, &retrievedUXTO)
	return retrievedUXTO, err
}
func DeserializeMempool(data []byte) (Transaction, error) {
	var req Transaction
	err := json.Unmarshal(data, &req)
	if err != nil {
		return Transaction{}, fmt.Errorf("failed to unmarshal transaction request: %w", err)
	}
	return req, nil
}
func SerializeUtxo(utxo Utxo) ([]byte, error) {
	return json.Marshal(utxo)
}
func GetDB(path string) (*DBman, error) {
	db, openErr := bbolt.Open(path, 0666, nil)
	if openErr != nil {
		err := fmt.Errorf("failed to open database: %w", openErr)
		return &DBman{}, err
	}
	instance = &DBman{
		db: db,
	}

	return instance, nil
}

func (dm *DBman) GenerateGenesisBlock() (*Block, Transaction, error) {

	genesisMinerPubkey := "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048"

	coinbaseTx := Transaction{
		BlockHash: "",
		TxnHash:   "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048",
		Sender:    "",
		Receiver:  genesisMinerPubkey,
		Signature: "",
		Amount:    50 * SatoshisPerCoin,
		Inputs:    []Utxo{},
		Outputs: []Utxo{
			{
				Id: "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048",
				PreviousOutput: OutPoint{
					Hash:  "",
					Index: 0,
				},
				Output: TxOutput{
					Value:        50 * SatoshisPerCoin,
					ScriptPubKey: "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048",
				},
			},
		},
		LockTime:   0,
		IsCoinbase: true,
		Height:     1,
		Subsidy:    50 * SatoshisPerCoin,
		Fee:        0,
	}
	header := BlockHeader{
		Version:       1.0,
		PrevBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
		MerkleRoot:    "",
		Timestamp:     0,
		Bits:          0x1d00ffff,
		Nonce:         0,
	}

	// ORDERING RULE (same as the miner fix): the coinbase's TxnHash must
	// be computed BEFORE the merkle root, which must be computed BEFORE
	// the block hash. Previously the root was taken over a transaction
	// whose TxnHash was still "" and the txn hash was assigned afterwards.
	// NOTE: this changes the genesis hash — existing local DBs must be
	// deleted (MainServer.db) or the new genesis will be rejected as a
	// non-extending block.
	txHash := CalculateTransactionHash(coinbaseTx)
	coinbaseTx.TxnHash = txHash

	block := &Block{
		Height:       1,
		Header:       header,
		Hash:         "",
		Transactions: []Transaction{coinbaseTx},
	}

	merkleRoot := CalculateMerkleRoot(block.Transactions)
	block.Header.MerkleRoot = merkleRoot

	blockHash := CalculateBlockHash(block.Header)
	fmt.Printf("Genesis Block hash %s\n", blockHash)
	block.Hash = blockHash

	block.Transactions[0].BlockHash = blockHash

	return block, coinbaseTx, nil
}

func CalculateMerkleRoot(transactions []Transaction) string {
	if len(transactions) == 0 {
		return ""
	}

	hashes := make([][]byte, len(transactions))
	for i, tx := range transactions {
		hash := sha256.Sum256([]byte(tx.TxnHash))
		hashes[i] = hash[:]
	}

	for len(hashes) > 1 {
		if len(hashes)%2 != 0 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		newLevel := make([][]byte, 0, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			concatenated := append(hashes[i], hashes[i+1]...)
			hash := sha256.Sum256(concatenated)
			newLevel = append(newLevel, hash[:])
		}

		hashes = newLevel
	}

	return hex.EncodeToString(hashes[0])
}
// PrivKeyToHex renders a private key scalar as a fixed-width hex string.
func PrivKeyToHex(privKey *ecdsa.PrivateKey) string {
	return fmt.Sprintf("%064x", privKey.D)
}

// PubKeyToHex encodes raw public key bytes as hex.
func PubKeyToHex(pubKeyBytes []byte) string {
	return hex.EncodeToString(pubKeyBytes)
}

// CalculateBlockHash is the CANONICAL block hash: double SHA-256 over the
// serialized header, matching what ProofOfWork and block validation use.
// All hashing lives here in DBMan — the package that owns the types being
// hashed — so miner and validator can never drift.
func CalculateBlockHash(header BlockHeader) string {
	headerBytes := []byte(fmt.Sprintf("%f%s%s%d%d%d",
		header.Version,
		header.PrevBlockHash,
		header.MerkleRoot,
		header.Timestamp,
		header.Bits,
		header.Nonce,
	))

	hash := sha256.Sum256(headerBytes)
	finalHash := sha256.Sum256(hash[:])
	return hex.EncodeToString(finalHash[:])
}

// CompactToBig converts a compact difficulty representation (e.g.
// 0x1e00ffff) into the full big.Int target. Difficulty is consensus data,
// so it lives here next to the hash functions that use it.
func CompactToBig(compact uint32) *big.Int {
	exponent := uint(compact >> 24)
	coefficient := compact & 0x007fffff
	target := new(big.Int).SetUint64(uint64(coefficient))
	target.Lsh(target, 8*(exponent-3))
	return target
}

func CalculateTransactionHash(tx Transaction) string {
	// Amount/Subsidy are int64 satoshis and hash as %d. NOTE: this changes
	// the txn hash format — the wallet repo must use the same format when
	// signing (it signs over this hash).
	txBytes := []byte(fmt.Sprintf("%s%s%s%d%d%d%d%d",
		tx.Sender,
		tx.Receiver,
		tx.Signature,
		tx.Amount,
		tx.LockTime,
		tx.Height,
		tx.Subsidy,
		tx.Fee,
	))

	hash := sha256.Sum256(txBytes)
	return hex.EncodeToString(hash[:])
}

func InitDB(path string) (*DBman, error) {
	db, err := GetDB(path)
	if err != nil {
		fmt.Printf("error occured Initialising the DBMan")
	}
	err = db.db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{"BlockHeight", "Transactions", "prev_hash", "MemPoolTransactions", "UTXO", "Blocks"}
		for _, bucketName := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
			if err != nil {
				return fmt.Errorf("error creating %s bucket: %w", bucketName, err)
			}
			fmt.Printf("%s bucket created or already exists\n", bucketName)
		}

		return nil
	})
	if err != nil {
		return &DBman{}, fmt.Errorf("error occured initialising Buckets %w", err)
	}

	block, coinbaseTx, err := db.GenerateGenesisBlock()
	if err != nil {
		return &DBman{}, fmt.Errorf("error generating Genesis Block : %w", err)
	}
	StoreBlockerr := db.StoreBlock(*block)
	if StoreBlockerr != nil {
		return &DBman{}, fmt.Errorf("error Storing block %w", StoreBlockerr)
	}
	Txnerr := db.StoreTransaction(coinbaseTx.TxnHash, coinbaseTx)
	if Txnerr != nil {
		return &DBman{}, fmt.Errorf("error Storing Transaction %w", Txnerr)
	}
	for _, txn := range block.Transactions {
		for _, output := range txn.Outputs {
			db.StoreUTXO(output.Id, output)
		}
	}

	return db, nil
}
func (b *Block) BytesLen() uint32 {
	var buff bytes.Buffer
	gob.NewEncoder(&buff).Encode(b)
	return uint32(buff.Len())
}

// Transaction.CalculateHash was deleted: it hashed its own TxnHash and
// BlockHash fields (a hash that covers the field it produces). The one
// caller now uses CalculateTransactionHash.

func (dm *DBman) DeleteMempoolTransaction(hash string) (bool, error) {
	var deleted bool
	err := dm.db.Update(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket([]byte("MemPoolTransactions"))

		cursor := bucket.Cursor()

		for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
			if bytes.Equal(k, []byte(hash)) {
				bucket.Delete(k)
				break
			}

		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("error deleting transaction from the mempool")
	}
	fmt.Print("\n", "Cleaning up Successfull Transactions from the Mempool currently cleaning %s", hash, "\n")
	return deleted, nil
}
func (dm *DBman) IsInMempool(hash string) (bool, error) {
	log.Printf("IsInMempool: Checking if transaction with hash %s is in the mempool", hash)
	var exists bool

	err := dm.db.View(func(tx *bbolt.Tx) error {
		log.Println("IsInMempool: Starting bbolt transaction for mempool check")
		bucket := tx.Bucket([]byte("MemPoolTransactions"))
		if bucket == nil {
			return nil
		}

		key := []byte(hash)
		if bucket.Get(key) == nil {
			log.Printf("IsInMempool: Transaction %s not found in the mempool", hash)
			return nil
		}

		log.Printf("IsInMempool: Transaction %s found in the mempool", hash)
		exists = true
		return nil
	})

	if err != nil {
		log.Printf("IsInMempool: Error occurred - %v", err)
		return false, err
	}

	log.Printf("IsInMempool: Transaction %s exists: %v", hash, exists)
	return exists, nil
}

func (dm *DBman) VerifyTransaction(ctx context.Context, transaction Transaction) (bool, error) {
	log.Printf("VerifyTransaction: Starting verification for transaction %s", transaction.TxnHash)
	var successfullyAdded bool

	err := dm.db.View(func(tx *bbolt.Tx) error {
		select {
		case <-ctx.Done():
			log.Println("VerifyTransaction: Context canceled")
			return ctx.Err()
		default:
			log.Println("VerifyTransaction: Starting bbolt transaction for UTXO check")
			bucketName := []byte("UTXO")
			bucket := tx.Bucket(bucketName)
			if bucket == nil {
				log.Println("VerifyTransaction: UTXO bucket does not exist")
				return fmt.Errorf("UTXO bucket not initialized")
			}

			var total int64
			for _, utxo := range transaction.Inputs {
				select {
				case <-ctx.Done():
					log.Println("VerifyTransaction: Context canceled during UTXO check")
					return ctx.Err()
				default:
					data := bucket.Get([]byte(utxo.Id))
					if data == nil {
						log.Printf("VerifyTransaction: Invalid input found for UTXO %s", utxo.Id)
						return fmt.Errorf("invalid Inputs Input-Id: %s", string(utxo.Id))
					}
					// Use the STORED UTXO's value, not the value the sender
					// claims in the transaction. Otherwise a forged wallet
					// could inflate its balance by claiming inputs hold
					// more than the chain says they do.
					stored, err := DeserializeUTXO(data)
					if err != nil {
						return fmt.Errorf("failed to deserialize stored UTXO %s: %w", utxo.Id, err)
					}
					total += stored.Output.Value
				}
			}

			log.Printf("VerifyTransaction: Total UTXO value: %d, Transaction amount: %d", total, transaction.Amount)
			if total < transaction.Amount {

				log.Printf("VerifyTransaction: Insufficient balance for transaction %s", transaction.TxnHash)
				return fmt.Errorf("insufficient balance, transaction rejected")
			}

			select {
			case <-ctx.Done():
				log.Println("VerifyTransaction: Context canceled before mempool check")
				return ctx.Err()
			default:
				log.Println("VerifyTransaction: Checking if transaction is in mempool")
				exist, err := dm.IsInMempool(transaction.TxnHash)
				if err != nil {
					log.Printf("VerifyTransaction: Error checking mempool for transaction %s - %v", transaction.TxnHash, err)
					return fmt.Errorf("error occurred during mempool check: %w", err)
				}

				if !exist {
					log.Printf("VerifyTransaction: Adding transaction %s to mempool", transaction.TxnHash)
					err := dm.AddToMempool(transaction.TxnHash, transaction)
					if err != nil {
						log.Printf("VerifyTransaction: Error adding transaction %s to mempool - %v", transaction.TxnHash, err)
						return fmt.Errorf("error adding transaction to mempool: %w", err)
					}
					log.Printf("VerifyTransaction: Transaction %s successfully added to mempool", transaction.TxnHash)
					successfullyAdded = true
				} else {
					log.Printf("VerifyTransaction: Transaction %s already exists in the mempool", transaction.TxnHash)
					successfullyAdded = false
				}
			}

			return nil
		}
	})

	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			log.Printf("VerifyTransaction: Context canceled or deadline exceeded for transaction %s", transaction.TxnHash)
			return false, err
		}
		log.Printf("VerifyTransaction: Error occurred during transaction verification - %v", err)
		return false, fmt.Errorf("transaction verification failed: %w", err)
	}

	log.Printf("VerifyTransaction: Transaction %s verified successfully", transaction.TxnHash)
	return successfullyAdded, nil
}

// AddToMempool atomically validates and inserts a transaction:
//  1. every input must exist in the UTXO set
//  2. no input may already be spent by another transaction in the mempool
//  3. insert
//
// All three steps run inside ONE bbolt write transaction so two concurrent
// transactions spending the same UTXO cannot both pass the check — the write
// lock serializes them, and the second one sees the first's insert.
func (dm *DBman) AddToMempool(hash string, txn Transaction) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()

	return dm.db.Update(func(tx *bbolt.Tx) error {
		utxoBucket := tx.Bucket([]byte("UTXO"))
		if utxoBucket == nil {
			return fmt.Errorf("bucket UTXO does not exist")
		}
		mempool := tx.Bucket([]byte("MemPoolTransactions"))
		if mempool == nil {
			return fmt.Errorf("bucket MemPoolTransactions does not exist")
		}

		// 1. inputs must exist in the UTXO set
		inputSet := make(map[string]bool, len(txn.Inputs))
		for _, utxo := range txn.Inputs {
			if utxoBucket.Get([]byte(utxo.Id)) == nil {
				return fmt.Errorf("invalid input: UTXO %s does not exist", utxo.Id)
			}
			inputSet[utxo.Id] = true
		}

		// 2. inputs must not already be spent by a mempool transaction
		//    (double spend). UTXOs are only deleted when a block is mined,
		//    so the UTXO set alone can't detect this.
		cursor := mempool.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var pending Transaction
			if err := json.Unmarshal(v, &pending); err != nil {
				return fmt.Errorf("failed to deserialize mempool txn %s: %w", string(k), err)
			}
			for _, spent := range pending.Inputs {
				if inputSet[spent.Id] {
					return fmt.Errorf("double spend rejected: UTXO %s already spent by mempool txn %s", spent.Id, pending.TxnHash)
				}
			}
		}

		// 3. insert
		serialized, err := json.Marshal(txn)
		if err != nil {
			return fmt.Errorf("serialize transaction: %w", err)
		}
		return mempool.Put([]byte(hash), serialized)
	})
}
func (dm *DBman) GetMempoolTxns() ([]Transaction, error) {

	fmt.Println("GetMempoolTxns: Starting function")
	var transactions []Transaction

	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("MemPoolTransactions"))
		if bucket == nil {
			fmt.Println("GetMempoolTxns: No transactions bucket found")
			return nil
		}

		cursor := bucket.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {

			var txn Transaction
			err := json.Unmarshal(v, &txn)
			if err != nil {
				fmt.Printf("GetMempoolTxns: Failed to deserialize transaction: %v\n", err)
				return fmt.Errorf("failed to deserialize transaction: %w", err)
			}

			transactions = append(transactions, txn)
			fmt.Printf("GetMempoolTxns: Transaction added, total count: %d\n", len(transactions))
		}

		//fmt.Printf("GetMempoolTxns: Finished processing. Total transactions: %v\n", transactions)
		return nil
	})

	if err != nil {
		fmt.Printf("GetMempoolTxns: Error occurred: %v\n", err)
		return nil, err
	}

	//fmt.Printf("GetMempoolTxns: Finished processing. Total transactions: %v\n", transactions)
	return transactions, nil
}
func (dm *DBman) GetBlocksForGivenHashes(hashes []string) ([]Block, error) {
	var requestedBlocks []Block
	err := dm.db.View(func(tx *bbolt.Tx) error {
		blocks := tx.Bucket([]byte("Blocks"))
		if blocks == nil {
			return fmt.Errorf("bucket %s doesn't exist", "Blocks")
		}

		for i, hash := range hashes {
			data := blocks.Get([]byte(hash))
			if data == nil {
				continue
			}
			fmt.Printf("Hashe no %d Hash %s\n", i, hash)
			var block Block
			buff := bytes.NewBuffer(data)
			err := gob.NewDecoder(buff).Decode(&block)
			if err != nil {
				return fmt.Errorf("error decoding block for hash %s: %w", hash, err)
			}

			requestedBlocks = append(requestedBlocks, block)
		}

		return nil
	})
	sort.Slice(requestedBlocks, func(i, j int) bool {
		return requestedBlocks[i].Height < requestedBlocks[j].Height
	})
	return requestedBlocks, err
}

func (dm *DBman) GetPrevHash() (string, error) {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	var prevHash string
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("prev_hash")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket %s doesn't exist", bucketName)
		}

		key := []byte("Prev")
		hash := bucket.Get(key)
		if hash == nil {
			return fmt.Errorf("prev hash doesn't exist in bucket %s", bucketName)
		}

		prevHash = string(hash)
		return nil
	})

	if err != nil {
		return "", err
	}

	return prevHash, nil
}
func (dm *DBman) LogAllTheBlocks() error {
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("Blocks")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("error occurred: bucket %s does not exist", bucketName)
		}

		cursor := bucket.Cursor()
		var index int
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			index++
			var buff bytes.Buffer
			buff.Write(v)
			decoder := gob.NewDecoder(&buff)
			var block Block
			if err := decoder.Decode(&block); err != nil {
				return fmt.Errorf("failed to decode block: %v", err)
			}

			// Pretty print the block information
			fmt.Printf("\n==================== Block %d ====================\n", block.Height)
			fmt.Printf("Hash:            %s\n", block.Hash)
			fmt.Printf("Difficulty:      %v\n", block.Header.Bits)
			fmt.Printf("Nonce:           %v\n", block.Header.Nonce)
			fmt.Printf("Merkle Root:     %s\n", block.Header.MerkleRoot)
			fmt.Printf("Previous Hash:   %s\n", block.Header.PrevBlockHash)
			fmt.Printf("Version:         %v\n", block.Header.Version)
			fmt.Printf("Block Height:    %v\n", block.Height)

			fmt.Println("\nTransactions:")
			for _, txn := range block.Transactions {
				fmt.Printf("  Txn Hash:      %v\n", txn.TxnHash)
				fmt.Printf("  Block Hash:    %v\n", txn.BlockHash)
				fmt.Printf("  Reciever:   %s\n", txn.Receiver)
				fmt.Printf("  Sender:   %s\n", txn.Sender)
				fmt.Printf("  Miner Subsidy: %v\n", txn.Subsidy)
				if txn.IsCoinbase {
					fmt.Print("No Inputs Since its a CoinBase Txn\n")
				} else {
					fmt.Println("  Outputs:")
					for _, otpt := range txn.Outputs {
						fmt.Printf("    Output ID:              %v\n", otpt.Id)
						fmt.Printf("    Previous Output Hash:   %v\n", otpt.PreviousOutput.Hash)
						fmt.Printf("    Output Value:           %v\n", otpt.Output.Value)
						fmt.Printf("    Output Script PubKey:   %v\n", otpt.Output.ScriptPubKey)
					}
				}
				fmt.Println("  -----------------------------")
			}

			fmt.Printf("==================================================\n")
		}
		return nil
	})
	return err
}
func (dm *DBman) IsBlockAlreadyPresent(hash string) (bool, error) {
	var exists bool
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Blocks"))
		if bucket == nil {
			return fmt.Errorf("bucket 'Blocks' does not exist")
		}
		exists = bucket.Get([]byte(hash)) != nil
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("error checking block presence: %w", err)
	}
	return exists, nil
}
func (dm *DBman) StoreBlock(block Block) error {
	exists, err := dm.IsBlockAlreadyPresent(block.Hash)
	if err != nil {
		return fmt.Errorf("failed to check if block exists: %w", err)
	}
	if exists {
		return nil
	}

	// Chain-tip protection: a block may only be appended if it explicitly
	// extends the current tip. Without this, ANY peer could send any valid-
	// PoW block and this node would store it AND repoint prev_hash at it —
	// a remote chain-tip hijack. Blocks that don't extend the tip are
	// rejected as orphans (fork/reorg handling is future work: competing
	// forks at the same height are simply not followed).
	tipHash, tipErr := dm.GetPrevHash()
	if tipErr == nil && tipHash != "" {
		if block.Header.PrevBlockHash != tipHash {
			return fmt.Errorf("rejected block %s: prev hash %s does not extend current tip %s (orphan/fork)",
				block.Hash, block.Header.PrevBlockHash, tipHash)
		}
		tipHeight, heightErr := dm.GetBlockHeight()
		if heightErr == nil && block.Height != tipHeight+1 {
			return fmt.Errorf("rejected block %s: height %d does not follow tip height %d",
				block.Hash, block.Height, tipHeight)
		}
	}
	// tipErr != nil means no tip exists yet (fresh DB) — allow genesis bootstrap.

	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	if err := encoder.Encode(block); err != nil {
		return fmt.Errorf("failed to encode block: %w", err)
	}
	err = dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Blocks"))
		if err != nil {
			return fmt.Errorf("failed to create or open bucket: %w", err)
		}

		key := []byte(block.Hash)
		if err := bucket.Put(key, buffer.Bytes()); err != nil {
			return fmt.Errorf("failed to store block: %w", err)
		}
		prevHashBucket := tx.Bucket([]byte("prev_hash"))
		err = prevHashBucket.Put([]byte("Prev"), []byte(block.Hash))
		if err != nil {
			return fmt.Errorf("error setting BlockPrev hash")
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, txn := range block.Transactions {
		if !txn.IsCoinbase {
			_, err := dm.DeleteMempoolTransaction(txn.TxnHash)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}

		}

	}

	return nil
}

// GetBlock retrieves a block from the database by its hash.
func (dm *DBman) GetBlock(hash string) (Block, error) {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	var block Block

	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Blocks"))
		if bucket == nil {
			return fmt.Errorf("bucket 'Blocks' not found")
		}

		data := bucket.Get([]byte(hash))
		if data == nil {
			return fmt.Errorf("block not found for hash: %s", hash)
		}

		buffer := bytes.NewBuffer(data)
		decoder := gob.NewDecoder(buffer)
		if err := decoder.Decode(&block); err != nil {
			return fmt.Errorf("failed to decode block: %w", err)
		}
		return nil
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

func (dm *DBman) FromNToLatestHash(hash string) ([]string, error) {
	var blockHashes []string

	err := dm.db.View(func(tx *bbolt.Tx) error {
		blocks := tx.Bucket([]byte("Blocks"))
		if blocks == nil {
			return fmt.Errorf("blocks bucket not found")
		}

		data := blocks.Get([]byte(hash))
		if data == nil {
			return fmt.Errorf("reference block hash %s not found", hash)
		}

		var referenceBlock Block
		if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&referenceBlock); err != nil {
			return fmt.Errorf("failed to decode reference block: %w", err)
		}

		blockHashes = make([]string, 0, 1000)

		cursor := blocks.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var block Block
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&block); err != nil {
				return fmt.Errorf("failed to decode block at cursor position: %w", err)
			}

			if block.Height > referenceBlock.Height {
				blockHashes = append(blockHashes, block.Hash)
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("database operation failed: %w", err)
	}

	return blockHashes, err
}

func (dm *DBman) IsTransactionAlreadyPresent(hash string) bool {
	var exists bool
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Transactions"))
		if bucket == nil {
			return fmt.Errorf("bucket Trasactions does not exist Problem with InitDb Function")
		}
		if data := bucket.Get([]byte(hash)); data != nil {
			exists = true
			return nil
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Ignoring Transaction its already present Txn hash %s\n", hash)
	}
	return exists
}

func (dm *DBman) StoreTransaction(hash string, txn Transaction) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	err := dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Transactions"))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		//* we dont have to do this beacuse if accidantly we do end up sending duplicate utxo itll get over written due to same hash but better to be safe than sorry
		if !dm.IsTransactionAlreadyPresent(hash) {
			var buff bytes.Buffer

			encoder := gob.NewEncoder(&buff)
			if err = encoder.Encode(txn); err != nil {

				return fmt.Errorf("serialize transaction: %w", err)
			}
			key := []byte(hash)
			return bucket.Put(key, buff.Bytes())
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error occured trying to store Transaction Hash %s", hash)
	}
	return nil
}

func (dm *DBman) GetTxnFromMempool(hash string) (Transaction, error) {
	var Transact Transaction
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("MemPoolTransactions")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket MemPoolTransactions Does not exist ")
		}
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			if bytes.Equal(k, []byte(hash)) {
				json.Unmarshal(v, &Transact)
			}
		}
		return nil
	})
	if err != nil {
		return Transaction{}, fmt.Errorf("error occured while trying to fetch Transaction with HASH %s In the Mempool", hash)
	}
	return Transact, nil
}
func (dm *DBman) HasTxnAlreadyBeenMined(hash string) (bool, error) {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	var exists bool
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("Transactions")

		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			exists = false
			return fmt.Errorf("error occured: Bucket %s Does Not Exist", bucketName)
		}

		data := bucket.Get([]byte(hash))

		exists = data != nil
		return nil
	})
	if err != nil {
		fmt.Printf("error occured trying to get Txn with `Hash`: %s From DB Err:-%v", hash, err)
	}
	return exists, nil
}

func (dm *DBman) StoreUTXO(hash string, utxo Utxo) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()

	return dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("UTXO"))
		if err != nil {
			return fmt.Errorf("failed to create UTXO bucket: %w", err)
		}

		// Check if UTXO already exists
		if existing := bucket.Get([]byte(hash)); existing != nil {
			return fmt.Errorf("UTXO with hash %s already exists", hash)
		}

		serialized, err := SerializeUtxo(utxo)
		if err != nil {
			return fmt.Errorf("failed to serialize UTXO: %w", err)
		}

		return bucket.Put([]byte(hash), serialized)
	})
}

func (dm *DBman) GetBlockHeight() (int, error) {
	var height int
	err := dm.db.View(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket([]byte("Blocks"))
		if bucket == nil {
			return fmt.Errorf("bucket Blocks Does not exist")
		}
		hash, err := dm.GetPrevHash()
		if err != nil {
			fmt.Printf("error occred trying to be get Previous hash")
		}
		sBlock := bucket.Get([]byte(hash))
		if sBlock != nil {
			var block Block
			var buff bytes.Buffer
			buff.Write(sBlock)
			err := gob.NewDecoder(&buff).Decode(&block)
			if err != nil {
				fmt.Printf("error decoding extracted block\n")
			}
			height = block.Height
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return height, nil
}

// bucketName := []byte("Blocks")
// bucket := tx.Bucket(bucketName)
// if bucket == nil {
// 	fmt.Errorf("bucket %s Buket Does Not exist", string(bucketName))
// }
// cursor := bucket.Cursor()
// for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
// 	prev_hash, err := dm.GetPrevHash() //*since we dont have the genesis block yet it might say prev has doesnt exist we should also check if the prev has cur hash are the same
// 	//*prev hash function returns the lastest block hash as everytime a new block is added its hash is set to prev hash
// 	if err != nil {
// 		return fmt.Errorf("error occured trying to get previous hash %v", err)
// 	}
// 	if bytes.Compare([]byte(prev_hash), []byte(hash)) == 0 {
// 		return []Block{}, nil, true
// 	}
// 	if bytes.Compare(k, []byte(prev_hash)) == 0 {

//		}
//	}
func (dm *DBman) GetUTXO(hash string) (Utxo, error) {
	var utxo Utxo
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()

	err := dm.db.View(func(tx *bbolt.Tx) error {
		BucketName := []byte("UTXO")
		bucket := tx.Bucket([]byte(BucketName))
		if bucket == nil {
			return fmt.Errorf("bucket doenst exists: %s", BucketName)
		}

		key := []byte(hash)

		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("extracted utxo was null")
		}
		u, err := DeserializeUTXO(data)
		if err != nil {
			return fmt.Errorf("error deserializingUtxo %e", err)
		}
		utxo = u
		return nil
	})
	if err != nil {
		return Utxo{}, fmt.Errorf("error extracting utxo %e", err)
	}
	return utxo, nil

}
func (dm *DBman) UTXOExists(hash string) (bool, error) {
	var exists bool

	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("UTXO"))
		if bucket == nil {
			return fmt.Errorf("bucket UTXO does not exist")
		}

		data := bucket.Get([]byte(hash))
		exists = (data != nil)
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("error checking UTXO existence: %w", err)
	}

	return exists, nil
}

func (dm *DBman) DeleteUtxo(hash string) error {
	err := dm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("UTXO"))
		if bucket == nil {
			return fmt.Errorf("error occurred: bucket UTXO does not exist")
		}

		err := bucket.Delete([]byte(hash))
		if err != nil {
			return fmt.Errorf("error deleting UTXO %s: %v", hash, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error occurred trying to delete UTXO %s: %v", hash, err)
	}
	fmt.Printf("\nDELETING UTXO %s\n", hash)
	return nil
}
func (dm *DBman) Close() {
	if dm.db != nil {
		fmt.Printf("Closing down DBMan")
		dm.db.Close()
	}
}
func (dm *DBman) GetBalance(pubkey string) (int64, []Utxo, error) {

	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	var balance int64
	var utxos []Utxo
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("UTXO")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		cursor := bucket.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			data, err := DeserializeUTXO(v)
			if err != nil {
				return fmt.Errorf("failed to deserialize UTXO: %v", err)
			}

			// Check if the key (key) matches the public key.
			if data.Output.ScriptPubKey == pubkey {
				utxos = append(utxos, data)
				balance += data.Output.Value
			}

		}
		return nil
	})

	if err != nil {
		return 0, []Utxo{}, fmt.Errorf("error getting balance for %s: %v", pubkey, err)
	}

	return balance, utxos, nil
}

