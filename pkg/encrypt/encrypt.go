package encrypt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	pauth "github.com/pinpt/go-common/auth"
)

func GenerateKey() (string, error) {
	b, err := randBytes(32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func decodeKey(keyHex string) (key string, _ error) {
	b, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", err
	}
	if len(b) != 32 {
		return "", errors.New("want len(key) == 32")
	}
	return string(b), nil
}

func Encrypt(data []byte, keyHex string) ([]byte, error) {
	res, err := DecryptString(string(data), keyHex)
	if err != nil {
		return nil, err
	}
	return []byte(res), nil
}

func EncryptString(data string, keyHex string) (string, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return "", err
	}
	return pauth.EncryptString(string(data), key)
}

func Decrypt(data []byte, keyHex string) ([]byte, error) {
	res, err := DecryptString(string(data), keyHex)
	if err != nil {
		return nil, err
	}
	return []byte(res), nil
}

func DecryptString(data string, keyHex string) (string, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return "", err
	}
	return pauth.DecryptString(data, key)
}
