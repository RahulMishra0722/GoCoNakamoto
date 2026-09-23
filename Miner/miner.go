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

	"github.com/RahulMishra0722/GoCoNakamoto/DBMan"
	MinerCacheMan "github.com/RahulMishra0722/GoCoNakamoto/MinerCasheMan"

	"github.com/btcsuite/btcd/btcec/v2"
	"go.uber.org/zap/buffer"
)

type Miner struct {
	casheMan *MinerCacheMan.MinerCacheMan
}

// ! should close the db after use it causes the db to get stuck
// Todo[] One more feature in wallet currently we clear the whole thing and that looses the public key and that will cause the user to enter public key every transaction
// Todo[] what would be really fun is a retry function it'll figure out if we should retry and will if we have to given the right conitions
const (
	// Subsidy is in whole coins; multiply by DBMan.SatoshisPerCoin when used.
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

func (m *Miner) CreateCoinbaseTransaction(totalFee int64) (DBMan.Transaction, error) {
	_, publicKey, err := GenerateKeys()
	if err != nil {
		fmt.Printf("Error Generating Keys CoinBase %s", err)
	}
	coinBaseTxn := DBMan.Transaction{
		BlockHash:  "",
		TxnHash:    "", //Make a txn hasing function
		Inputs:     []DBMan.Utxo{},
		Outputs:    []DBMan.Utxo{},
		Subsidy:    Subsidy * DBMan.SatoshisPerCoin,
		Sender:     "",
		Receiver:   publicKey,
		LockTime:   0,
		Fee:        0,
		IsCoinbase: true,
		Height:     0, //* Mental note get the prev_hash from db and with that get block number and based on that +1 this
	}

	if err != nil {
		return DBMan.Transaction{}, fmt.Errorf("error Occured while trying to decompress pubkey from []byte To publicKey format %e", err)
	}

	output := DBMan.TxOutput{
		Value:        Subsidy*DBMan.SatoshisPerCoin + totalFee,
		ScriptPubKey: publicKey,
	}
	outPoint := DBMan.OutPoint{
		Hash:  DBMan.CalculateTransactionHash(coinBaseTxn),
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

// CalculateMerkleRoot was deleted: the miner now uses
// DBMan.CalculateMerkleRoot — the exact function the validating node
// recomputes with. Two different merkle implementations (single vs double
// SHA, bytes vs concatenated hex strings) meant even identical transaction
// lists produced different roots, so blocks were unvalidatable by design.

func GenerateKeys() (string, string, error) {
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		return "", "", err
	}

	ecdsaPrivKey := privateKey.ToECDSA()
	compressedPubKey := privateKey.PubKey().SerializeCompressed()
	priv := DBMan.PrivKeyToHex(ecdsaPrivKey)
	pub := DBMan.PubKeyToHex(compressedPubKey)
	fmt.Print("\n Miner Reward Public key: ", pub, "\n")
	fmt.Print("\n Miner Reward Private key: ", priv, "\n")
	fmt.Print("\n")
	return priv, pub, nil
}
func (m *Miner) ProofOfWork(block *DBMan.BlockHeader) string {

	target := DBMan.CompactToBig(uint32(block.Bits))
	fmt.Printf("Target: %s\n", target.Text(16))
	fmt.Println("Mining is in Progress")
	for {

		hash := DBMan.CalculateBlockHash(*block)

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

func (m *Miner) NewBlock() (*DBMan.Block, error) {
	fmt.Println("Mining starts")
	var successFullyMinedTransactions []DBMan.Transaction
	var totalFee int64
	template, err := m.casheMan.GetBlockTemplate()
	if err != nil {
		return &DBMan.Block{}, fmt.Errorf("error getting block template: %w", err)
	}

	for _, txn := range template.Transactions {
		isValid, err := DBMan.ValidateSignature(txn.Sender, txn.SigningHash, txn.Signature, &txn)
		if err != nil {
			fmt.Printf("invalid txn Signature verification falled with err: %v", err)
			// If sig fails reject txn and continue
			continue
		}
		//Todo check if the utxo's even exist
		if isValid {

			var totalValue int64
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
				// BOTH outputs belong to THIS transaction, so both outpoints
				// must reference this txn's hash. Previously the receiver's
				// outpoint pointed at the INPUT's prev hash — the hash of the
				// transaction that created the UTXO being spent — which
				// corrupts UTXO identity: spending logic and double-spend
				// detection key on (prev hash, index).
				recieverOutPoint := DBMan.OutPoint{
					Hash:  txn.TxnHash,
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
				// This txn's own hash, like every outpoint of its outputs.
				recieverOutPoint := DBMan.OutPoint{
					Hash:  txn.TxnHash,
					Index: 0,
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
		return &DBMan.Block{}, fmt.Errorf("error creating coinbase transaction: %w", err)
	}

	// ORDERING RULE: the merkle root is a fingerprint of the FINAL
	// transaction list, so every txn must be final (outputs appended,
	// coinbase hash computed) BEFORE the root is computed. Previously the
	// root was taken from the raw template txns while the block shipped
	// a different list (coinbase prepended, outputs appended) — every
	// validating node recomputing the root rejected the block.
	cbaseTxn.TxnHash = DBMan.CalculateTransactionHash(cbaseTxn)

	finalTxns := append([]DBMan.Transaction{cbaseTxn}, successFullyMinedTransactions...)

	merkleRoot := DBMan.CalculateMerkleRoot(finalTxns)

	blockHeader := DBMan.BlockHeader{
		Version:       template.Version,
		PrevBlockHash: template.PrevBlockHash,
		MerkleRoot:    merkleRoot,
		Timestamp:     template.Timestamp,
		Bits:          template.Bits,
		Nonce:         0,
	}

	Hash := m.ProofOfWork(&blockHeader)

	// Bind every txn to the block it shipped in (only after the hash exists).
	for i := range finalTxns {
		finalTxns[i].BlockHash = Hash
	}

	block := &DBMan.Block{
		Height:       template.Height, // since the function returns in a way the last blocks height we gotta + 1 it
		Header:       blockHeader,
		Transactions: finalTxns,
		Hash:         Hash,
	}

	return block, nil
}

func (m *Miner) MineBlock() (*DBMan.Block, error) {
	// NOTE: the merkle root is deliberately NOT computed here. It must be
	// taken over the FINAL transaction list inside NewBlock, after sig
	// validation, output construction and the coinbase have been assembled.
	block, err := m.NewBlock()
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
				fmt.Printf("Utxo value %d", opt.Output.Value)
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
