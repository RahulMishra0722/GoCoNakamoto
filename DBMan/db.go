package DBMan

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

type DBman struct {
	db   *bbolt.DB
	rwMu sync.RWMutex
}

var instance *DBman
var once sync.Once

func GetDB() *DBman {
	once.Do(func() {
		db, err := bbolt.Open("../../testDB", 0666, nil)
		if err != nil {
			log.Fatal(err)
		}
		instance = &DBman{
			db: db,
		}
	})
	if instance == nil || instance.db == nil {
		log.Fatal("Database is not initialized properly")
	}
	return instance
}

type CoinbaseTransaction struct {
	ID        string  // Unique identifier for the coinbase transaction
	Amount    float64 // Amount awarded to the miner
	Miner     string  // Address of the miner (newly generated)
	Signature string  // Signature for the coinbase transaction
}

// BlockHeader represents the structure of a simplified block header

type BlockHeader struct {
	Version       int
	PrevBlockHash string
	MerkleRoot    string
	Timestamp     time.Time
	Bits          int   // Difficulty target
	Nonce         int64 // The value we are trying to find
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
	Value     int64
	Publickey ecdsa.PublicKey
}

// OutPoint references a specific output in a previous transaction
type OutPoint struct {
	Hash  []byte
	Index uint32
}
type Block struct {
	Header       BlockHeader         // The block header
	Coinbase     CoinbaseTransaction // The coinbase transaction
	Hash         string              // Hash of the block
	Transactions []Transaction       // List of other transactions in the block
}

// CoinbaseTransaction represents the special coinbase transaction

// Block represents a complete block in the blockchain

func (txn *Transaction) SerializeTxn() ([]byte, error) {
	return json.Marshal(txn)
}

func DeserializeTxn(data []byte) ([]Transaction, error) {
	var retrievedTransactions []Transaction
	err := json.Unmarshal(data, &retrievedTransactions)
	return retrievedTransactions, err
}

// func (utxo *UTXO) SerializeUtxo() ([]byte, error) {
// 	return json.Marshal(utxo)
// }

func (dm *DBman) AddToMempool(txn Transaction) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	return dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("transactions"))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}

		serialized, err := txn.SerializeTxn()
		if err != nil {
			return fmt.Errorf("serialize transaction: %w", err)
		}
		key := []byte("txn")
		return bucket.Put(key, serialized)
	})
}

func (dm *DBman) IfExistPutInitElseMakeNew(txn Transaction) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	txns, err := dm.GetMempoolTxns()
	if err != nil {
		dm.AddToMempool(txn) //this means that that there hasnt been any activity related to mempool
		// and it hasnt been created so 	// create one
	}
	// if not get the old mempool data and add to the existing pool
	var transactionSlice []Transaction
	//appead the old data back
	transactionSlice = append(transactionSlice, txns...)
	//  appead the new
	transactionSlice = append(transactionSlice, txn)

	data, err := json.Marshal(transactionSlice)
	if err != nil {
		fmt.Print(err)
	}
	//Seralize
	dm.db.Update(func(tx *bbolt.Tx) error {
		key := []byte("txn")
		bucket := []byte("transactions")
		b := tx.Bucket(bucket)
		b.Put(key, data)
		return nil
	})
	return nil
}

// * will get called by miner and in turn by server/connmanger to pass it to other peers
func (dm *DBman) GetMempoolTxns() ([]Transaction, error) {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	var transactions []Transaction

	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("transactions"))
		if bucket == nil {
			return nil // No transactions bucket found
		}

		var txn Transaction
		key := []byte("txn")
		if err := json.Unmarshal(key, &txn); err != nil {
			return fmt.Errorf("unmarshal transaction: %w", err)

		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return transactions, nil
}

// setPrevioushash updates the previous hash value in the database.
func (dm *DBman) SetPrevioushash(hash string) error {
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

		// Convert the hash (byte slice) to string
		prevHash = string(hash)
		return nil
	})

	if err != nil {
		return "", err
	}

	return prevHash, nil
}

// StoreBlock stores a block in the database and updates the previous hash.
func (dm *DBman) StoreBlock(block Block, hash string) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)

	// Encode the block
	if err := encoder.Encode(block); err != nil {
		return fmt.Errorf("failed to encode block: %w", err)
	}

	// Store the block in the "Blocks" bucket
	err := dm.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("Blocks"))
		if err != nil {
			return fmt.Errorf("failed to create or open bucket: %w", err)
		}

		// Use the hash as the key for the block
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
