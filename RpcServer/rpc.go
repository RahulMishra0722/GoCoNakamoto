package RpcServer

import (
	"encoding/json"
	"fmt"
	"net/http"

	DBman "github.com/RahulMishra0722/GC/DBMan"
	types "github.com/RahulMishra0722/GC/Types"
)

type Transaction struct {
	TxnHash  string `json:"txnHash"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"` // Fixed typo in "receiver"
	Inputs   []UTXO `json:"inputs"`
	Outputs  []UTXO `json:"outputs"`
}
type RpcServer struct {
}

func (rpc *RpcServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	var req types.GetBalance
	json.NewDecoder(r.Body).Decode(&req)

}
func (rpc *RpcServer) Send(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req Transaction
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Convert types.Transaction to DBman.Transaction
	txn := DBman.Transaction{
		TxnHash:  req.TxnHash,
		Sender:   req.Sender,
		Receiver: req.Receiver,
		Inputs:   req.Inputs,
		Outputs:  req.Outputs,
	}

	db := DBman.GetDB()
	err = db.AddToMempool(txn)
	if err != nil {
		http.Error(w, "Failed to add to mempool", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Transaction added to mempool successfully")
}
