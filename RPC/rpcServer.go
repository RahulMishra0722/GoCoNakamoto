package rpc

import (
	"encoding/json"
	"net/http"

	types "github.com/RahulMishra0722/GC/Types"
)

func GetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	var req types.GetBalance
	json.NewDecoder(r.Body).Decode(&req)

}
