package consumer

import (
	"context"
	"encoding/json"
	"fmt"
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
		MaxWait:  time.Millisecond * 10,
		Dialer:   dialer,
	})
	c.reader.SetOffset(c.Offset)
}

func (c *Consumer[T]) Read(model T, callback func(T, error)) {
	for {
		ctx, cancelFunction := context.WithTimeout(context.Background(), time.Millisecond*80)
		defer func() {
			fmt.Println("doWorkContext complete")
			c.reader.Close()
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

func (c *Consumer[T]) Close() error {
	return c.reader.Close()
}
