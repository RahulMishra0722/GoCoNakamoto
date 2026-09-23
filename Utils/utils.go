package Utils

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"encoding/gob"
	"encoding/hex"
	"fmt"

	"github.com/RahulMishra0722/GC/DBMan"
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

func (u *Utils) DecodeBlocks(blocks []byte) ([]DBMan.Block, error) {
	var Decodedblocks []DBMan.Block
	var buff bytes.Buffer
	buff.Write(blocks)
	err := gob.NewDecoder(&buff).Decode(&Decodedblocks)
	return Decodedblocks, err
}
func (u *Utils) DecodeHashes(data []byte) ([]string, error) {
	var blocks []string
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(&blocks); err != nil {
		return nil, fmt.Errorf("failed to decode hashes: %w", err)
	}
	return blocks, nil
}
func (u *Utils) HandleIncomingHashes(data []byte) ([]string, error) {
	// Create a scanner to read the data
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Split(bufio.ScanWords)

	var hashes []string
	for scanner.Scan() {
		hash := scanner.Text()
		if len(hash) == 64 && isValidHexString(hash) {
			hashes = append(hashes, hash)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning hashes: %v", err)
	}

	return hashes, nil
}

func isValidHexString(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

type Message struct {
	Version byte
	Cammand byte
	Length  uint16
	Data    []byte
}
