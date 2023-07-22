package consumer

import (
	"context"
	"encoding/hex"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/infrastructure/kafka/consumer"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/AstraProtocol/astra-indexing/projection/account_transaction"
	accountTransactionView "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	transactionView "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	"github.com/segmentio/kafka-go"
)

var transferCouponType = map[string]bool{
	"safeTransferFrom": true,
}

func RunTokenTransfersConsumer(rdbHandle *rdb.Handle, config *config.Config, logger applogger.Logger, evmUtil evm.EvmUtils, sigchan chan os.Signal) error {
	signal.Notify(sigchan, os.Interrupt)

	tokenTransfersConsumer := consumer.Consumer[consumer.CollectedTokenTransfer]{
		TimeOut:            utils.KAFKA_TIME_OUT,
		Brokers:            config.KafkaService.Brokers,
		Topic:              utils.TOKEN_TRANSFERS_TOPIC,
		GroupId:            config.KafkaService.GroupID,
		User:               config.KafkaService.User,
		Password:           config.KafkaService.Password,
		AuthenticationType: config.KafkaService.AuthenticationType,
		Sigchan:            sigchan,
	}
	errConn := tokenTransfersConsumer.CreateConnection()
	if errConn != nil {
		return errConn
	}

	rdbTransactionView := transactionView.NewTransactionsView(rdbHandle)
	rdbAccountTransactionsView := accountTransactionView.NewAccountTransactions(rdbHandle)
	rdbAccountTransactionDataView := accountTransactionView.NewAccountTransactionData(rdbHandle)

	tokenTransfersConsumer.Fetch(
		consumer.CollectedTokenTransfer{},
		func(collectedTokenTransfer consumer.CollectedTokenTransfer, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Token Transfer Consumer error: %v", err)
			} else {
				//get evm types by tx hashes
				tokenTransfer := collectedTokenTransfer.TokenTransfers[0]
				txHashes := []string{tokenTransfer.TransactionHash}
				evmTxTypes, err := rdbTransactionView.GetTxsTypeByEvmHashes(txHashes)
				if err != nil {
					logger.Infof("get txs type query error: %v", err)
				}

				//handle when tx was indexed to chainindexing db
				if len(evmTxTypes) > 0 {
					//index token transfer when tx type are valid
					if transferCouponType[evmTxTypes[0].TxType] {
						accountTransactionRows := make([]accountTransactionView.AccountTransactionBaseRow, 0)
						txs := make([]accountTransactionView.TransactionRow, 0)
						fee := coin.MustNewCoins(coin.MustNewCoinFromString("aastra", "0"))

						transactionInfo := account_transaction.NewTransactionInfo(
							accountTransactionView.AccountTransactionBaseRow{
								Account:      "",
								BlockHeight:  tokenTransfer.BlockNumber,
								BlockHash:    "",
								BlockTime:    utctime.UTCTime{},
								Hash:         tokenTransfer.TransactionHash,
								MessageTypes: []string{},
								Success:      true,
							},
						)

						converted, _ := hex.DecodeString(tokenTransfer.ToAddressHash[2:])
						toAstraAddr, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)

						transactionInfo.AddAccount(toAstraAddr)

						transactionInfo.Row.FromAddress = strings.ToLower(tokenTransfer.FromAddressHash)
						transactionInfo.Row.ToAddress = strings.ToLower(tokenTransfer.ToAddressHash)

						transactionInfo.AddMessageTypes(event.MSG_ETHEREUM_TX)

						blockHash := ""
						blockTime := utctime.Now()
						transactionInfo.FillBlockInfo(blockHash, blockTime)

						//parse token transfer to message content
						legacyTx := model.LegacyTx{
							Type:  tokenTransfer.TokenType,
							Gas:   "0",
							To:    tokenTransfer.ToAddressHash,
							Value: "0",
							Data:  strconv.FormatInt(tokenTransfer.TokenId, 10),
						}
						rawMsgEthereumTx := model.RawMsgEthereumTx{
							Type: event.MSG_ETHEREUM_TOKEN_TRANSFER,
							Size: 0,
							From: tokenTransfer.FromAddressHash,
							Hash: tokenTransfer.TransactionHash,
							Data: legacyTx,
						}
						params := model.MsgEthereumTxParams{
							RawMsgEthereumTx: rawMsgEthereumTx,
						}
						evmEvent := event.NewMsgEthereumTx(event.MsgCommonParams{
							BlockHeight: tokenTransfer.BlockNumber,
							TxHash:      tokenTransfer.TransactionHash,
							TxSuccess:   true,
							MsgIndex:    int(tokenTransfer.LogIndex),
						}, params)
						tmpMessage := accountTransactionView.TransactionRowMessage{
							Type:    event.MSG_ETHEREUM_TX,
							EvmType: evmTxTypes[0].TxType,
							Content: evmEvent,
						}

						tx := accountTransactionView.TransactionRow{
							BlockHeight:   tokenTransfer.BlockNumber,
							BlockTime:     blockTime,
							BlockHash:     blockHash,
							Hash:          tokenTransfer.TransactionHash,
							Index:         int(tokenTransfer.LogIndex),
							Success:       true,
							Code:          0,
							Log:           "",
							Fee:           fee,
							FeePayer:      "",
							FeeGranter:    "",
							GasWanted:     0,
							GasUsed:       0,
							Memo:          "",
							TimeoutHeight: 0,
							Messages:      make([]accountTransactionView.TransactionRowMessage, 0),
							EvmHash:       tokenTransfer.TransactionHash,
							RewardTxType:  evmTxTypes[0].TxType,
							FromAddress:   strings.ToLower(tokenTransfer.FromAddressHash),
							ToAddress:     strings.ToLower(tokenTransfer.ToAddressHash),
						}
						tx.Messages = append(tx.Messages, tmpMessage)
						txs = append(txs, tx)
						accountTransactionRows = append(accountTransactionRows, transactionInfo.ToRowsIncludingInternalTx()...)

						err = rdbAccountTransactionsView.InsertAll(accountTransactionRows)
						if err == nil {
							err = rdbAccountTransactionDataView.InsertAll(txs)
							//commit offset
							if err == nil {
								if errCommit := tokenTransfersConsumer.Commit(ctx, message); errCommit != nil {
									logger.Infof("Topic: %s. Consumer partition %d failed to commit messages: %v", utils.TOKEN_TRANSFERS_TOPIC, message.Partition, errCommit)
								}
							} else {
								logger.Infof("Failed to insert account txs data from Consumer partition %d: %v", message.Partition, err)
							}
						} else {
							logger.Infof("Failed to insert account txs from Consumer partition %d: %v", message.Partition, err)
						}
					} else {
						//commit offset when tx type are not valid
						if errCommit := tokenTransfersConsumer.Commit(ctx, message); errCommit != nil {
							logger.Infof("Topic: %s. Consumer partition %d failed to commit messages: %v", utils.TOKEN_TRANSFERS_TOPIC, message.Partition, errCommit)
						}
					}
				}
			}
		},
	)
	return nil
}
