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

func GetDB() (*DBman, error) {
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
func (txn Transaction) SerializeTxn() ([]byte, error) {
	var serialize bytes.Buffer

	// Serialize the Receiver with length prefix
	if err := binary.Write(&serialize, binary.LittleEndian, int32(len(txn.Receiver))); err != nil {
		return nil, err
	}
	// serialize.Write(txn.Receiver)

	if err := binary.Write(&serialize, binary.LittleEndian, int32(txn.Amount)); err != nil {
		return nil, err
	}
	if err := binary.Write(&serialize, binary.LittleEndian, int32(txn.LockTime)); err != nil {
		return nil, err
	}

	if txn.IsCoinbase {
		if err := binary.Write(&serialize, binary.LittleEndian, int32(txn.Subsidy)); err != nil {
			return nil, err
		}
	}

	bh, err := SerializeUnserializableData(txn.BlockHash)
	if err != nil {
		return nil, err
	}
	serialize.Write(bh)

	outputs, err := SerializeUnserializableData(txn.Outputs)
	if err != nil {
		return nil, err
	}
	serialize.Write(outputs)

	sig, err := SerializeUnserializableData(txn.Signature)
	if err != nil {
		return nil, err
	}
	serialize.Write(sig)

	return serialize.Bytes(), nil
}

func (txn *Transaction) DeSerializeTxn(data []byte) error {
	buf := bytes.NewReader(data)

	var receivedLen int32
	if err := binary.Read(buf, binary.LittleEndian, &receivedLen); err != nil {
		return err
	}
	var amount float32
	var lockTime int32
	if err := binary.Read(buf, binary.LittleEndian, &amount); err != nil {
		return err
	}
	txn.Amount = float64(amount)
	if err := binary.Read(buf, binary.LittleEndian, &lockTime); err != nil {
		return err
	}
	txn.LockTime = uint32(lockTime)
	if err := binary.Read(buf, binary.LittleEndian, &txn.IsCoinbase); err != nil {
		return err
	}
	if txn.IsCoinbase {
		var subsidy float64
		if err := binary.Read(buf, binary.LittleEndian, &subsidy); err != nil {
			return err
		}
		txn.Subsidy = subsidy
	}
	var bh []byte
	if err := gob.NewDecoder(buf).Decode(&bh); err != nil {
		return err
	}
	txn.BlockHash = string(bh)
	var sig []byte
	if err := gob.NewDecoder(buf).Decode(&sig); err != nil {
		return err
	}
	txn.Signature = string(sig)
	return nil
}

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
func (dm *DBman) AddToMempool(hash string, txn interface{}) error {
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
func (dm *DBman) StoreTransactions(hash string, txn Transaction) error {

	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	return dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Transactions"))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}

		serialized, err := txn.SerializeTxn()
		if err != nil {
			return fmt.Errorf("serialize transaction: %w", err)
		}
		key := []byte(hash)
		return bucket.Put(key, serialized)
	})
}
func (dm *DBman) GetTransactions(hash string) (Transaction, error) {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	var deserializedTxn Transaction
	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucketName := []byte("Transactions")
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket with name %s doesnt exists", bucketName)
		}
		key := []byte(hash)
		txn := bucket.Get(key)

		if txn == nil {
			return fmt.Errorf("bucket Transaction does not have a transaction with hash %s doesnt exists", hash)
		}
		err := deserializedTxn.DeSerializeTxn(txn)
		if err != nil {
			return fmt.Errorf("error deserializing the transaction %s", err)
		}

		return nil
	})
	if err != nil {
		return Transaction{}, fmt.Errorf("error getting transaction %s", err)
	}
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
	err := dm.db.Update(func(tx *bbolt.Tx) error { // Should use Update, not View, for modifying data
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
