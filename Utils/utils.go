package Utils

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/RahulMishra0722/GC/DBMan"
)

type Utils struct {
}

// Encode serializes a transaction with gob for network transport.
func (u *Utils) Encode(data DBMan.Transaction) ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(data)
	if err != nil {
		return []byte{}, fmt.Errorf("error encoding data %v", err)
	}
	return buff.Bytes(), nil
}

// Decode deserializes a gob-encoded transaction.
func (u *Utils) Decode(data []byte, d interface{}) (DBMan.Transaction, error) {
	var txn DBMan.Transaction
	var buff = bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buff)
	err := decoder.Decode(&txn)
	if err != nil {
		return DBMan.Transaction{}, fmt.Errorf("error decoding data: %v (type: %T)", err, d)
	}
	return txn, nil
}
