package utils

import (
	"fmt"

	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	"github.com/calvinlauyh/cosmosutils"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authztypes "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	feegranttypes "github.com/cosmos/cosmos-sdk/x/feegrant"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcconnectiontypes "github.com/cosmos/ibc-go/v3/modules/core/03-connection/types"
	ibcchanneltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibccommitmenttypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	ibctypes "github.com/cosmos/ibc-go/v3/modules/core/types"
	ethermintcryptotypes "github.com/evmos/ethermint/crypto/codec"
	ethereminttypes "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	vestingtypes "github.com/evmos/evmos/v6/x/vesting/types"
	jsoniter "github.com/json-iterator/go"
)

type TxDecoder struct {
	decoder *cosmosutils.Decoder
}

func NewTxDecoder() *TxDecoder {
	return &TxDecoder{
		cosmosutils.NewDecoder().RegisterInterfaces(RegisterDecoderInterfaces),
	}
}

func RegisterDecoderInterfaces(interfaceRegistry types.InterfaceRegistry) {
	std.RegisterInterfaces(interfaceRegistry)
	authtypes.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	crisistypes.RegisterInterfaces(interfaceRegistry)
	distributiontypes.RegisterInterfaces(interfaceRegistry)
	evidencetypes.RegisterInterfaces(interfaceRegistry)
	govtypes.RegisterInterfaces(interfaceRegistry)
	proposal.RegisterInterfaces(interfaceRegistry)
	slashingtypes.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)
	upgradetypes.RegisterInterfaces(interfaceRegistry)
	vestingtypes.RegisterInterfaces(interfaceRegistry)
	ibctypes.RegisterInterfaces(interfaceRegistry)
	ibcclienttypes.RegisterInterfaces(interfaceRegistry)
	ibctransfertypes.RegisterInterfaces(interfaceRegistry)
	ibcconnectiontypes.RegisterInterfaces(interfaceRegistry)
	ibcchanneltypes.RegisterInterfaces(interfaceRegistry)
	ibccommitmenttypes.RegisterInterfaces(interfaceRegistry)
	authztypes.RegisterInterfaces(interfaceRegistry)
	feegranttypes.RegisterInterfaces(interfaceRegistry)
	ethereminttypes.RegisterInterfaces(interfaceRegistry)
	ethermintcryptotypes.RegisterInterfaces(interfaceRegistry)
	evmtypes.RegisterInterfaces(interfaceRegistry)
}

func (decoder *TxDecoder) Decode(base64Tx string) (*model.CosmosTx, error) {
	rawTx, err := decoder.decoder.DecodeBase64(base64Tx)
	if err != nil {
		return nil, fmt.Errorf("error decoding transaction: %v", err)
	}

	txJSONBytes, err := rawTx.MarshalToJSON()
	if err != nil {
		return nil, fmt.Errorf("error encoding decoded transaction to JSON: %v", err)
	}

	var tx *model.CosmosTx
	if err := jsoniter.Unmarshal(txJSONBytes, &tx); err != nil {
		return nil, fmt.Errorf("error decoding transaction JSON: %v", err)
	}

	return tx, nil
}

func (decoder *TxDecoder) GetFee(base64Tx string) (coin.Coins, error) {
	tx, err := decoder.Decode(base64Tx)
	if err != nil {
		return nil, fmt.Errorf("error decoding transaction: %v", err)
	}

	return decoder.sumAmount(tx.AuthInfo.Fee.Amount)
}

func (decoder *TxDecoder) sumAmount(amounts []model.CosmosTxAuthInfoFeeAmount) (coin.Coins, error) {
	var err error

	coins := coin.NewEmptyCoins()
	for _, amount := range amounts {
		var amountCoin coin.Coin
		amountCoin, err = coin.NewCoinFromString(amount.Denom, amount.Amount)
		if err != nil {
			return nil, fmt.Errorf("error parsing amount %s to Coin: %v", amount.Amount, err)
		}
		coins = coins.Add(amountCoin)
	}

	return coins, nil
}

func SumAmount(amounts []model.CosmosTxAuthInfoFeeAmount) (coin.Coins, error) {
	var err error

	coins := coin.NewEmptyCoins()
	for _, amount := range amounts {
		var amountCoin coin.Coin
		amountCoin, err = coin.NewCoinFromString(amount.Denom, amount.Amount)
		if err != nil {
			return nil, fmt.Errorf("error parsing amount %s to Coin: %v", amount.Amount, err)
		}
		coins = coins.Add(amountCoin)
	}

	return coins, nil
}
