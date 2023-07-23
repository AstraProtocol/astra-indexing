package consumer

import (
	"context"
	"math/big"
	"os"
	"os/signal"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	"github.com/AstraProtocol/astra-indexing/infrastructure/kafka/consumer"
	"github.com/segmentio/kafka-go"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	accountTransactionView "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	transactionView "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
)

func RunEvmTxsConsumer(rdbHandle *rdb.Handle, config *config.Config, logger applogger.Logger, sigchan chan os.Signal) error {
	signal.Notify(sigchan, os.Interrupt)

	rdbTransactionView := transactionView.NewTransactionsView(rdbHandle)
	rdbAccountTransactionDataView := accountTransactionView.NewAccountTransactionData(rdbHandle)

	evmTxsConsumer := consumer.Consumer[[]consumer.CollectedEvmTx]{
		TimeOut:            utils.KAFKA_TIME_OUT,
		Brokers:            config.KafkaService.Brokers,
		Topic:              utils.EVM_TXS_TOPIC,
		GroupId:            config.KafkaService.GroupID,
		User:               config.KafkaService.User,
		Password:           config.KafkaService.Password,
		AuthenticationType: config.KafkaService.AuthenticationType,
		Sigchan:            sigchan,
	}
	errConn := evmTxsConsumer.CreateConnection()
	if errConn != nil {
		return errConn
	}

	var mapValues []map[string]interface{}
	evmTxsConsumer.Fetch(
		[]consumer.CollectedEvmTx{},
		func(collectedEvmTxs []consumer.CollectedEvmTx, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Evm Txs Consumer error: %v", err)
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
						//commit offset
						if errUpdateTxData == nil {
							if errCommit := evmTxsConsumer.Commit(ctx, message); errCommit != nil {
								logger.Infof("Topic: %s. Consumer partition %d failed to commit messages: %v", utils.EVM_TXS_TOPIC, message.Partition, errCommit)
							}
						} else {
							logger.Infof("Failed to update account txs data from Consumer partition %d: %v", utils.EVM_TXS_TOPIC, message.Partition, errUpdate)
						}
					} else {
						logger.Infof("Failed to update txs from Consumer partition %d: %v", message.Partition, errUpdate)
					}
				}
			}
		},
	)
	return nil
}
