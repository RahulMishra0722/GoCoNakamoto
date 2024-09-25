package types

type GetBalance struct {
	Address string `json:"address"`
}
type GetBalanceResponse struct {
	Balance float64 `json:"balance"`
}
type UTXO struct {
	TxHash      [32]byte `json:"txid"`
	OutputIndex int      `json:"index"`
	Value       float64  `json:"amount"`
}
type SendTransaction struct {
	Signature  string `json:"signature"`
	InputUtxos []UTXO `json:"inputs"`
	Sender     string `json:"sender"`
	Reciver    string `json:"reciver"`
}
