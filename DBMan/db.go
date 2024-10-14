package DBMan

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

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

func GetDB(path string) (*DBman, error) {
	if instance == nil {
		dbMutex.Lock()
		defer dbMutex.Unlock()

		if instance == nil {
			var err error
			once.Do(func() {
				db, openErr := bbolt.Open("../../testDB", 0666, nil)
				if openErr != nil {
					err = fmt.Errorf("failed to open database: %w", openErr)
					return
				}
				instance = &DBman{
					db: db,
				}
			})
			if err != nil {
				return nil, err
			}
		}
	}

	if instance == nil || instance.db == nil {
		return nil, fmt.Errorf("database is not initialized properly")
	}

	return instance, nil
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

type Transaction struct {
	BlockHash  string  //Block Hash what block this transaction was formed in
	TxnHash    string  // Transaction hash
	Sender     string  // Sender's public key (not used in coinbase)
	Receiver   string  // Coinbase transactions for rewarding miner
	Signature  string  // Signature of the transaction
	Amount     float64 // Total amount in the transaction (used for non-coinbase transactions)
	Inputs     []Utxo  // Inputs for the transaction
	Outputs    []Utxo  // Outputs for the transaction
	LockTime   uint32  // Lock time for the transaction
	IsCoinbase bool    // Indicates if the transaction is a coinbase transaction
	Height     uint32  // Block height for the coinbase transaction
	Subsidy    float64 // Mining subsidy amount for the coinbase transaction
	Fee        float64
}

// TxInput represents an input of a Bitcoin transaction
type OutPoint struct {
	Hash  string // Identifies which transaction created the UTXO
	Index uint32 // Refers to where the UTXO lies in the list of outputs in that transaction
}

type TxInput struct {
	Value          float64
	PreviousOutput OutPoint
	Publickey      ecdsa.PublicKey
}
type Utxo struct {
	Id             string
	PreviousOutput OutPoint
	Output         TxOutput
}

type TxOutput struct {
	Value        float64 // Amount in satoshis (1 BTC = 100,000,000 satoshis) well  wont take this route of manging floats ut gets trick very fast
	ScriptPubKey string  // Locking script, setting conditions to spend this output (e.g., requires a signature)
}

type Block struct {
	Height       int
	Header       BlockHeader   // The block header
	Hash         string        // Hash of the block
	Transactions []Transaction // List of other transactions in the block
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
func (tx *Transaction) CalculateHash() string {

	data := tx.BlockHash + tx.TxnHash + string(tx.Sender) + string(tx.Receiver) + tx.Signature

	// Serialize Amount
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, math.Float64bits(tx.Amount))
	data += string(amountBytes)

	// Serialize LockTime
	lockTimeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(lockTimeBytes, tx.LockTime)
	data += string(lockTimeBytes)

	// Serialize IsCoinbase
	isCoinbaseByte := []byte{0}
	if tx.IsCoinbase {
		isCoinbaseByte[0] = 1
	}
	data += string(isCoinbaseByte)

	// Serialize Height
	heightBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(heightBytes, tx.Height)
	data += string(heightBytes)

	// Serialize Subsidy
	subsidyBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(subsidyBytes, math.Float64bits(tx.Subsidy))
	data += string(subsidyBytes)

	// Create a SHA-256 hash of the serialized data
	hash := sha256.Sum256([]byte(data))

	// Return the hash as a hexadecimal string
	return hex.EncodeToString(hash[:])
}
func (dm *DBman) AddToMempool(hash string, txn Transaction) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()

	return dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("MemPoolTransactions"))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		serialized, err := json.Marshal(txn)
		if err != nil {
			return fmt.Errorf("serialize transaction: %w", err)
		}
		key := []byte(hash)
		return bucket.Put(key, serialized)
	})
}
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

// VerifyTransaction verifies the validity of a transaction.
func (dm *DBman) VerifyTransaction(transaction Transaction) (bool, error) {

	log.Printf("VerifyTransaction: Verifying transaction %s", transaction.TxnHash)
	var SuccessfullyAdded bool

	err := dm.db.View(func(tx *bbolt.Tx) error {
		log.Println("VerifyTransaction: Starting bbolt transaction for UTXO check")
		bucketName := []byte("UTXO")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			log.Println("VerifyTransaction: UTXO bucket does not exist")
			return fmt.Errorf("UTXO bucket not initialized")
		}

		var total float64
		for _, utxo := range transaction.Inputs {
			data := bucket.Get([]byte(utxo.Id))
			if data == nil {
				log.Printf("VerifyTransaction: Invalid input found for UTXO %s", utxo.Id)
				SuccessfullyAdded = false
				return fmt.Errorf("invalid Inputs Input-Id: %s", string(utxo.Id))
			}
			total += utxo.Output.Value
		}

		log.Printf("VerifyTransaction: Total UTXO value: %f, Transaction amount: %f", total, transaction.Amount)
		if total < transaction.Amount {
			log.Printf("VerifyTransaction: Insufficient balance for transaction %s", transaction.TxnHash)
			SuccessfullyAdded = false
			return fmt.Errorf("insufficient balance, transaction rejected")
		}

		exist, err := dm.IsInMempool(transaction.TxnHash)
		if err != nil {
			log.Printf("VerifyTransaction: Error checking mempool for transaction %s - %v", transaction.TxnHash, err)
			return fmt.Errorf("error occurred during mempool check")
		}

		if !exist {
			log.Printf("VerifyTransaction: Adding transaction %s to mempool", transaction.TxnHash)
			err := dm.AddToMempool(transaction.TxnHash, transaction)
			if err != nil {
				log.Printf("VerifyTransaction: Error adding transaction %s to mempool - %v", transaction.TxnHash, err)
				SuccessfullyAdded = false
				return fmt.Errorf("error adding transaction to mempool %v", err)
			}
			log.Printf("VerifyTransaction: Transaction %s successfully added to mempool", transaction.TxnHash)
			SuccessfullyAdded = true
		} else {
			log.Printf("VerifyTransaction: Transaction %s already exists in the mempool", transaction.TxnHash)
		}

		return nil
	})

	if err != nil {
		log.Printf("VerifyTransaction: Error occurred during transaction verification - %v", err)
		return SuccessfullyAdded, err
	}

	log.Printf("VerifyTransaction: Transaction %s verified successfully", transaction.TxnHash)
	return SuccessfullyAdded, nil
}
func (dm *DBman) GetMempoolTxns() ([]Transaction, error) {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	fmt.Println("GetMempoolTxns: Starting function")
	var transactions []Transaction

	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("MemPoolTransactions"))
		if bucket == nil {
			fmt.Println("GetMempoolTxns: No transactions bucket found")
			return nil
		}

		cursor := bucket.Cursor()
		iterationCount := 0
		maxIterations := 1000

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			iterationCount++
			if iterationCount > maxIterations {
				fmt.Printf("GetMempoolTxns: Exceeded maximum iterations (%d)\n", maxIterations)
				return fmt.Errorf("possible infinite loop detected")
			}

			fmt.Printf("GetMempoolTxns: Processing transaction %d, key: %s\n", iterationCount, string(k))

			var txn Transaction
			err := json.Unmarshal(v, &txn)
			if err != nil {
				fmt.Printf("GetMempoolTxns: Failed to deserialize transaction: %v\n", err)
				return fmt.Errorf("failed to deserialize transaction: %w", err)
			}

			transactions = append(transactions, txn)
			fmt.Printf("GetMempoolTxns: Transaction added, total count: %d\n", len(transactions))
		}

		// fmt.Printf("GetMempoolTxns: Finished processing. Total transactions: %d\n", len(transactions))
		return nil
	})

	if err != nil {
		fmt.Printf("GetMempoolTxns: Error occurred: %v\n", err)
		return nil, err
	}

	// fmt.Printf("GetMempoolTxns: Returning %d transactions\n", len(transactions))
	return transactions, nil
}

// setPrevioushash updates the previous hash value in the database.
func (dm *DBman) SetPrevioushash(hash string) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	err := dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("prev_hash"))
		if err != nil {
			return fmt.Errorf("failed to create or open bucket: %w", err)
		}
		key := []byte("Prev")
		if err := bucket.Put(key, []byte(hash)); err != nil {
			return fmt.Errorf("failed to set previous hash: %w", err)
		}
		return nil
	})
	return err
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
			fmt.Printf("\n==================== Block %d ====================\n", index)
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
				fmt.Printf("  Is Coinbase:   %v\n", txn.IsCoinbase)
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
				fmt.Println("  Outputs:")
				for _, otpt := range txn.Outputs {
					fmt.Printf("    Output ID:              %v\n", otpt.Id)
					fmt.Printf("    Previous Output Hash:   %v\n", otpt.PreviousOutput.Hash)
					fmt.Printf("    Output Value:           %v\n", otpt.Output.Value)
					fmt.Printf("    Output Script PubKey:   %v\n", otpt.Output.ScriptPubKey)
				}
				fmt.Println("  -----------------------------")
			}

			fmt.Printf("==================================================\n")
		}
		return nil
	})
	return err
}

func (dm *DBman) StoreBlock(block Block, hash string) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)

	if err := encoder.Encode(block); err != nil {
		return fmt.Errorf("failed to encode block: %w", err)
	}

	err := dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Blocks"))
		if err != nil {
			return fmt.Errorf("failed to create or open bucket: %w", err)
		}

		key := []byte(hash)
		if err := bucket.Put(key, buffer.Bytes()); err != nil {
			return fmt.Errorf("failed to store block: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = dm.SetBlockHeight()
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	// Set the previous hash
	if err := dm.SetPrevioushash(hash); err != nil {
		return fmt.Errorf("failed to set previous hash: %w", err)
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

// IsInMempool checks if a transaction with a given hash is in the mempool.

func (dm *DBman) GetBlocksFromGivenHashTillLatest(hash string) ([]byte, error, bool) { //*Bool is to indicate if both nodes are at the same level / have the same number of blocks
	var BlockHashThatTheNodeSent Block
	var LatestBlock Block
	var payloadBlocks []Block
	dm.db.View(func(tx *bbolt.Tx) error {

		bucketName := []byte("Blocks")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket block does not exists this shouldnt be happening to begin with %s", string(bucketName))
		}
		key := []byte(hash)
		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("block for the key %s does not exist", hash)
		}

		buffer := bytes.NewBuffer(data)
		decoder := gob.NewDecoder(buffer)
		if err := decoder.Decode(&BlockHashThatTheNodeSent); err != nil {
			return fmt.Errorf("failed to decode block with hash %s", hash)
		}
		prev_hash, err := dm.GetPrevHash()

		if err != nil {
			return fmt.Errorf("error occured getting previos hash Error%v", err)
		}

		Prevkey := []byte(prev_hash)
		latestBlockbyte := bucket.Get(Prevkey)
		if data == nil {
			return fmt.Errorf("block for the key %s does not exist", hash)
		}

		latestBuffer := bytes.NewBuffer(latestBlockbyte)
		Latestdecoder := gob.NewDecoder(latestBuffer)
		if err := Latestdecoder.Decode(&LatestBlock); err != nil {
			return fmt.Errorf("failed to decode block with hash %s", hash)
		}

		cursor := bucket.Cursor()
		numberOfBlocksNeeded := LatestBlock.Height - BlockHashThatTheNodeSent.Height + 1
		fmt.Println(numberOfBlocksNeeded)
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			if numberOfBlocksNeeded == 0 {
				break
			}
			var block Block
			buff := bytes.NewBuffer(v)
			decoder := gob.NewDecoder(buff)
			if err := decoder.Decode(&block); err != nil {
				return fmt.Errorf("error occured decoding block during itteration block hash %s", string(k))
			}
			//	fmt.Printf("height outside of the check %d\n", block.Height)
			//	fmt.Printf("BlockHashThatTheNodeSent__Height %d\n", BlockHashThatTheNodeSent.Height)
			if block.Height > BlockHashThatTheNodeSent.Height {
				fmt.Println(block.Height)
				payloadBlocks = append(payloadBlocks, block)
			}
			numberOfBlocksNeeded--
		}
		return nil
	})
	if len(payloadBlocks) == 0 {
		return []byte{}, fmt.Errorf("both Nodes have the same know blocks"), true
	}
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	if err := encoder.Encode(payloadBlocks); err != nil {
		return nil, fmt.Errorf("error encoding blocks: %v", err), false
	}
	return buff.Bytes(), nil, false
}
func (dm *DBman) StoreTransactions(hash string, txn Transaction) error {

	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	return dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Transactions"))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}

		var buff bytes.Buffer

		encoder := gob.NewEncoder(&buff)
		if err = encoder.Encode(txn); err != nil {
			return fmt.Errorf("serialize transaction: %w", err)
		}

		key := []byte(hash)
		return bucket.Put(key, buff.Bytes())
	})
}
func (dm *DBman) TransactionLogger() error {
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("Transactions")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket with name %s does not yet exist", bucketName)
		}
		cursor := bucket.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var txn Transaction
			var buff bytes.Buffer
			buff.Write(v)
			decoder := gob.NewDecoder(&buff)
			decoder.Decode(&txn)
			fmt.Printf("  Txn Hash:      %v\n", txn.TxnHash)
			fmt.Printf("  Block Hash:    %v\n", txn.BlockHash)
			fmt.Printf("  Is Coinbase:   %v\n", txn.IsCoinbase)
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
			fmt.Println("  Outputs:")
			for _, otpt := range txn.Outputs {
				fmt.Printf("    Output ID:              %v\n", otpt.Id)
				fmt.Printf("    Previous Output Hash:   %v\n", otpt.PreviousOutput.Hash)
				fmt.Printf("    Output Value:           %v\n", otpt.Output.Value)
				fmt.Printf("    Output Script PubKey:   %v\n", otpt.Output.ScriptPubKey)
			}
			fmt.Println("  -----------------------------")
		}
		return nil
	})
	return err
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
func (dm *DBman) GetTransactions(hash string) (interface{}, error) {
	dm.rwMu.RLock()
	defer dm.rwMu.RUnlock()

	var deserializedTxn interface{}
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("Transactions")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket 'Transactions' does not exist")
		}

		txnData := bucket.Get([]byte(hash))
		if txnData == nil {
			return fmt.Errorf("transaction with hash %s does not exist", hash)
		}

		err := gob.NewDecoder(bytes.NewBuffer(txnData)).Decode(&deserializedTxn)
		if err != nil {
			return fmt.Errorf("error deserializing transaction: %w", err)
		}

		return nil
	})

	if err != nil {
		return Transaction{}, fmt.Errorf("error getting transaction: %w", err)
	}

	fmt.Printf("Transaction being sent to peer from db: %v\n", deserializedTxn)
	return deserializedTxn, nil
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
func (dm *DBman) SetBlockHeight() error {
	return dm.db.Update(func(tx *bbolt.Tx) error {
		bucketName := []byte("BlockHeight")
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return fmt.Errorf("error creating %s bucket: %v", string(bucketName), err)
		}

		key := []byte("latestBlockHeight")
		value := bucket.Get(key)

		var currHeight int
		if value == nil {
			// Initialize height to 1
			currHeight = 1
			fmt.Println("Initializing block height to 1")
		} else {
			prevHeight := btoi(value)
			currHeight = prevHeight + 1
			fmt.Printf("Incrementing block height from %d to %d\n", prevHeight, currHeight)
		}

		err = bucket.Put(key, itob(currHeight))
		if err != nil {
			return fmt.Errorf("error putting new height: %v", err)
		}

		fmt.Printf("New block height set to %d\n", currHeight)
		return nil
	})
}

func (dm *DBman) GetBlockHeight() (int, error) {
	var height int
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("BlockHeight")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			fmt.Println("blockHeight bucket does not exist yet")
			return fmt.Errorf("blockHeight bucket does not exist yet")
		}
		key := []byte("latestBlockHeight")
		data := bucket.Get(key)
		if data == nil {
			fmt.Println("height is yet to be set")
			return fmt.Errorf("height is yet to be set")
		}
		height = btoi(data)
		fmt.Printf("Retrieved block height: %d\n", height)
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

func (dm *DBman) GetBalance(pubkey string) (float64, []Utxo, error) {

	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	var balance float64
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
func (dm *DBman) ListAllUTXOs() error {
	return dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("MemPoolTransactions"))
		if bucket == nil {
			return fmt.Errorf("UTXO bucket not found")
		}

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			utxo, err := DeserializeMempool(v)
			if err != nil {
				return fmt.Errorf("failed to deserialize UTXO: %v", err)
			}
			fmt.Printf("Key: %x\n", k)
			fmt.Printf("  ScriptPubKey: %x\n", utxo)

		}
		return nil
	})
}
