package tmcosmosutils

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	event_usecase "github.com/AstraProtocol/astra-indexing/usecase/event"

	"github.com/btcsuite/btcutil/bech32"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	tmed25519 "github.com/tendermint/tendermint/crypto/ed25519"
)

func TmAddressFromTmPubKey(pubKey []byte) string {
	key := make(tmed25519.PubKey, ed25519.PubKeySize)
	copy(key, pubKey)

	return key.Address().String()
}

func MustConsensusAddressFromTmPubKey(bech32Prefix string, pubKey []byte) string {
	address, err := ConsensusNodeAddressFromTmPubKey(bech32Prefix, pubKey)
	if err != nil {
		panic(err)
	}

	return address
}

func ConsensusNodeAddressFromTmPubKey(bech32Prefix string, pubKey []byte) (string, error) {
	cosmosPubKey := &ed25519.PubKey{
		Key: pubKey,
	}

	conv, err := bech32.ConvertBits(cosmosPubKey.Address().Bytes(), 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("error converting tendermint public key to bech32 bits: %v", err)
	}
	address, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus address: %v", err)
	}

	return address, nil
}

func MustConsensusNodePubKeyFromTmPubKey(bech32Prefix string, pubKey []byte) string {
	address, err := ConsensusNodePubKeyFromTmPubKey(bech32Prefix, pubKey)
	if err != nil {
		panic(err)
	}

	return address
}

func ConsensusNodePubKeyFromTmPubKey(bech32Prefix string, pubKey []byte) (string, error) {
	pkToMarshal := ed25519.PubKey{Key: pubKey}

	conv, err := bech32.ConvertBits(legacy.Cdc.MustMarshal(pkToMarshal), 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("error converting tendermint public key to bech32 bits: %v", err)
	}
	address, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus public key: %v", err)
	}

	return address, nil
}

func MustAccountAddressFromValidatorAddress(bech32Prefix string, srcAddress string) string {
	return MustValidatorAddressFromAccountAddress(bech32Prefix, srcAddress)
}

func AccountAddressFromValidatorAddress(bech32Prefix string, srcAddress string) (string, error) {
	return ValidatorAddressFromAccountAddress(bech32Prefix, srcAddress)
}

func MustValidatorAddressFromAccountAddress(bech32Prefix string, srcAddress string) string {
	address, err := ValidatorAddressFromAccountAddress(bech32Prefix, srcAddress)
	if err != nil {
		panic(err)
	}

	return address
}

func ValidatorAddressFromAccountAddress(bech32Prefix string, srcAddress string) (string, error) {
	_, conv, err := bech32.Decode(srcAddress)
	if err != nil {
		return "", fmt.Errorf("error Decoding provided address: %v", err)
	}
	validatorAddress, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus address: %v", err)
	}

	return validatorAddress, nil
}

func ConsensusNodeAddressFromConsensusNodePubKey(bech32Prefix string, consensusNodePubKey string) (string, error) {
	_, conv, err := bech32.Decode(consensusNodePubKey)
	if err != nil {
		return "", fmt.Errorf("error converting consensus node pubkey to address")
	}

	pkToUnmarshal, err := bech32.ConvertBits(conv, 5, 8, false)
	if err != nil {
		return "", fmt.Errorf("error converting bech32 bits to tendermint public key: %v", err)
	}
	var pubKey cryptotypes.PubKey
	legacy.Cdc.MustUnmarshal(pkToUnmarshal, &pubKey)

	conv, err = bech32.ConvertBits(pubKey.Address().Bytes(), 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("error converting tendermint public key to bech32 bits: %v", err)
	}
	address, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus address: %v", err)
	}

	return address, nil
}

func MustAccountAddressFromPubKey(bech32Prefix string, pubKey []byte) string {
	address, err := AccountAddressFromPubKey(bech32Prefix, pubKey)
	if err != nil {
		panic(err)
	}

	return address
}

func AccountAddressFromPubKey(bech32Prefix string, pubKey []byte) (string, error) {
	pkToMarshal := secp256k1.PubKey{Key: pubKey}

	conv, err := bech32.ConvertBits(pkToMarshal.Address().Bytes(), 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("error converting tendermint public key to bech32 bits: %v", err)
	}
	address, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus address: %v", err)
	}

	return address, nil
}

func AccountAddressFromBytes(bech32Prefix string, b []byte) (string, error) {
	conv, err := bech32.ConvertBits(b, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("error converting tendermint public key to bech32 bits: %v", err)
	}
	address, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus address: %v", err)
	}

	return address, nil
}

func MustModuleAccountFromBytes(bech32Prefix string, b []byte) string {
	address, err := ModuleAccountFromBytes(bech32Prefix, b)
	if err != nil {
		panic(err)
	}

	return address
}

func ModuleAccountFromBytes(bech32Prefix string, b []byte) (string, error) {
	conv, err := bech32.ConvertBits(b, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("error converting tendermint public key to bech32 bits: %v", err)
	}
	address, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus address: %v", err)
	}

	return address, nil
}

func PubKeyFromCosmosPubKey(accountPubKey string) ([]byte, error) {
	_, conv, err := bech32.Decode(accountPubKey)
	if err != nil {
		return nil, fmt.Errorf("error bech32 decoding Cosmos public key")
	}

	pkToUnmarshal, err := bech32.ConvertBits(conv, 5, 8, false)
	if err != nil {
		return nil, fmt.Errorf("error converting bech32 bits to public key: %v", err)
	}

	var pubKey cryptotypes.PubKey
	legacy.Cdc.MustUnmarshal(pkToUnmarshal, &pubKey)

	return pubKey.Bytes(), nil
}

func MustMultiSigAddressFromPubKeys(
	bech32Prefix string,
	pubKeys [][]byte,
	threshold int,
	sortPubKeys bool,
) string {
	address, err := MultiSigAddressFromPubKeys(bech32Prefix, pubKeys, threshold, sortPubKeys)
	if err != nil {
		panic(err)
	}

	return address
}

func MultiSigAddressFromPubKeys(bech32Prefix string, pubKeys [][]byte, threshold int, sortPubKeys bool) (string, error) {
	pks := make([]cryptotypes.PubKey, 0, len(pubKeys))
	for _, pubKey := range pubKeys {
		pks = append(pks, &secp256k1.PubKey{Key: pubKey})
	}

	if sortPubKeys {
		sort.Slice(pks, func(i, j int) bool {
			return bytes.Compare(pks[i].Address(), pks[j].Address()) < 0
		})
	}

	aminoPubKey := multisig.NewLegacyAminoPubKey(threshold, pks)
	conv, err := bech32.ConvertBits(aminoPubKey.Address().Bytes(), 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("error converting tendermint public key to bech32 bits: %v", err)
	}
	address, err := bech32.Encode(bech32Prefix, conv)
	if err != nil {
		return "", fmt.Errorf("error encoding tendermint public key bits to consensus address: %v", err)
	}

	return address, nil
}

func IsValidCosmosAddress(address string) bool {
	_, conv, err := bech32.Decode(address)
	if err != nil {
		return false
	}

	_, err = bech32.ConvertBits(conv, 5, 8, false)
	return err == nil
}

func EncodeHexToAddress(hrp string, data []byte) (string, error) {
	converted, err := bech32.ConvertBits(data, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("encoding bech32 failed: %w", err)
	}
	return bech32.Encode(hrp, converted)
}

func DecodeAddressToHex(text string) (string, []byte, error) {
	hrp, data, err := bech32.Decode(text)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}
	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}
	return hrp, converted, nil
}

func ParseSenderAddressFromMsgEvent(msgEvent event_usecase.MsgEvent) string {
	msg := msgEvent.String()
	if strings.Contains(msg, "/ibc.core") {
		rgx := regexp.MustCompile(`Signer:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "Sender:") {
		rgx := regexp.MustCompile(`Sender:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "FunderAddress:") {
		rgx := regexp.MustCompile(`FunderAddress:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "FromAddress:") {
		rgx := regexp.MustCompile(`FromAddress:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "From:") {
		rgx := regexp.MustCompile(`From:"(0x[a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "Grantee:") {
		rgx := regexp.MustCompile(`Grantee:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "DelegatorAddress:") {
		rgx := regexp.MustCompile(`DelegatorAddress:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "ProposerAddress:") {
		rgx := regexp.MustCompile(`ProposerAddress:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "Depositor:") {
		rgx := regexp.MustCompile(`Depositor:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "Voter:") {
		rgx := regexp.MustCompile(`Voter:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "ValidatorAddr:") {
		rgx := regexp.MustCompile(`ValidatorAddr:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	} else if strings.Contains(msg, "ValidatorAddress:") {
		rgx := regexp.MustCompile(`ValidatorAddress:"([a-zA-Z0-9]+)"`)
		rs := rgx.FindStringSubmatch(msg)
		if len(rs) > 1 {
			return strings.ToLower(rs[1])
		}
	}
	return "cannot parse sender address from msg event"
}
