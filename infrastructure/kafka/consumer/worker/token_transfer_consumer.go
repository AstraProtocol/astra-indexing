package consumer

import (
	"context"
	"os"
	"os/signal"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/infrastructure/kafka/consumer"
	"github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/segmentio/kafka-go"
)

func RunTokenTransferConsumer(rdbHandle *rdb.Handle, config *config.Config, logger applogger.Logger, evmUtil evm.EvmUtils, sigchan chan os.Signal) error {
	signal.Notify(sigchan, os.Interrupt)

	tokenTransferConsumer := consumer.Consumer[consumer.CollectedTokenTransfer]{
		TimeOut:            utils.KAFKA_TIME_OUT,
		Brokers:            config.KafkaService.Brokers,
		Topic:              utils.TOKEN_TRANSFERS_TOPIC,
		GroupId:            config.KafkaService.GroupID,
		User:               config.KafkaService.User,
		Password:           config.KafkaService.Password,
		AuthenticationType: config.KafkaService.AuthenticationType,
		Sigchan:            sigchan,
	}
	errConn := tokenTransferConsumer.CreateConnection()
	if errConn != nil {
		return errConn
	}

	tokenTransferConsumer.Fetch(
		consumer.CollectedTokenTransfer{},
		func(collectedTokenTransfer consumer.CollectedTokenTransfer, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Token Transfer Consumer error: %v", err)
			} else {
				//TODO: implement here
				logger.Infof("Not supported")
			}
		},
	)
	return nil
}
