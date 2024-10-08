package Utils

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
)

type Utils struct {
}

func PrivKeyToHex(privKey *ecdsa.PrivateKey) string {
	return fmt.Sprintf("%064x", privKey.D)
}
func PubKeyToHex(pubKeyBytes []byte) string {
	return hex.EncodeToString(pubKeyBytes)
}
