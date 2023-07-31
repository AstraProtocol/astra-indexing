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
	pwd, _ := os.Getwd()
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
	/*
		key := signature
		if strings.HasPrefix(key, "0x") {
			key = key[2:]
		}
	*/
	key := strings.TrimPrefix(signature, "0x")
	value, err := utils.db.Get([]byte(key))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (utils *EvmUtils) GetMethodNameFromMethodId(methodId string) string {
	key := strings.TrimPrefix(methodId, "0x")
	value, err := utils.db.Get([]byte(key))
	if err != nil {
		return ""
	} else {
		return strings.Split(string(value), "(")[0]
	}
}

func (utils *EvmUtils) GetMethodNameFromData(base64Data string) string {
	if base64Data == "" {
		return "transfer"
	}
	p, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
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
			return "0x" + h[0:8]
		}
	}
}

func (utils *EvmUtils) UpdateSignature(methodId string, signature string) {
	data := strings.Split(methodId, "x")
	if len(data) > 1 {
		methodId = data[1]
	} else {
		methodId = data[0]
	}
	utils.db.Put([]byte(methodId), []byte(signature))
}

func IsHexTx(hexTx string) bool {
	match, err := regexp.MatchString("^0x[a-fA-F0-9]{64}$", hexTx)
	if err != nil {
		return false
	}
	return match
}

func IsHexAddress(hexAddress string) bool {
	match, err := regexp.MatchString("^0x[a-fA-F0-9]{40}$", hexAddress)
	if err != nil {
		return false
	}
	return match
}
