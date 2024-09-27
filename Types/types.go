package types

type GetBalance struct {
	Address string `json:"address"`
}
type GetBalanceResponse struct {
	Balance float64 `json:"balance"`
}
type UTXO struct {
	TxID         string `json:"txhash"`
	OutputIndex  uint32 `json:"outputIndex"`
	Amount       int64  `json:"amount"`
	ScriptPubKey string `json:"scriptKey"`
	BlockHash    string `json:"blockhash"`
}
type Transaction struct {
	TxnHash  string `json:"txnHash"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"` // Fixed typo in "receiver"
	Inputs   []UTXO `json:"inputs"`
	Outputs  []UTXO `json:"outputs"`
}
