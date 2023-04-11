package consumer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
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
		StartOffset:      0,
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
		ctx, cancelFunction := context.WithTimeout(context.Background(), utils.KAFKA_NEW_DATA_MAX_WAIT*3)
		defer func() {
			// Do nothing
			cancelFunction()
		}()

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
		ctx, cancelFunction := context.WithTimeout(context.Background(), utils.KAFKA_NEW_DATA_MAX_WAIT*3)
		defer func() {
			// do nothing
			cancelFunction()
		}()

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
