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
type GetBalance struct {
	PrivateKey string `json:"PrivateKey"`
}

func NewRpcServer(server *Server.Server, DB *DBMan.DBman, mempool map[string]DBMan.Transaction) *RpcServer {
	return &RpcServer{
		Connman: server,
		DBman:   DB,
		Mempool: mempool,
	}
}
func (rpc *RpcServer) GetBalanceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	privateKey := r.URL.Query().Get("PublicKey")

	if privateKey == "" {
		http.Error(w, "PrivateKey is missing", http.StatusBadRequest)
		return
	}

	// Get balance from the DB based on the PrivateKey
	//Todo get it as a dependecy im going insane

	balance, utxo, err := rpc.DBman.GetBalance(privateKey)
	if err != nil {
		fmt.Print(err)
		http.Error(w, "Failed to fetch balance", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"PrivateKey": privateKey,
		"balance":    balance,
		"UTXO":       utxo,
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
	// log.Printf("SendTransaction: Received transaction: %+v", req)

	rpc.Connman.DBMan.AddToMempool(req.TxnHash, req)
	response := map[string]interface{}{
		"Response": "We Have Recived Your Transaction wait till The Node will verifies your transaction",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
func (rpc *RpcServer) SubmitBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
	}
	var reqBlock DBMan.Block

	if err := json.Unmarshal(body, &reqBlock); err != nil {
		http.Error(w, "error unmarshaling json", http.StatusBadRequest)
	}
	ok, err := rpc.Connman.ProcessAndStoreNewMinedBlock(reqBlock)

	for _, txn := range reqBlock.Transactions {
		for _, opt := range txn.Outputs {
			fmt.Printf("Value %v", opt.Output.Value)
		}
	}
	if err != nil {
		fmt.Print(err)
		http.Error(w, "Error storing block %w", http.StatusInternalServerError)
	}

	if ok {
		fmt.Print("STORED YOHOO")
	}
	// m.DBman.StoreUTXO(outputSenderUtxo.Id, outputSenderUtxo)
}
func (rpc *RpcServer) GetBlockTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	template, err := rpc.Connman.BlockTemplate()

	if err != nil {
		//Todo
		//* 1) init func to create all the buckets and one to shove a genesis block so we never run into getting error on get PrevHash
		//* 20 change the code so its now compatible with http as in new format
		json.NewEncoder(w).Encode("Error occured Trying to generate block template")
	}

	errrr := json.NewEncoder(w).Encode(template)

	if errrr != nil {
		log.Printf("Error encoding block template: %v", err)
	}
}

// for transactions to be propogated through correctly without making the code ugly / re intailzing server to avoid we'd have to do this
// * --> Rpc -> Call a function inside server / Connman and that in turn calls DBMan to store in db so [--> RpcServer --> Server --> DBMan]
