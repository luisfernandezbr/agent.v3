package main

import (
	"fmt"
	"os"

	"github.com/99designs/keyring"
	"github.com/mitchellh/go-homedir"
)

const storageKey = "pinpoint-example-config-encryption"

func main() {
	home, _ := homedir.Dir()
	if home == "" {
		panic("could not get homedir")
	}

	kr, err := keyring.Open(keyring.Config{
		ServiceName:                    "Pinpoint Example",
		KeychainTrustApplication:       true,
		KeychainSynchronizable:         false,
		KeychainAccessibleWhenUnlocked: false,
	})

	if err != nil {
		panic("could not get keyring")
	}

	if len(os.Args) < 2 {
		panic("pass get or set")
	}

	switch os.Args[1] {
	case "get":
		res, err := kr.Get(storageKey)
		if err != nil {
			if err == keyring.ErrKeyNotFound {
				return
			}
			panic(err)
		}
		fmt.Println(string(res.Data))
	case "set":
		if len(os.Args) < 3 {
			panic("pass data to set")
		}
		data := os.Args[2]
		if len(data) == 0 {
			panic("empty data")
		}
		err := kr.Set(keyring.Item{Key: storageKey, Data: []byte(data)})
		if err != nil {
			panic(err)
		}
	default:
		panic("unknown command")
	}
}
