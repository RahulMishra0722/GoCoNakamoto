package DBman

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

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
	return instance
}

type UTXO struct {
	TxID         string `json:"txhash"`
	OutputIndex  uint32 `json:"outputIndex"`
	Amount       int64  `json:"amount"`
	ScriptPubKey string `json:"scriptKey"`
	BlockHash    string `json:"blockhash"`
}

type Transaction struct {
	TxnHash  string `json:"txnHash"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"` // Fixed typo in "receiver"
	Inputs   []UTXO `json:"inputs"`
	Outputs  []UTXO `json:"outputs"`
}

type Block struct {
	BlockHash     string        `json:"blockhash"`
	PrevBlockHash string        `json:"prevBlockHash"`
	Transactions  []Transaction `json:"transactions"`
	Nonce         int64         `json:"nonce"`
}

func (txn *Transaction) SerializeTxn() ([]byte, error) {
	return json.Marshal(txn)
}

func DeserializeTxn(data []byte) ([]Transaction, error) {
	var retrievedTransactions []Transaction
	err := json.Unmarshal(data, &retrievedTransactions)
	return retrievedTransactions, err
}

func (utxo *UTXO) SerializeUtxo() ([]byte, error) {
	return json.Marshal(utxo)
}

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
