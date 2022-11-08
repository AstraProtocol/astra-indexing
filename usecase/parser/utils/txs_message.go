package utils

import (
	"encoding/json"
	"regexp"
)

type MsgEvmBase struct {
	Type    string        `json:"type"`
	Content MsgEvmContent `json:"content"`
}

type MsgEvmContent struct {
	Name    string       `json:"name"`
	Version int          `json:"version"`
	Params  MsgEvmParams `json:"params"`
}

type MsgEvmParams struct {
	Hash string `json:"hash"`
}

func ParseMsgEvmTx(tx_message string) MsgEvmBase {
	var result MsgEvmBase

	if err := json.Unmarshal([]byte(tx_message), &result); err != nil {
		return MsgEvmBase{}
	}

	return result
}

func IsEvmTxHash(evm_tx_hash string) bool {
	match, err := regexp.MatchString("^0x[a-fA-F0-9]{64}$", evm_tx_hash)
	if err != nil {
		return false
	}
	return match
}
