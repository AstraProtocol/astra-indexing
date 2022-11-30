package evm

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/akrylysov/pogreb"
	"github.com/akrylysov/pogreb/fs"
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
		if len(h) < 8 {
			return ""
		}
		value, err := utils.GetSignature(h[0:8])
		if err == nil {
			return strings.Split(value, "(")[0]
		} else {
			return ""
		}
	}
}

func IsHexTx(hex_tx string) bool {
	match, err := regexp.MatchString("^0x[a-fA-F0-9]{64}$", hex_tx)
	if err != nil {
		return false
	}
	return match
}

func IsHexAddress(hex_address string) bool {
	match, err := regexp.MatchString("^0x[a-fA-F0-9]{40}$", hex_address)
	if err != nil {
		return false
	}
	return match
}
