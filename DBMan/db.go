package DBMan

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"strings"
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
	Value        float64 // Amount in satoshis (1 BTC = 100,000,000 satoshis) well  wont take this route of manging floats it gets trick very fast
	ScriptPubKey string  // Locking script, setting conditions to spend this output (e.g., requires a signature)
}

type Block struct {
	Height       int
	Header       BlockHeader   // The block header
	Hash         string        // Hash of the block
	Transactions []Transaction // List of other transactions in the block
	Difficulty   int
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

func (dm *DBman) GenerateGenesisBlock() (*Block, Transaction, error) {

	genesisMinerPubkey := "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048"

	coinbaseTx := Transaction{
		BlockHash: "",
		TxnHash:   "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048",
		Sender:    "",
		Receiver:  genesisMinerPubkey,
		Signature: "",
		Amount:    50.0,
		Inputs:    []Utxo{},
		Outputs: []Utxo{
			{
				Id: "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048",
				PreviousOutput: OutPoint{
					Hash:  "",
					Index: 0,
				},
				Output: TxOutput{
					Value:        50.0,
					ScriptPubKey: "020eb83857e3e0778ca6377b1fa171cf6e5f36ce0de59c72874e96086b99c32048",
				},
			},
		},
		LockTime:   0,
		IsCoinbase: true,
		Height:     1,
		Subsidy:    50.0,
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

	block := &Block{
		Height:       1,
		Header:       header,
		Hash:         "",
		Transactions: []Transaction{coinbaseTx},
	}
	// // Blockerr := dm.SetBlockHeight()
	// if Blockerr != nil {
	// 	return &Block{}, Transaction{}, fmt.Errorf("error Storing block height %w", Blockerr)
	// }

	merkleRoot := CalculateMerkleRoot(block.Transactions)
	block.Header.MerkleRoot = merkleRoot

	blockHash := CalculateBlockHash(block.Header)
	fmt.Println("Genesis Block hash %s", blockHash)
	block.Hash = blockHash

	block.Transactions[0].BlockHash = blockHash

	txHash := CalculateTransactionHash(block.Transactions[0])
	block.Transactions[0].TxnHash = txHash

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
func PrivKeyToHex(privKey *ecdsa.PrivateKey) string {
	return fmt.Sprintf("%064x", privKey.D)
}
func PubKeyToHex(pubKeyBytes []byte) string {
	return hex.EncodeToString(pubKeyBytes)
}

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
	return hex.EncodeToString(hash[:])
}

func CalculateTransactionHash(tx Transaction) string {
	txBytes := []byte(fmt.Sprintf("%s%s%s%f%d%d%f%f",
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

func InitDB() (*DBman, error) {
	db, err := GetDB()
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
	PrevhashErr := db.SetPrevioushash(block.Hash)
	if PrevhashErr != nil {
		return &DBman{}, fmt.Errorf("error Storing Prevhash %w", PrevhashErr)
	}
	return db, nil
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

			var total float64
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
					total += utxo.Output.Value
				}
			}

			log.Printf("VerifyTransaction: Total UTXO value: %f, Transaction amount: %f", total, transaction.Amount)
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
func (dm *DBman) AddToMempool(hash string, txn Transaction) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()

	valid, err := dm.AreInputsValid(txn.Inputs)
	if err != nil {
		return fmt.Errorf("validating inputs: %w", err)
	}
	if !valid {
		return fmt.Errorf("transaction rejected due to invalid inputs")
	}

	return dm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("MemPoolTransactions"))
		if bucket == nil {
			return fmt.Errorf("bucket MemPoolTransactions does not exist")
		}
		serialized, err := json.Marshal(txn)
		if err != nil {
			return fmt.Errorf("serialize transaction: %w", err)
		}
		key := []byte(hash)
		return bucket.Put(key, serialized)
	})
}

// * I know it could be done by utxo function i dont wanna edit the older function and make it incompatible for other related function

func (dm *DBman) AreInputsValid(inputs []Utxo) (bool, error) {
	var invalidInputs []string

	err := dm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("UTXO"))
		if bucket == nil {
			return fmt.Errorf("UTXO bucket does not exist; this indicates a serious database initialization problem")
		}

		for _, utxo := range inputs {
			if data := bucket.Get([]byte(utxo.Id)); data == nil {
				invalidInputs = append(invalidInputs, utxo.Id)
			}
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("database error while validating inputs: %w", err)
	}

	if len(invalidInputs) > 0 {
		return false, fmt.Errorf("invalid inputs found: %s", strings.Join(invalidInputs, ", "))
	}

	return true, nil
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

// setPrevioushash updates the previous hash value in the database.
func (dm *DBman) SetPrevioushash(hash string) error {
	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()
	err := dm.db.Update(func(tx *bbolt.Tx) error {
		fmt.Printf("Setting BlockHash %s\n", hash)
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

		return nil
	})
	if err != nil {
		return err
	}

	for _, txn := range block.Transactions {
		_, err := dm.DeleteMempoolTransaction(txn.TxnHash)
		if err != nil {
			fmt.Println(err)
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

// Todo a function to create all the buckets on start so we dont run into dumb issues
func (dm *DBman) StoreBlocks(senderAddr net.Addr, blocks *[]Block) (bool, error) {

	dm.rwMu.Lock()
	defer dm.rwMu.Unlock()

	err := dm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Blocks"))
		if bucket == nil {
			return fmt.Errorf("bucket 'Blocks' does not exist")
		}

		for i, block := range *blocks {
			var buff bytes.Buffer
			err := gob.NewEncoder(&buff).Encode(block)
			if err != nil {
				return fmt.Errorf("error encoding block %d: %v", i, err)
			}

			err = bucket.Put([]byte(block.Hash), buff.Bytes())
			if err != nil {
				return fmt.Errorf("error putting block %d into bucket: %v", i, err)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Println("Error storing blocks, attempting to store in temp bucket:", err)
		// Store in temp bucket for later retry
		err = dm.db.Update(func(tx *bbolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists([]byte("tempBlocksStorage"))
			if err != nil {
				return fmt.Errorf("error creating temp bucket: %v", err)
			}
			//! check if the error was on our side if so store the blocks in a
			//!temp bucket and after error checks retry also ask who has the
			//!longest block given all the peers we know of rn if the error was
			//! inside block / something was wrong init clear temp bucket can move on with our lives
			// Use a unique key for storing the block set
			var buff bytes.Buffer
			if err = gob.NewEncoder(&buff).Encode(blocks); err != nil {
				return fmt.Errorf("error encoding RecoveryBlocks: %v", err)
			}
			err = bucket.Put([]byte("temp"), buff.Bytes())
			if err != nil {
				return fmt.Errorf("error storing recovery blocks: %v", err)
			}

			return nil
		})
		if err != nil {
			return false, fmt.Errorf("failed to store recovery blocks: %v", err)
		}
		return false, nil
	}

	return true, nil
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
			fmt.Printf("ScriptPubKey: %v", utxo)

		}
		return nil
	})
}
