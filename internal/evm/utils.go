package evm

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/akrylysov/pogreb"
	"github.com/akrylysov/pogreb/fs"
	"os"
	"strings"
)

type EvmUtils struct {
	db *pogreb.DB
}

func NewEvmUtils() (EvmUtils, error) {
	pwd, err := os.Getwd()
	fmt.Println("hoank", pwd)
	db, err := pogreb.Open(pwd+"/4bytes.db", &pogreb.Options{FileSystem: fs.OSMMap})
	if err != nil {
		fmt.Println(err)
		return EvmUtils{}, err
	}
	return EvmUtils{
		db: db,
	}, nil
}

func (utils *EvmUtils) GetSignature(signature string) (string, error) {
	key := signature
	if strings.HasPrefix(key, "0x") {
		key = key[2:]
	}
	value, err := utils.db.Get([]byte(key))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (utils *EvmUtils) GetSignatureFromData(base64Data string) string {
	p, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		fmt.Println(err)
		return ""
	} else {
		h := hex.EncodeToString(p)
		value, err := utils.GetSignature(h[0:8])
		if err == nil {
			return strings.Split(value, "(")[0]
		} else {
			return ""
		}
	}
}
