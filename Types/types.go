package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
)

func PrivKeyToHex(privKey *ecdsa.PrivateKey) string {
	return fmt.Sprintf("%064x", privKey.D) // Ensure 32 byte (64 hex char) representation
}

func PubKeyToHex(pubKeyBytes []byte) string {
	return hex.EncodeToString(pubKeyBytes)
}

func GenerateKeys() (string, string, error) {
	privateKey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return "", "", err
	}

	ecdsaPrivKey := privateKey.ToECDSA()
	compressedPubKey := privateKey.PubKey().SerializeCompressed()
	priv := PrivKeyToHex(ecdsaPrivKey)
	pub := PubKeyToHex(compressedPubKey)
	return priv, pub, nil
}

func SignMessage(privKeyHex string, message string) (string, error) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return "", fmt.Errorf("error decoding private key: %v", err)
	}

	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyBytes)

	messageHash := sha256.Sum256([]byte(message))

	signature, err := privKey.Sign(messageHash[:])
	if err != nil {
		return "", fmt.Errorf("error signing message: %v", err)
	}

	return hex.EncodeToString(signature.Serialize()), nil
}

func VerifySignature(pubKeyHex string, message string, signatureHex string) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false, fmt.Errorf("error decoding public key: %v", err)
	}
	pubKey, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256())
	if err != nil {
		return false, fmt.Errorf("error parsing public key: %v", err)
	}

	messageHash := sha256.Sum256([]byte(message))

	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("error decoding signature: %v", err)
	}

	signature, err := btcec.ParseSignature(signatureBytes, btcec.S256())
	if err != nil {
		return false, fmt.Errorf("error parsing signature: %v", err)
	}

	return signature.Verify(messageHash[:], pubKey), nil
}

func main() {
	privKey, pubKey, err := GenerateKeys()
	if err != nil {
		fmt.Println("Error generating keys:", err)
		return
	}

	fmt.Println("Private Key:", privKey)
	fmt.Println("Compressed Public Key:", pubKey)

	message := "Hello, World!"
	signature, err := SignMessage(privKey, message)
	if err != nil {
		fmt.Println("Error signing message:", err)
		return
	}
	fmt.Println("Signature:", signature)

	isValid, err := VerifySignature(pubKey, message, signature)
	if err != nil {
		fmt.Println("Error verifying signature:", err)
		return
	}
	fmt.Println("Signature is valid:", isValid)
}
