package consumer

import (
	"context"
	"math/big"
	"os"
	"os/signal"
	"strings"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/segmentio/kafka-go"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/projection/account_transaction"
	accountTransactionView "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	transactionView "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const EVM_TXS_TOPIC = "evm-txs"
const INTERNAL_TXS_TOPIC = "internal-txs"

func RunConsumerEvmTxs(rdbHandle *rdb.Handle, config *config.Config, logger applogger.Logger) error {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	rdbTransactionView := transactionView.NewTransactionsView(rdbHandle)
	rdbAccountTransactionDataView := accountTransactionView.NewAccountTransactionData(rdbHandle)

	consumer := Consumer[[]CollectedEvmTx]{
		TimeOut:            utils.KAFKA_TIME_OUT,
		Brokers:            config.KafkaService.Brokers,
		Topic:              EVM_TXS_TOPIC,
		GroupId:            config.KafkaService.GroupID,
		User:               config.KafkaService.User,
		Password:           config.KafkaService.Password,
		AuthenticationType: config.KafkaService.AuthenticationType,
		Sigchan:            sigchan,
	}
	errConn := consumer.CreateConnection()
	if errConn != nil {
		return errConn
	}

	var mapValues []map[string]interface{}
	consumer.Fetch(
		[]CollectedEvmTx{},
		func(collectedEvmTxs []CollectedEvmTx, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Consumer error: %v", err)
			} else {
				mapValues = nil
				for _, evmTx := range collectedEvmTxs {
					feeValue := big.NewInt(0).Mul(big.NewInt(evmTx.GasUsed), big.NewInt(evmTx.GasPrice)).String()
					isSuccess := true
					if evmTx.Status == "error" {
						isSuccess = false
					}
					mapValue := map[string]interface{}{
						"evm_hash":  evmTx.TransactionHash,
						"fee_value": feeValue,
						"success":   isSuccess,
					}
					mapValues = append(mapValues, mapValue)
				}

				if len(mapValues) > 0 {
					errUpdate := rdbTransactionView.UpdateAll(mapValues)
					if errUpdate == nil {
						errUpdateTxData := rdbAccountTransactionDataView.UpdateAll(mapValues)
						// Commit offset
						if errUpdateTxData == nil {
							if errCommit := consumer.Commit(ctx, message); errCommit != nil {
								logger.Infof("Consumer partition %d failed to commit messages: %v", message.Partition, errCommit)
							}
						} else {
							logger.Infof("failed to update account txs data from Consumer partition %d: %v", message.Partition, errUpdate)
						}
					} else {
						logger.Infof("failed to update txs from Consumer partition %d: %v", message.Partition, errUpdate)
					}
				}
			}
		},
	)
	return nil
}

func RunConsumerInternalTxs(rdbHandle *rdb.Handle, config *config.Config, logger applogger.Logger) error {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	//rdbTransactionView := transactionView.NewTransactionsView(rdbHandle)
	rdbAccountTransactionDataView := accountTransactionView.NewAccountTransactionData(rdbHandle)

	consumer := Consumer[[]CollectedInternalTx]{
		TimeOut:            utils.KAFKA_TIME_OUT,
		Brokers:            config.KafkaService.Brokers,
		Topic:              INTERNAL_TXS_TOPIC,
		GroupId:            config.KafkaService.GroupID,
		User:               config.KafkaService.User,
		Password:           config.KafkaService.Password,
		AuthenticationType: config.KafkaService.AuthenticationType,
		Sigchan:            sigchan,
	}
	errConn := consumer.CreateConnection()
	if errConn != nil {
		return errConn
	}

	evmUtil, err := evm.NewEvmUtils()
	if err != nil {
		return err
	}

	consumer.Fetch(
		[]CollectedInternalTx{},
		func(collectedInternalTxs []CollectedInternalTx, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Consumer error: %v", err)
			} else {
				txs := make([]accountTransactionView.TransactionRow, 0)
				fee := coin.MustNewCoins(coin.MustNewCoinFromString("aastra", "0"))
				for _, internalTx := range collectedInternalTxs {
					transactionInfo := account_transaction.NewTransactionInfo(
						accountTransactionView.AccountTransactionBaseRow{
							Account:      "",
							BlockHeight:  internalTx.BlockNumber,
							BlockHash:    "",
							BlockTime:    utctime.UTCTime{},
							Hash:         internalTx.TransactionHash,
							MessageTypes: []string{},
							Success:      true,
						},
					)

					fromAstraAddr, _ := sdk.AccAddressFromHex(internalTx.FromAddressHash[2:])
					toAstraAddr, _ := sdk.AccAddressFromHex(internalTx.ToAddressHash[2:])

					transactionInfo.AddAccount(fromAstraAddr.String())
					transactionInfo.AddAccount(toAstraAddr.String())

					transactionInfo.Row.FromAddress = strings.ToLower(internalTx.FromAddressHash)
					transactionInfo.Row.ToAddress = strings.ToLower(internalTx.ToAddressHash)

					transactionInfo.AddMessageTypes(event.MSG_ETHEREUM_TX)

					blockHash := ""
					blockTime := utctime.Now()
					transactionInfo.FillBlockInfo(blockHash, blockTime)

					evmType := evmUtil.GetMethodNameFromMethodId(internalTx.Input[2:10])

					tmpMessage := accountTransactionView.TransactionRowMessage{
						Type:    event.MSG_ETHEREUM_TX,
						EvmType: evmType,
					}
					//TODO: parse message content
					tx := accountTransactionView.TransactionRow{
						BlockHeight:   internalTx.BlockNumber,
						BlockTime:     blockTime,
						BlockHash:     blockHash,
						Hash:          internalTx.TransactionHash,
						Index:         int(internalTx.Index),
						Success:       true,
						Code:          0,
						Log:           "",
						Fee:           fee,
						FeePayer:      "",
						FeeGranter:    "",
						GasWanted:     0,
						GasUsed:       int(internalTx.GasUsed),
						Memo:          "",
						TimeoutHeight: 0,
						Messages:      make([]accountTransactionView.TransactionRowMessage, 0),
						EvmHash:       internalTx.TransactionHash,
						RewardTxType:  evmType,
					}
					tx.Messages = append(tx.Messages, tmpMessage)
					txs = append(txs, tx)

					if insertErr := rdbAccountTransactionDataView.InsertAll(txs); insertErr != nil {

					}
				}
			}
		},
	)
	return nil
}
