package main

//Todo dont let the txn get in from the wallet if its sig is invalid
import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RahulMishra0722/GC/DBMan"
	MinerCacheMan "github.com/RahulMishra0722/GC/MinerCasheMan"
	"github.com/RahulMishra0722/GC/Utils"

	"github.com/btcsuite/btcd/btcec"
	"go.uber.org/zap/buffer"
)

type Miner struct {
	casheMan *MinerCacheMan.MinerCacheMan
}

// ! should close the db after use it causes the db to get stuck
// Todo[] One more feature in wallet currently we clear the whole thing and that looses the public key and that will cause the user to enter public key every transaction
// Todo[] what would be really fun is a retry function it'll figure out if we should retry and will if we have to given the right conitions
const (
	Subsidy    = 50 // will get halved every 250000 blocks
	CurVersion = 0.1
)

func doubleSHA256(data []byte) string {
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])
	return hex.EncodeToString(hash2[:])
}

// func (m *Miner) NewUtxo(hash string) TxOutput {

// }

func (m *Miner) CreateCoinbaseTransaction(totalFee float64) (DBMan.Transaction, error) {
	_, publicKey, err := GenerateKeys()
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
		return DBMan.Transaction{}, fmt.Errorf("error Occured while trying to decompress pubkey from []byte To publicKey format %e", err)
	}

	output := DBMan.TxOutput{
		Value:        Subsidy + totalFee,
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
	return coinBaseTxn, nil
}
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
	fmt.Print("\n Miner Reward Private key: ", priv, "\n")
	fmt.Print("\n")
	return priv, pub, nil
}
func (m *Miner) ProofOfWork(block *DBMan.BlockHeader) string {

	target := CompactToBig(uint32(block.Bits))
	fmt.Printf("Target: %s\n", target.Text(16))
	fmt.Println("Mining is in Progress")
	for {

		hash := HashBlock(*block)

		hashInt := new(big.Int)
		hashInt.SetString(hash, 16)

		if hashInt.Cmp(target) == -1 {
			fmt.Printf("Valid hash found: %s with nonce: %d\n", hash, block.Nonce)
			return hash
		}
		//fmt.Printf("current hash %s current nonce %d\n", hash, block.Nonce)
		block.Nonce++

	}
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

type Template struct {
	PrevBlockHash string
	Transactions  []DBMan.Transaction
	Timestamp     time.Time
	Bits          int
}

func (m *Miner) NewBlock(merkelRoot string) (*DBMan.Block, error) {
	fmt.Println("Mining starts")
	var successFullyMinedTransactions []DBMan.Transaction
	var totalFee float64
	template, err := m.casheMan.GetBlockTemplate()
	if err != nil {
		fmt.Println(err)
	}

	for _, txn := range template.Transactions {
		isValid, err := ValidateSignature(txn.Sender, txn.TxnHash, txn.Signature)
		if err != nil {
			fmt.Printf("invalid txn Signature verification falled with err: %v", err)
			// If sig fails reject txn and continue
			continue
		}
		//Todo check if the utxo's even exist
		if isValid {

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
				var prevHash string
				for _, input := range txn.Inputs {
					prevHash = input.PreviousOutput.Hash // im not sure if the inputs will look like this but for now we pick any of the utxo input and grab the txnHash and exit
					// *Previos hash is the hash of the transaction that created the utxo, i know names confusing
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
				totalFee += txn.Fee
				txn.Outputs = append(txn.Outputs, outputRecieverUtxo)
				txn.Outputs = append(txn.Outputs, outputSenderUtxo)

				successFullyMinedTransactions = append(successFullyMinedTransactions, txn)
			} else if totalValue-txn.Fee-txn.Amount == 0 {
				ReciverOutput := DBMan.TxOutput{
					Value:        txn.Amount,
					ScriptPubKey: txn.Receiver,
				}
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

				//!m.DBman.StoreUTXO(ReciverOutput.ScriptPubKey, outputRecieverUtxo)
				totalFee += txn.Fee
				txn.Outputs = append(txn.Outputs, outputRecieverUtxo)
				successFullyMinedTransactions = append(successFullyMinedTransactions, txn)

			} else {
				return &DBMan.Block{}, fmt.Errorf("insufficient balance")
			}
		} else {
			continue
		}

	}

	cbaseTxn, err := m.CreateCoinbaseTransaction(totalFee) // push it at the front of the outputs array in block transaction

	if err != nil {
		fmt.Print(err)
	}

	blockHeader := DBMan.BlockHeader{
		Version:       template.Version,
		PrevBlockHash: template.PrevBlockHash,
		MerkleRoot:    merkelRoot,
		Timestamp:     template.Timestamp,
		Bits:          template.Bits,
		Nonce:         0,
	}

	Hash := m.ProofOfWork(&blockHeader)
	cbaseTxn.TxnHash = cbaseTxn.CalculateHash() // Update txn hash before adding to the block
	cbaseTxn.BlockHash = Hash

	successFullyMinedTransactions = append([]DBMan.Transaction{cbaseTxn}, successFullyMinedTransactions...)

	block := &DBMan.Block{
		Height:       template.Height, // since the function returns in a way the last blocks height we gotta + 1 it
		Header:       blockHeader,
		Transactions: successFullyMinedTransactions,
		Hash:         Hash,
	}

	cbaseTxn.BlockHash = block.Hash

	return block, nil
}

func onTransactionEmpty() string {
	return strings.Repeat("0", 64)
}

func (m *Miner) MineBlock() (*DBMan.Block, error) {
	template, err := m.casheMan.GetBlockTemplate()
	if err != nil {
		return &DBMan.Block{}, fmt.Errorf("error MineBlock %w", err)
	}

	if len(template.Transactions) == 0 {
		root := onTransactionEmpty()
		block, err :=
			m.NewBlock(
				root,
			)
		if err != nil {
			fmt.Printf("Error creating empty block: %v\n", err)
		}
		return block, err

	}
	root := CalculateMerkleRoot(template.Transactions)

	block, err := m.NewBlock(root)
	if err != nil {
		fmt.Printf("Error occurred during block creation: %v\n", err)
	}

	return block, err
}

func (m *Miner) GetBlockTemplate(urlStr string) (DBMan.BlockTemplate, error) {
	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return DBMan.BlockTemplate{}, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}

	if parsedURL.Host == "" {
		parts := strings.SplitN(parsedURL.Path, "/", 2)
		parsedURL.Host = parts[0]
		if len(parts) > 1 {
			parsedURL.Path = "/" + parts[1]
		} else {
			parsedURL.Path = ""
		}
	}

	// Reconstruct the URL
	urlStr = parsedURL.String()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(urlStr)
	if err != nil {
		return DBMan.BlockTemplate{}, fmt.Errorf("error sending GET request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DBMan.BlockTemplate{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DBMan.BlockTemplate{}, fmt.Errorf("error reading response body: %w", err)
	}

	//fmt.Printf("Raw body from response: %s\n", string(body)) // Log the raw response body

	var block DBMan.BlockTemplate
	err = json.Unmarshal(body, &block)
	if err != nil {
		return DBMan.BlockTemplate{}, fmt.Errorf("error unmarshaling block template: %w", err)
	}

	//fmt.Printf("Block template received and parsed successfully %v\n", block)
	return block, nil
}
func (m *Miner) SubmitBlock(urlStr string, block DBMan.Block) error {

	jsonBlock, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("error marshaling block %w", err)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonBlock))
	if err != nil {
		return fmt.Errorf("error creating req %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected error occured %d", resp.StatusCode)
	}
	m.casheMan.ClearDB()
	fmt.Println("block submitted successfully")
	return nil
}
func main() {
	casheMan, err := MinerCacheMan.NewMinerCacheMan()
	if err != nil {
		fmt.Printf("Error Initlizing DB %v", err)
	}
	miner := &Miner{
		casheMan: casheMan,
	}

	url := "http://localhost:8087/GetBlockTemplate"
	blockTemplate, err := miner.GetBlockTemplate(url)
	if err != nil {
		log.Fatalf("Failed to get block template: %v", err)
	}
	miner.casheMan.SetBlockTemplate(blockTemplate)

	block, err := miner.MineBlock()
	for i := 0; i < 1; i++ {
		for _, txn := range block.Transactions {
			for _, opt := range txn.Outputs {
				fmt.Printf("Utxo value %f", opt.Output.Value)
			}
		}
	}
	urlSend := "http://localhost:8087/SubmitBlock"

	sumbitError := miner.SubmitBlock(urlSend, *block)
	if sumbitError != nil {
		fmt.Printf("Error Submitting block %v", sumbitError)
	}

	if err != nil {
		fmt.Printf("Error MineBlock %v\n", err)
	}

}
