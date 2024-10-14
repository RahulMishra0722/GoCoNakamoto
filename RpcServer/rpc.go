package RpcServer

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/RahulMishra0722/GC/DBMan"
	"github.com/RahulMishra0722/GC/Server"
)

type RpcServer struct {
	Connman *Server.Server
	DBman   *DBMan.DBman
}
type GetBalance struct {
	PrivateKey string `json:"PrivateKey"`
}

func NewRpcServer(server *Server.Server, DB *DBMan.DBman) *RpcServer {
	return &RpcServer{
		Connman: server,
		DBman:   DB,
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
	// Handle preflight request
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return // Respond with 200 OK for preflight request
	}

	// Set CORS headers for actual requests
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var reqOuter TransactionRequest
	// bodyBytes, _ := ioutil.ReadAll(r.Body)
	// fmt.Printf("Request body: %s\n", bodyBytes)
	// r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	err := json.NewDecoder(r.Body).Decode(&reqOuter)
	if err != nil {
		http.Error(w, "Error Parsing Req data", http.StatusBadRequest)
		return
	}

	req := reqOuter.Transaction

	err, stored := rpc.Connman.StoreInMempoolAndRelay(req)
	if !stored {
		response := map[string]interface{}{
			"Response": fmt.Sprintf("Errror adding Transaction to the meempool Node Responded with Error: %v", err),
		}
		json.NewEncoder(w).Encode(response)
	}

	if err != nil {
		http.Error(w, "Error Adding Transaction to mempool", http.StatusInternalServerError)
		return
	}
	fmt.Printf("Processed transaction: %+v\n", req)

	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"Response": "Your transaction has been received. Please wait while we process it.",
	}
	json.NewEncoder(w).Encode(response)
}

// for transactions to be propogated through correctly without making the code ugly / re intailzing server to avoid we'd have to do this
// * --> Rpc -> Call a function inside server / Connman and that in turn calls DBMan to store in db so [--> RpcServer --> Server --> DBMan]
