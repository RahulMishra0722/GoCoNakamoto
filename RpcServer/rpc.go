package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/RahulMishra0722/GC/DBMan"
)

type RpcServer struct {
}

type GetBalance struct {
	PrivateKey string `json:"PrivateKey"`
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
	db, err := DBMan.GetDB()
	if err != nil {
		fmt.Print(err)
	}

	balance, utxo, err := db.GetBalance(privateKey)
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

type Transaction struct {
	BlockHash string  `json:"BlockHash"`
	TxnHash   string  `json:"TxnHash"`
	Sender    string  `json:"Sender"`
	Receiver  string  `json:"Receiver"`
	Signature string  `json:"Signature"`
	Amount    float64 `json:"Amount"`
	Inputs    []struct {
		Id             string `json:"Id"`
		PreviousOutput struct {
			Hash  string `json:"Hash"`
			Index int    `json:"Index"`
		} `json:"PreviousOutput"`
		Output struct {
			Value        float64 `json:"Value"`
			ScriptPubKey string  `json:"ScriptPubKey"`
		} `json:"Output"`
	} `json:"Inputs"`
	Outputs    []interface{} `json:"Outputs"`
	LockTime   int           `json:"LockTime"`
	IsCoinbase bool          `json:"IsCoinbase"`
	Height     int           `json:"Height"`
	Subsidy    int           `json:"Subsidy"`
	Fee        float64       `json:"Fee"`
}

type TransactionRequest struct {
	Transaction Transaction `json:"Transaction"`
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
	bodyBytes, _ := ioutil.ReadAll(r.Body)
	fmt.Printf("Request body: %s\n", bodyBytes)
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes)) // Recreate the body reader

	err := json.NewDecoder(r.Body).Decode(&reqOuter)
	if err != nil {
		http.Error(w, "Error Parsing Req data", http.StatusBadRequest)
		return
	}

	// Now we can access the actual Transaction object
	req := reqOuter.Transaction
	db, err := DBMan.GetDB()
	//Todo close the db after use it hangs the db in place and thus cant mine
	db.AddToMempool(req.TxnHash, req)

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

func main() {
	rpc := RpcServer{}
	router := http.NewServeMux()
	router.HandleFunc("/GetBalance", rpc.GetBalanceHandler)
	router.HandleFunc("/Send", rpc.SendTransaction)
	err := http.ListenAndServe(":8087", router)
	if err != nil {
		fmt.Print(err)
	}
}
