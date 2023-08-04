package consumer

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	"github.com/AstraProtocol/astra-indexing/infrastructure/kafka/consumer"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/segmentio/kafka-go"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/projection/account_transaction"
	accountTransactionView "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

var rewardType = map[string]bool{
	"sendReward":        true,
	"redeemReward":      true,
	"exchange":          true,
	"exchangeWithValue": true,
}

func RunInternalTxsConsumer(rdbHandle *rdb.Handle, config *config.Config, logger applogger.Logger, evmUtil evm.EvmUtils, sigchan chan os.Signal) error {
	signal.Notify(sigchan, os.Interrupt)

	rdbAccountTransactionsView := accountTransactionView.NewAccountTransactions(rdbHandle)
	rdbAccountTransactionDataView := accountTransactionView.NewAccountTransactionData(rdbHandle)

	internalTxsConsumer := consumer.Consumer[[]consumer.CollectedInternalTx]{
		TimeOut:            utils.KAFKA_TIME_OUT,
		Brokers:            config.KafkaService.Brokers,
		Topic:              utils.INTERNAL_TXS_TOPIC,
		GroupId:            config.KafkaService.GroupID,
		User:               config.KafkaService.User,
		Password:           config.KafkaService.Password,
		AuthenticationType: config.KafkaService.AuthenticationType,
		Sigchan:            sigchan,
	}
	errConn := internalTxsConsumer.CreateConnection()
	if errConn != nil {
		return errConn
	}

	internalTxsConsumer.Fetch(
		[]consumer.CollectedInternalTx{},
		func(collectedInternalTxs []consumer.CollectedInternalTx, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Internal Txs Consumer error: %v", err)
			} else {
				txTypeMapping := make(map[string]string)
				for _, internalTx := range collectedInternalTxs {
					if internalTx.Index == 0 && len(internalTx.Input) >= 10 {
						evmType := evmUtil.GetMethodNameFromMethodId(internalTx.Input[2:10])
						if rewardType[evmType] {
							txTypeMapping[internalTx.TransactionHash] = evmType
						}
					}
				}

				accountTransactionRows := make([]accountTransactionView.AccountTransactionBaseRow, 0)
				txs := make([]accountTransactionView.TransactionRow, 0)
				fee := coin.MustNewCoins(coin.MustNewCoinFromString("aastra", "0"))
				for _, internalTx := range collectedInternalTxs {
					if internalTx.CallType != "call" {
						continue
					}
					if internalTx.Value.String() == "0" {
						continue
					}
					if internalTx.FromAddressHash == "" || internalTx.ToAddressHash == "" {
						continue
					}
					//ignore if internal tx is not reward tx
					txType := txTypeMapping[internalTx.TransactionHash]
					if !rewardType[txType] {
						continue
					}
					//ignore if internal tx is same data with parent tx
					//blockscout approach
					if internalTx.Index == 0 {
						continue
					}

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

					converted, _ := hex.DecodeString(internalTx.FromAddressHash[2:])
					fromAstraAddr, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)

					converted, _ = hex.DecodeString(internalTx.ToAddressHash[2:])
					toAstraAddr, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)

					transactionInfo.AddAccount(fromAstraAddr)
					transactionInfo.AddAccount(toAstraAddr)

					transactionInfo.Row.FromAddress = strings.ToLower(internalTx.FromAddressHash)
					transactionInfo.Row.ToAddress = strings.ToLower(internalTx.ToAddressHash)

					transactionInfo.AddMessageTypes(event.MSG_ETHEREUM_TX)

					blockHash := ""
					blockTime := utctime.Now()
					transactionInfo.FillBlockInfo(blockHash, blockTime)

					//parse internal tx to message content
					legacyTx := model.LegacyTx{
						Type:  internalTx.CallType,
						Gas:   strconv.FormatInt(internalTx.GasUsed, 10),
						To:    internalTx.ToAddressHash,
						Value: string(internalTx.Value),
						Data:  internalTx.Input,
					}
					rawMsgEthereumTx := model.RawMsgEthereumTx{
						Type: event.MSG_ETHEREUM_INTERNAL_TX,
						Size: 0,
						From: internalTx.FromAddressHash,
						Hash: internalTx.TransactionHash,
						Data: legacyTx,
					}
					params := model.MsgEthereumTxParams{
						RawMsgEthereumTx: rawMsgEthereumTx,
					}
					evmEvent := event.NewMsgEthereumTx(event.MsgCommonParams{
						BlockHeight: internalTx.BlockNumber,
						TxHash:      internalTx.TransactionHash,
						TxSuccess:   true,
						MsgIndex:    int(internalTx.Index),
					}, params)
					tmpMessage := accountTransactionView.TransactionRowMessage{
						Type:    event.MSG_ETHEREUM_TX,
						EvmType: txType,
						Content: evmEvent,
					}

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
						GasWanted:     int(internalTx.Gas),
						GasUsed:       int(internalTx.GasUsed),
						Memo:          "",
						TimeoutHeight: 0,
						Messages:      make([]accountTransactionView.TransactionRowMessage, 0),
						EvmHash:       internalTx.TransactionHash,
						RewardTxType:  txType,
						FromAddress:   strings.ToLower(internalTx.FromAddressHash),
						ToAddress:     strings.ToLower(internalTx.ToAddressHash),
					}
					tx.Messages = append(tx.Messages, tmpMessage)
					txs = append(txs, tx)
					accountTransactionRows = append(accountTransactionRows, transactionInfo.ToRowsIncludingInternalTx()...)
				}
				if len(txs) == 0 {
					//commit offset when no internal txs are valid
					if errCommit := internalTxsConsumer.Commit(ctx, message); errCommit != nil {
						logger.Infof("Topic: %s. Consumer partition %d failed to commit messages: %v", utils.INTERNAL_TXS_TOPIC, message.Partition, errCommit)
					}
				}
				err = rdbAccountTransactionsView.InsertAll(accountTransactionRows)
				if err == nil {
					err = rdbAccountTransactionDataView.InsertAll(txs)
					//commit offset
					if err == nil {
						if errCommit := internalTxsConsumer.Commit(ctx, message); errCommit != nil {
							logger.Infof("Topic: %s. Consumer partition %d failed to commit messages: %v", utils.INTERNAL_TXS_TOPIC, message.Partition, errCommit)
						}
					} else {
						logger.Infof("Failed to insert account txs data from Consumer partition %d: %v", message.Partition, err)
					}
				} else {
					logger.Infof("Failed to insert account txs from Consumer partition %d: %v", message.Partition, err)
					//commit offset when duplicated message
					if strings.Contains(fmt.Sprint(err), "duplicate key value violates unique constraint") {
						if errCommit := internalTxsConsumer.Commit(ctx, message); errCommit != nil {
							logger.Infof("Topic: %s. Consumer partition %d failed to commit messages: %v", utils.INTERNAL_TXS_TOPIC, message.Partition, errCommit)
						}
					}
				}
			}
		},
	)
	return nil
}
