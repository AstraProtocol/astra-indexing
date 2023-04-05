package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer[T comparable] struct {
	reader  *kafka.Reader
	dialer  *kafka.Dialer
	brokers []string
	topic   string
	groupId string
}

func (c *Consumer[T]) CreateConnection() {
	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:  c.brokers,
		Topic:    c.topic,
		GroupID:  c.groupId,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		MaxWait:  time.Millisecond * 10,
		Dialer:   c.dialer,
	})

	c.reader.SetOffset(0)
}

func (c *Consumer[T]) Read(model T, callback func(T, error)) {
	for {
		ctx, cancelFunction := context.WithTimeout(context.Background(), time.Millisecond*80)
		defer func() {
			fmt.Println("doWorkContext complete")
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
