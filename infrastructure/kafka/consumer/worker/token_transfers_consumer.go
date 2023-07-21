package consumer

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/infrastructure/kafka/consumer"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
	transactionView "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
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

	tokenTransfersConsumer.Fetch(
		consumer.CollectedTokenTransfer{},
		func(collectedTokenTransfer consumer.CollectedTokenTransfer, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Token Transfer Consumer error: %v", err)
			} else {
				//get evm types by tx hashes
				txHashes := []string{collectedTokenTransfer.TokenTransfers[0].TransactionHash}
				evmTxTypes, err := rdbTransactionView.GetTxsTypeByEvmHashes(txHashes)
				if err != nil {
					logger.Infof("get txs type query error: %v", err)
				}

				//handle when tx was indexed to chainindexing db
				if len(evmTxTypes) > 0 {
					//index token transfer when tx type are valid
					if transferCouponType[evmTxTypes[0].TxType] {
						//TODO: implement here
						fmt.Println("CHECK")
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
