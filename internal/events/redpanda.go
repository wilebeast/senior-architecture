package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type RedpandaBus struct {
	writers map[string]*kafka.Writer
}

func NewRedpandaBus(brokers []string, topics []string) (*RedpandaBus, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no redpanda brokers configured")
	}
	if err := ensureTopics(brokers, topics); err != nil {
		return nil, fmt.Errorf("ensure topics: %w", err)
	}

	writers := make(map[string]*kafka.Writer, len(topics))
	for _, topic := range topics {
		writers[topic] = &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			RequiredAcks: kafka.RequireOne,
			BatchTimeout: 10 * time.Millisecond,
			Balancer:     &kafka.LeastBytes{},
		}
	}

	return &RedpandaBus{writers: writers}, nil
}

func ensureTopics(brokers []string, topics []string) error {
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}

	controllerConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return err
	}
	defer controllerConn.Close()

	configs := make([]kafka.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		configs = append(configs, kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}

	return controllerConn.CreateTopics(configs...)
}

func (b *RedpandaBus) Publish(ctx context.Context, topic, key string, payload any) error {
	writer, ok := b.writers[topic]
	if !ok {
		return fmt.Errorf("topic %s not configured", topic)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: data,
		Time:  time.Now().UTC(),
	})
}

func (b *RedpandaBus) Close() error {
	var firstErr error
	for _, writer := range b.writers {
		if err := writer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
