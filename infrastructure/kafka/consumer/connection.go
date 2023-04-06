package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer[T comparable] struct {
	reader    *kafka.Reader
	Brokers   []string
	Topic     string
	GroupId   string
	TimeOut   time.Duration
	DualStack bool
	Offset    int64
}

func (c *Consumer[T]) CreateConnection() {
	dialer := &kafka.Dialer{
		Timeout:   c.TimeOut,
		DualStack: c.DualStack,
	}
	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:  c.Brokers,
		Topic:    c.Topic,
		GroupID:  c.GroupId,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		MaxWait:  time.Millisecond * 50,
		Dialer:   dialer,
	})
	c.reader.SetOffset(c.Offset)
}

// Auto commit offset
func (c *Consumer[T]) Read(model T, callback func(T, error)) {
	for {
		ctx, cancelFunction := context.WithTimeout(context.Background(), time.Millisecond*100)
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
		ctx, cancelFunction := context.WithTimeout(context.Background(), time.Millisecond*100)
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
