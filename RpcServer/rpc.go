package RpcServer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/RahulMishra0722/GC/DBMan"
	"github.com/RahulMishra0722/GC/Server"
)

type RpcServer struct {
	Connman *Server.Server
	DBman   *DBMan.DBman
	Mempool map[string]DBMan.Transaction
}

// BalanceRequest: the field is a PUBLIC KEY (it's matched against
// ScriptPubKey). It used to be named PrivateKey and passed as a URL query
// param — both wrong: the name invited sending actual private keys, and
// URLs leak into logs/history.
type BalanceRequest struct {
	PublicKey string `json:"PublicKey"`
}

func NewRpcServer(server *Server.Server, DB *DBMan.DBman, mempool map[string]DBMan.Transaction) *RpcServer {
	return &RpcServer{
		Connman: server,
		DBman:   DB,
		Mempool: mempool,
	}
}

func (rpc *RpcServer) GetBalanceHandler(w http.ResponseWriter, r *http.Request) {
	// CORS is handled by the middleware in main.go — no per-handler headers.

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req BalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "error parsing request body", http.StatusBadRequest)
		return
	}
	publicKey := req.PublicKey
	if publicKey == "" {
		http.Error(w, "PublicKey is missing", http.StatusBadRequest)
		return
	}

	// Get balance from the DB based on the public key
	balance, utxo, err := rpc.DBman.GetBalance(publicKey)
	if err != nil {
		fmt.Print(err)
		http.Error(w, "Failed to fetch balance", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"PublicKey": publicKey,
		"balance":   balance,
		"UTXO":      utxo,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type TransactionRequest struct {
	Transaction DBMan.Transaction `json:"Transaction"`
}

func (rpc *RpcServer) SendTransaction(w http.ResponseWriter, r *http.Request) {
	log.Println("SendTransaction: Starting")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqOuter TransactionRequest

	// log.Println("SendTransaction: Decoding request body")
	err := json.NewDecoder(r.Body).Decode(&reqOuter)
	if err != nil {
		log.Printf("SendTransaction: Error decoding request body: %v", err)
		http.Error(w, "Error parsing request data", http.StatusBadRequest)
		return
	}

	req := reqOuter.Transaction
	valid, sigErr := DBMan.ValidateSignature(req.Sender, req.SigningHash, req.Signature, &req)
	if sigErr != nil || !valid {
		response := map[string]interface{}{
			"Response": "Invalid Signature transaction is being rejected",
		}
		w.WriteHeader(http.StatusNotAcceptable)
		json.NewEncoder(w).Encode(response)
		return
	}
	// Full verification (UTXO existence, balance against STORED values,
	// mempool dedup, mempool insert) + relay to peers — the same path a
	// peer-relayed transaction takes, so wallet txns and network txns
	// can't diverge in what they accept.
	relayErr, wasAdded := rpc.Connman.StoreInMempoolAndRelay(rpc.Connman.Ctx, req)
	if relayErr != nil {
		http.Error(w, fmt.Sprintf("transaction rejected: %v", relayErr), http.StatusBadRequest)
		return
	}
	if !wasAdded {
		response := map[string]interface{}{
			"Response": "Transaction already known to this node",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}
	response := map[string]interface{}{
		"Response": "We Have Recived Your Transaction wait till The Node will verifies your transaction",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
func (rpc *RpcServer) SubmitBlock(w http.ResponseWriter, r *http.Request) {
	// CORS is handled by the middleware in main.go.
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
		return
	}
	var reqBlock DBMan.Block

	if err := json.Unmarshal(body, &reqBlock); err != nil {
		http.Error(w, "error unmarshaling json", http.StatusBadRequest)
		return
	}
	ok, err := rpc.Connman.ProcessAndStoreNewMinedBlock(reqBlock)

	for _, txn := range reqBlock.Transactions {
		for _, opt := range txn.Outputs {
			fmt.Printf("Valuex %v", opt.Output.Value)
		}
	}
	if err != nil {
		fmt.Print(err)
		http.Error(w, fmt.Sprintf("Error storing block: %v", err), http.StatusInternalServerError)
		return
	}
	if ok {
		fmt.Printf("Block Stored Successfully \n")
	}
	// m.DBman.StoreUTXO(outputSenderUtxo.Id, outputSenderUtxo)
}
func (rpc *RpcServer) GetBlockTemplate(w http.ResponseWriter, r *http.Request) {
	// CORS is handled by the middleware in main.go.
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodOptions {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	template, err := rpc.Connman.BlockTemplate()

	if err != nil {
		//Todo
		//* 1) init func to create all the buckets and one to shove a genesis block so we never run into getting error on get PrevHash
		//* 20 change the code so its now compatible with http as in new format
		http.Error(w, "Error occured Trying to generate block template", http.StatusInternalServerError)
		return
	}

	errrr := json.NewEncoder(w).Encode(template)

	if errrr != nil {
		log.Printf("Error encoding block template: %v", err)
	}
}

// for transactions to be propogated through correctly without making the code ugly / re intailzing server to avoid we'd have to do this
// * --> Rpc -> Call a function inside server / Connman and that in turn calls DBMan to store in db so [--> RpcServer --> Server --> DBMan]
