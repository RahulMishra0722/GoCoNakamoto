package Utils

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/gob"
	"encoding/hex"
	"fmt"

	"github.com/RahulMishra0722/GC/DBMan"
	"github.com/RahulMishra0722/GC/TestDBMan"
)

type Utils struct {
}

func PrivKeyToHex(privKey *ecdsa.PrivateKey) string {
	return fmt.Sprintf("%064x", privKey.D)
}
func PubKeyToHex(pubKeyBytes []byte) string {
	return hex.EncodeToString(pubKeyBytes)
}
func (u *Utils) Encode(data DBMan.Transaction) ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(data)
	if err != nil {
		return []byte{}, fmt.Errorf("error encoding data %v", err)
	}
	return buff.Bytes(), nil
}
func (u *Utils) TestEncode(data TestDBMan.Transaction) ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(data)
	if err != nil {
		return []byte{}, fmt.Errorf("error encoding data %v", err)
	}
	return buff.Bytes(), nil
}
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
func (u *Utils) TestDecode(data []byte, d interface{}) (TestDBMan.Transaction, error) {
	var txn TestDBMan.Transaction
	var buff = bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buff)
	err := decoder.Decode(&txn)
	if err != nil {
		return TestDBMan.Transaction{}, fmt.Errorf("error decoding data: %v (type: %T)", err, d)
	}
	return txn, nil
}
