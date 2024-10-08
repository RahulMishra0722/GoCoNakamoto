package main

import (
	"bytes"
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
	Utils "github.com/RahulMishra0722/GC/utils"
	"github.com/btcsuite/btcd/btcec"
	"go.uber.org/zap/buffer"
)

type Miner struct {
	DBman *DBMan.DBman
}

// ! TxnBlock Hash Is empty somethings wrong
// Todo[] store transactions
// Todo[] Store blocks and until we are testing make a function to mine a Genesis block on second thought we dont have to until everything is done
// Todo[] One more feature in wallet currently we clear the whole thing and that looses the public key and that will cause the user to enter public key every transaction
// Todo[] what would be really fun is a retry function it'll figure out if we should retry and will if we have to given the right conitions
const (
	Difficulty = 1
	Subsidy    = 50 // will get halved every 250000 blocks
	CurVersion = 0.1
)

type ProofOfWork struct {
	block  *DBMan.Block
	Target *big.Int
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

func (m *Miner) CreateCoinbaseTransaction() (DBMan.Transaction, string, string, DBMan.Utxo, error) {
	privateKey, publicKey, err := GenerateKeys()
	if err != nil {
		fmt.Printf("Error Generating Keys CoinBase %s", err)
	}
	coinBaseTxn := DBMan.Transaction{
		BlockHash:  "",
		TxnHash:    "", //Make a txn hasing function
		Inputs:     []DBMan.Utxo{},
		Outputs:    []DBMan.Utxo{},
		Subsidy:    50,
		Sender:     "",
		Receiver:   publicKey,
		LockTime:   0,
		Fee:        0.00,
		IsCoinbase: true,
		Height:     0, //* Mental note get the prev_hash from db and with that get block number and based on that +1 this
	}

	if err != nil {
		return DBMan.Transaction{}, "", "", DBMan.Utxo{}, fmt.Errorf("error Occured while trying to decompress pubkey from []byte To publicKey format %e", err)
	}

	output := DBMan.TxOutput{
		Value:        Subsidy,
		ScriptPubKey: publicKey,
	}
	outPoint := DBMan.OutPoint{

		Hash:  coinBaseTxn.CalculateHash(),
		Index: 0,
	}

	utxo := DBMan.Utxo{
		Id:             "",
		Output:         output,
		PreviousOutput: outPoint,
	}
	var buff buffer.Buffer

	encoder := gob.NewEncoder(&buff)
	err = encoder.Encode(utxo)
	if err != nil {
		fmt.Print(err)
	}
	//*Mental note i think i should calculate the txn hash just before adding to block
	//!Mental note i think i should calculate the txn hash just before adding to block
	utxo.Id = doubleSHA256(buff.Bytes())
	coinBaseTxn.Outputs = append(coinBaseTxn.Outputs, utxo)
	return coinBaseTxn, publicKey, privateKey, utxo, nil
}

//	func ProcessTransaction(Transactions []DBMan.Transaction) {
//		for _, txn := range Transactions {
//			txn.Signature
//		}
//	}
func HashBlock(header DBMan.BlockHeader, coinbase DBMan.Transaction) string {
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
func CalculateMerkleRoot(transactions []DBMan.Transaction) string {
	if len(transactions) == 0 {
		return ""
	}

	var hashes []string
	for _, tx := range transactions {
		hashes = append(hashes, tx.TxnHash)
	}

	for len(hashes) > 1 {
		var newHashes []string
		for i := 0; i < len(hashes); i += 2 {
			if i+1 == len(hashes) {
				newHashes = append(newHashes, hashes[i])
			} else {
				combinedHash := hashes[i] + hashes[i+1]
				hash := sha256.Sum256([]byte(combinedHash))
				newHashes = append(newHashes, doubleSHA256(hash[:]))
			}
		}
		hashes = newHashes
	}

	return hashes[0] // The last remaining hash is the Merkle root
}

func GenerateKeys() (string, string, error) {
	privateKey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return "", "", err
	}

	ecdsaPrivKey := privateKey.ToECDSA()
	compressedPubKey := privateKey.PubKey().SerializeCompressed()
	priv := Utils.PrivKeyToHex(ecdsaPrivKey)
	pub := Utils.PubKeyToHex(compressedPubKey)
	fmt.Print("\n Miner Reward Public key: ", pub, "\n")
	fmt.Print("\n Miner Reward Public key: ", priv, "\n")
	fmt.Print("\n")
	return priv, pub, nil
}

func ValidateSignature(pubKeyString string, transactionHash string, signatureHex string) (bool, error) {

	pubKeyBytes, err := hex.DecodeString(pubKeyString)
	if err != nil {
		return false, fmt.Errorf("error decoding public key: %v", err)
	}

	pubkey, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256())
	if err != nil {
		return false, fmt.Errorf("error parsing public key: %v", err)
	}

	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("error decoding signature: %v", err)
	}

	signature, err := btcec.ParseDERSignature(signatureBytes, btcec.S256())
	if err != nil {
		return false, fmt.Errorf("error parsing signature: %v", err)
	}

	txnHashBytes, err := hex.DecodeString(transactionHash)
	if err != nil {
		return false, fmt.Errorf("error decoding transaction hash: %v", err)
	}
	msgHash := sha256.Sum256(txnHashBytes)

	valid := signature.Verify(msgHash[:], pubkey)
	fmt.Print(valid)
	if !valid {
		fmt.Print("\n", "Signature failed", "\n")
	}
	fmt.Print("\n", "Signature Succeeded", "\n")
	return valid, nil
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
func (m *Miner) NewBlock(merkelRoot string, Transactions []DBMan.Transaction) (*DBMan.Block, string, string, DBMan.Utxo, []string, error) {
	fmt.Println("Mining starts")
	var successFullyMinedTransactions []string
	var ToBeAddedBlockTransactions []DBMan.Transaction
MainLoop:
	for _, txn := range Transactions {
		isValid, err := ValidateSignature(txn.Sender, txn.TxnHash, txn.Signature)
		if err != nil {
			return &DBMan.Block{}, "", "", DBMan.Utxo{}, []string{}, fmt.Errorf("invalid txn Signature verification falled with err: %v", err)
		}
		//Todo check if the utxo's even exist
		if isValid {
			// balance, err := m.DBman.GetBalance(txn.Sender)
			// if err != nil {
			// 	fmt.Print(err)
			// }
			// skipping the above part we'll just make sure that the wallet never sends the transaction if the user doenst have enough funds
			//but on second thought will have to check it any way because what if someone tries to mess whit it
			for _, coin := range txn.Inputs {
				exist, err := m.DBman.UTXOExists(coin.Id)
				if err != nil {
					fmt.Print(err)
				}
				if !exist {
					fmt.Print("\n", "Trying to double spend", "\n")
					break MainLoop
				}
			}

			var totalValue float64
			for _, tx := range txn.Inputs {
				totalValue += tx.Output.Value
			}
			if (totalValue - txn.Fee - txn.Amount) > 0 {
				fmt.Print("\n", txn.Amount, "\n")
				ReciverOutput := DBMan.TxOutput{
					Value:        txn.Amount,
					ScriptPubKey: txn.Receiver,
				}
				fmt.Print("\n", totalValue, "\n")
				sendersOutPut := DBMan.TxOutput{
					Value:        totalValue - txn.Amount - txn.Fee,
					ScriptPubKey: txn.Sender,
				}

				// sendersOutPutUtxo := DBMan.Utxo{
				// 	PreviousOutput: txn.Inp,
				// }
				var prevHash string
				for _, input := range txn.Inputs {
					prevHash = input.PreviousOutput.Hash // im not sure if the inputs will look like this but for now we pick any of the utxo input and grab the txnHash and exit
					break
				}
				recieverOutPoint := DBMan.OutPoint{
					Hash:  prevHash,
					Index: 0, //* ill only be allowing two outputs one and that is if the sender sent more than whats being transefered 1 otherwise so index 0, 1 and is fine unless i decide to change it
				}
				senderUtxoOutPoint := DBMan.OutPoint{
					Hash:  txn.TxnHash,
					Index: 1,
				}
				//*
				outputRecieverUtxo := DBMan.Utxo{
					Id:             "",
					PreviousOutput: recieverOutPoint,
					Output:         ReciverOutput,
				}
				var buff buffer.Buffer

				encoder := gob.NewEncoder(&buff)
				err = encoder.Encode(outputRecieverUtxo)
				if err != nil {
					fmt.Print(err)
				}

				outputRecieverUtxo.Id = doubleSHA256(buff.Bytes())
				//*
				outputSenderUtxo := DBMan.Utxo{
					Id:             "",
					PreviousOutput: senderUtxoOutPoint,
					Output:         sendersOutPut,
				}
				var SenderUtxoId buffer.Buffer

				SenderUtxoIdEncoder := gob.NewEncoder(&SenderUtxoId)
				err = SenderUtxoIdEncoder.Encode(outputSenderUtxo)
				if err != nil {
					fmt.Print(err)
				}

				outputSenderUtxo.Id = doubleSHA256(SenderUtxoId.Bytes())
				//* store the new utxos

				m.DBman.StoreUTXO(outputRecieverUtxo.Id, outputRecieverUtxo)
				m.DBman.StoreUTXO(outputSenderUtxo.Id, outputSenderUtxo)
				//*Deleting inputs
				for _, utxo := range txn.Inputs {
					m.DBman.DeleteUtxo(utxo.Id)
				}
				txn.Outputs = append(txn.Outputs, outputRecieverUtxo)
				txn.Outputs = append(txn.Outputs, outputSenderUtxo)
				ToBeAddedBlockTransactions = append(ToBeAddedBlockTransactions, txn)
				successFullyMinedTransactions = append(successFullyMinedTransactions, txn.TxnHash)
			} else if totalValue-txn.Fee-txn.Amount == 0 {
				ReciverOutput := DBMan.TxOutput{
					Value:        txn.Amount,
					ScriptPubKey: txn.Receiver,
				}

				// sendersOutPutUtxo := DBMan.Utxo{
				// 	PreviousOutput: txn.Inp,
				// }
				var prevHash string
				for _, input := range txn.Inputs {
					prevHash = input.PreviousOutput.Hash // im not sure if the inputs will look like this but for now we pick any of the utxo input and grab the txnHash and exit
					break
				}
				recieverOutPoint := DBMan.OutPoint{
					Hash:  prevHash,
					Index: 0, //* ill only be allowing two outputs one and that is if the sender sent more than whats being transefered 1 otherwise so index 0, 1 and is fine unless i decide to change it
				}

				outputRecieverUtxo := DBMan.Utxo{
					PreviousOutput: recieverOutPoint,
					Output:         ReciverOutput,
				}

				m.DBman.StoreUTXO(ReciverOutput.ScriptPubKey, outputRecieverUtxo)
				txn.Outputs = append(txn.Outputs, outputRecieverUtxo)
				successFullyMinedTransactions = append(successFullyMinedTransactions, txn.TxnHash)

			} else {
				return &DBMan.Block{}, "", "", DBMan.Utxo{}, []string{}, fmt.Errorf("insufficient balance")
			}
		}

	}

	cbaseTxn, minerPublicKey, minerPrivatekey, coinbaseUtxo, err := m.CreateCoinbaseTransaction() // push it at the front of the outputs array in block transaction

	if err != nil {
		fmt.Print(err)
	}
	Prevhash, err := m.DBman.GetPrevHash()
	if err != nil {
		fmt.Println(err)
		Prevhash = onTransactionEmpty()
	}

	blockHeader := DBMan.BlockHeader{
		Version:       CurVersion,
		PrevBlockHash: Prevhash,
		MerkleRoot:    merkelRoot,
		Timestamp:     time.Now(),
		Bits:          Difficulty,
		Nonce:         0,
	}
	Hash := HashBlock(blockHeader, cbaseTxn)

	// for _, txn := range ToBeAddedBlockTransactions {
	// 	txn.BlockHash = Hash
	// }
	cbaseTxn.TxnHash = cbaseTxn.CalculateHash() // Update txn hash before adding to the block
	cbaseTxn.BlockHash = Hash

	ToBeAddedBlockTransactions = append([]DBMan.Transaction{cbaseTxn}, ToBeAddedBlockTransactions...)

	block := &DBMan.Block{
		Height:       0,
		Header:       blockHeader,
		Transactions: ToBeAddedBlockTransactions,
		Hash:         Hash,
	}

	pow := NewProofOfWork(block)
	nonce, _ := pow.Run()

	block.Header.Nonce = nonce

	cbaseTxn.BlockHash = block.Hash
	// fmt.Print(cbaseTxn.BlockHash)

	if !pow.Validate() {
		fmt.Println("Invalid block: Proof of Work validation failed")
		os.Exit(1)
	}
	for _, txn := range ToBeAddedBlockTransactions {
		fmt.Printf("Transactions are  being Stored currently storing %v\n", txn.TxnHash)
		m.DBman.StoreTransactions(txn.TxnHash, txn)
	}

	return block, minerPublicKey, minerPrivatekey, coinbaseUtxo, successFullyMinedTransactions, nil
}

func NewProofOfWork(block *DBMan.Block) *ProofOfWork {
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
			// fmt.Printf("\rNonce found: %d", nonce)
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

func (m *Miner) MineBlock() (*DBMan.Block, string, string, DBMan.Utxo, []string) {
	// maybe we'll call all the nodes and ask for txns and also get it from mempool if there arent any meaning we'd have to mine an empty block which is fine
	//in newwer blockchains
	// we gotta distribute the keys somehow
	db, err := DBMan.GetDB()
	if err != nil {
		fmt.Print("\n", err, "\n")
	}

	transactions, err := db.GetMempoolTxns()

	if err != nil {

		fmt.Print(err)
	}

	if len(transactions) == 0 {
		//  here we will need to make empty blocks
		root := onTransactionEmpty()
		block, minerPublicKey, minerPrivatekey, coinbaseUtxo, successFullyMinedTransactions, err := m.NewBlock(root, transactions)
		if err != nil {
			fmt.Print(err)
		}
		return block, minerPublicKey, minerPrivatekey, coinbaseUtxo, successFullyMinedTransactions
	}

	root := CalculateMerkleRoot(transactions)
	block, minerPublicKey, minerPrivatekey, coinbaseUtxo, successFullyMinedTransactions, err := m.NewBlock(root, transactions)
	if err != nil {
		fmt.Printf("Error occured during block creation %e", err)
	}
	return block, minerPublicKey, minerPrivatekey, coinbaseUtxo, successFullyMinedTransactions
}

func InitializeMiner() (*Miner, error) {
	db, err := DBMan.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	mine := &Miner{
		DBman: db,
	}

	return mine, nil
}

func (m *Miner) GetMempoolTransactions() ([]DBMan.Transaction, error) {
	if m.DBman == nil {
		return nil, fmt.Errorf("database manager is not initialized")
	}

	txns, err := m.DBman.GetMempoolTxns()
	if err != nil {
		return nil, fmt.Errorf("failed to get mempool transactions: %w", err)
	}

	return txns, nil
}
func main() {
	miner, err := InitializeMiner()
	if err != nil {
		fmt.Print(err)
	}

	//Mine a block and retrieve coinbase UTXO

	block, _, _, coinbaseUtxo, successFullyMinedTransactions := miner.MineBlock()
	miner.DBman.StoreBlock(*block, block.Hash)

	// // coinbaseUtxo.Output.ScriptPubKey = []byte("03f614928ce8b1200e2bebdf4ae871467dc3d9b3961381e487424a5c45fe843a70")//test working
	for _, txnHash := range successFullyMinedTransactions {
		miner.DBman.DeleteMempoolTransaction(txnHash)
	}
	err = miner.DBman.StoreUTXO(coinbaseUtxo.Id, coinbaseUtxo)
	if err != nil {
		fmt.Print(err)
	}
	// txn, err := miner.DBman.GetTransactions("109154696d92897ad1b47131e449d3d6e7bcf928f5903f904d37e74c8a17e519")
	// if err != nil {
	// 	fmt.Print(err)
	// }
	// fmt.Printf("  Txn Hash:      %v\n", txn.TxnHash)
	// fmt.Printf("  Block Hash:    %v\n", txn.BlockHash)
	// fmt.Printf("  Is Coinbase:   %v\n", txn.IsCoinbase)
	// fmt.Printf("  Miner Subsidy: %v\n", txn.Subsidy)
	// if txn.IsCoinbase {
	// 	fmt.Print("No Inputs Since its a CoinBase Txn\n")
	// } else {
	// 	fmt.Println("  Outputs:")
	// 	for _, otpt := range txn.Outputs {
	// 		fmt.Printf("    Output ID:              %v\n", otpt.Id)
	// 		fmt.Printf("    Previous Output Hash:   %v\n", otpt.PreviousOutput.Hash)
	// 		fmt.Printf("    Output Value:           %v\n", otpt.Output.Value)
	// 		fmt.Printf("    Output Script PubKey:   %v\n", otpt.Output.ScriptPubKey)
	// 	}
	// }
	// fmt.Println("  Outputs:")
	// for _, otpt := range txn.Outputs {
	// 	fmt.Printf("    Output ID:              %v\n", otpt.Id)
	// 	fmt.Printf("    Previous Output Hash:   %v\n", otpt.PreviousOutput.Hash)
	// 	fmt.Printf("    Output Value:           %v\n", otpt.Output.Value)
	// 	fmt.Printf("    Output Script PubKey:   %v\n", otpt.Output.ScriptPubKey)
	// }
	// fmt.Println("  -----------------------------")

	// if err != nil {
	// 	fmt.Print(err)
	// }
	// _, _, err  mine.DBman.GetBalance(coinbaseUtxo.Output.ScriptPubKey)
	// if err != nil {
	// 	fmt.Print(err)
	// }
	// fmt.Print(coinbaseUtxo.Output.ScriptPubKey)
	// if err != nil {
	// 	fmt.Print(err)
	// }
	// fmt.Print("\n", balance, "\n")
	// fmt.Print"\n", utxo, "\n")

	// miner.DBman.ListAllUTXOs() //debug function

}
