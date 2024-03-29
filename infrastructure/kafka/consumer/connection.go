package consumer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"

	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
)

type Consumer[T any] struct {
	reader             *kafka.Reader
	Brokers            []string
	Topic              string
	GroupId            string
	TimeOut            time.Duration
	User               string
	Password           string
	AuthenticationType string
	Sigchan            chan os.Signal
	CaCertPath         string
	TlsCertPath        string
	TlsKeyPath         string
}

func (c *Consumer[T]) CreateConnection() error {
	dialer, err := c.getDialer()
	if err != nil {
		return fmt.Errorf("error setup dialer: %v", err)
	}
	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:               c.Brokers,
		Topic:                 c.Topic,
		GroupID:               c.GroupId,
		MinBytes:              1,        // same value of Shopify/sarama
		MaxBytes:              57671680, // java client default
		MaxWait:               utils.KAFKA_NEW_DATA_MAX_WAIT,
		ReadBatchTimeout:      utils.KAFKA_READ_BATCH_TIME_OUT,
		Dialer:                dialer,
		WatchPartitionChanges: true,
		ReadBackoffMax:        utils.KAFKA_READ_BACKOFF_MAX,
		ErrorLogger:           kafka.LoggerFunc(logf),
		//Logger:                kafka.LoggerFunc(logf),
	})
	return nil
}

// Auto commit offset
func (c *Consumer[T]) Read(model T, callback func(T, error)) {
	for {
		select {
		case <-c.Sigchan:
			c.Close()
			return
		default:
			ctx := context.Background()
			message, err := c.reader.ReadMessage(ctx)

			if err != nil {
				callback(model, err)
				continue
			}

			err = json.Unmarshal(message.Value, &model)

			if err != nil {
				callback(model, err)
				continue
			}

			callback(model, nil)
		}
	}
}

func (c *Consumer[T]) Fetch(model T, callback func(T, kafka.Message, context.Context, error)) {
	for {
		select {
		case <-c.Sigchan:
			c.Close()
			return
		default:
			ctx := context.Background()
			message, err := c.reader.FetchMessage(ctx)

			if err != nil {
				callback(model, message, ctx, err)
				continue
			}

			err = json.Unmarshal(message.Value, &model)

			if err != nil {
				callback(model, message, ctx, err)
				continue
			}

			callback(model, message, ctx, nil)
		}
	}
}

// Commit offset manual
func (c *Consumer[T]) Commit(ctx context.Context, msgs ...kafka.Message) error {
	return c.reader.CommitMessages(ctx, msgs...)
}

func (c *Consumer[T]) Close() error {
	return c.reader.Close()
}

func (c *Consumer[T]) getDialer() (*kafka.Dialer, error) {
	switch c.AuthenticationType {
	case "SASL":
		caCert, err := os.ReadFile(utils.CA_CERT_PATH)
		if err != nil {
			caCert, err = os.ReadFile(utils.CA_CERT_LOCAL_PATH)
		}
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, errors.New("failed to parse CA Certificate file")
		}
		tlsConfig := &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		}
		mechanism, err := scram.Mechanism(scram.SHA256, c.User, c.Password)
		if err != nil {
			return nil, err
		}
		dialer := &kafka.Dialer{
			Timeout:       c.TimeOut,
			KeepAlive:     time.Hour,
			DualStack:     true,
			TLS:           tlsConfig,
			SASLMechanism: mechanism,
		}
		return dialer, nil
	case "SSL":
		caCertPath := utils.CA_CERT_PATH
		if c.CaCertPath != "" {
			caCertPath = c.CaCertPath
		}

		tlsCertPath := utils.TLS_CERT_PATH
		if c.TlsCertPath != "" {
			tlsCertPath = c.TlsCertPath
		}

		tlsKeyPath := utils.TLS_KEY_PATH
		if c.TlsKeyPath != "" {
			tlsKeyPath = c.TlsKeyPath
		}

		keypair, err := tls.LoadX509KeyPair(
			tlsCertPath, tlsKeyPath,
		)
		if err != nil {
			return nil, err
		}
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, errors.New("failed to parse CA Certificate file")
		}
		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{keypair},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		}
		dialer := &kafka.Dialer{
			Timeout:   c.TimeOut,
			KeepAlive: time.Hour,
			DualStack: true,
			TLS:       tlsConfig,
		}
		return dialer, nil
	default:
		return nil, errors.New(c.AuthenticationType + ": Kafka authentication type is not supported")
	}
}

func logf(msg string, a ...interface{}) {
	fmt.Printf(msg, a...)
	fmt.Println()
}
