package consumer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	config "github.com/AstraProtocol/astra-indexing/bootstrap/config"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	transactionView "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type Consumer[T comparable] struct {
	reader   *kafka.Reader
	Brokers  []string
	Topic    string
	GroupId  string
	TimeOut  time.Duration
	User     string
	Password string
}

func (c *Consumer[T]) CreateConnection() error {
	caCert, err := os.ReadFile("infrastructure/kafka/ca-dev.crt")
	if err != nil {
		return fmt.Errorf("error reading ca cert file: %v", err)
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		return fmt.Errorf("error appending ca cert")
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		RootCAs:            caCertPool,
	}

	mechanism, err := scram.Mechanism(scram.SHA256, c.User, c.Password)
	if err != nil {
		return fmt.Errorf("error setup scram mechanism: %v", err)
	}
	dialer := &kafka.Dialer{
		Timeout:       c.TimeOut,
		DualStack:     true,
		TLS:           tlsConfig,
		SASLMechanism: mechanism,
	}

	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:          c.Brokers,
		Topic:            c.Topic,
		GroupID:          c.GroupId,
		MinBytes:         1,        // same value of Shopify/sarama
		MaxBytes:         57671680, // java client default
		MaxWait:          utils.KAFKA_NEW_DATA_MAX_WAIT,
		ReadBatchTimeout: utils.KAFKA_READ_BATCH_TIME_OUT,
		Dialer:           dialer,
		Logger:           kafka.LoggerFunc(logf),
		ErrorLogger:      kafka.LoggerFunc(logf),
	})
	return nil
}

// Auto commit offset
func (c *Consumer[T]) Read(model T, callback func(T, error)) {
	for {
		ctx := context.Background()
		message, err := c.reader.ReadMessage(ctx)

		if err != nil {
			callback(model, err)
			return
		}

		err = json.Unmarshal(message.Value, &model)

		if err != nil {
			callback(model, err)
			continue
		}

		callback(model, nil)
	}
}

func (c *Consumer[T]) Fetch(model T, callback func(T, kafka.Message, context.Context, error)) {
	for {
		ctx := context.Background()
		message, err := c.reader.FetchMessage(ctx)

		if err != nil {
			callback(model, message, ctx, err)
			return
		}

		err = json.Unmarshal(message.Value, &model)

		if err != nil {
			callback(model, message, ctx, err)
			continue
		}

		callback(model, message, ctx, nil)
	}
}

// Commit offset manual
func (c *Consumer[T]) Commit(ctx context.Context, msgs ...kafka.Message) error {
	return c.reader.CommitMessages(ctx, msgs...)
}

func (c *Consumer[T]) Close() error {
	return c.reader.Close()
}

func logf(msg string, a ...interface{}) {
	fmt.Printf(msg, a...)
	fmt.Println()
}

func RunConsumerEvmTxs(rdbHandle *rdb.Handle, config *config.Config, logger applogger.Logger) error {
	rdbTransactionView := transactionView.NewTransactionsView(rdbHandle)

	consumer := Consumer[CollectedEvmTx]{
		TimeOut:  utils.KAFKA_TIME_OUT,
		Brokers:  config.KafkaService.Brokers,
		Topic:    config.KafkaService.Topic,
		GroupId:  config.KafkaService.GroupID,
		User:     config.KafkaService.User,
		Password: config.KafkaService.Password,
	}
	errConn := consumer.CreateConnection()
	if errConn != nil {
		return errConn
	}

	var messages []kafka.Message
	var mapValues []map[string]interface{}
	blockNumber := int64(0)
	consumer.Fetch(
		CollectedEvmTx{},
		func(collectedEvmTx CollectedEvmTx, message kafka.Message, ctx context.Context, err error) {
			if err != nil {
				logger.Infof("Kafka Consumer error: %v", err)
			} else {
				if collectedEvmTx.BlockNumber != blockNumber {
					if len(mapValues) > 0 {
						errUpdate := rdbTransactionView.UpdateAll(mapValues)
						if errUpdate == nil {
							// Commit offset
							if errCommit := consumer.Commit(ctx, messages...); errCommit != nil {
								logger.Infof("Consumer partition %d failed to commit messages: %v", message.Partition, errCommit)
							}
						} else {
							logger.Infof("failed to update txs from Consumer partition %d: %v", message.Partition, errUpdate)
						}
					}

					// Reset status
					messages = nil
					mapValues = nil
					blockNumber = collectedEvmTx.BlockNumber
				}
				feeValue := big.NewInt(0).Mul(big.NewInt(collectedEvmTx.GasUsed), big.NewInt(collectedEvmTx.GasPrice)).String()

				isSuccess := true
				if collectedEvmTx.Status == "error" {
					isSuccess = false
				}

				mapValue := map[string]interface{}{
					"evm_hash":  collectedEvmTx.TransactionHash,
					"fee_value": feeValue,
					"success":   isSuccess,
				}

				mapValues = append(mapValues, mapValue)
				messages = append(messages, message)
			}
		},
	)

	return nil
}
