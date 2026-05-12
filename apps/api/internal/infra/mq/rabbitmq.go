package inframq

import (
	applicationinteraction "GCFeed/internal/application/interaction"
	infraconfig "GCFeed/internal/infra/config"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultInteractionExchange = "gcfeed.interaction"
const defaultActionChangedQueue = "gcfeed.interaction.action_changed"
const defaultActionChangedRouting = "interaction.action_changed"

var ErrEmptyRabbitMQURL = errors.New("rabbitmq url is empty")

type RabbitMQ struct {
	conn            *amqp.Connection
	publishChannel  *amqp.Channel
	consumerChannel *amqp.Channel
	config          infraconfig.RabbitMQConfig
}

func NewRabbitMQ(cfg infraconfig.RabbitMQConfig) (*RabbitMQ, error) {
	cfg = normalizeRabbitMQConfig(cfg)
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, ErrEmptyRabbitMQURL
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, err
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	consumerChannel, err := conn.Channel()
	if err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, err
	}

	client := &RabbitMQ{
		conn:            conn,
		publishChannel:  channel,
		consumerChannel: consumerChannel,
		config:          cfg,
	}
	if err := client.ensureTopology(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (r *RabbitMQ) Close() error {
	if r == nil {
		return nil
	}
	if r.publishChannel != nil {
		_ = r.publishChannel.Close()
	}
	if r.consumerChannel != nil {
		_ = r.consumerChannel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *RabbitMQ) PublishActionChanged(ctx context.Context, event *applicationinteraction.ActionChangedEvent) error {
	if event == nil {
		return nil
	}
	content, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return r.publishChannel.PublishWithContext(
		ctx,
		r.config.InteractionExchange,
		r.config.ActionChangedRouting,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    event.EventID,
			Timestamp:    time.Now(),
			Body:         content,
		},
	)
}

func (r *RabbitMQ) ConsumeActionChanged(ctx context.Context, handler func(context.Context, *applicationinteraction.ActionChangedEvent) error) error {
	deliveries, err := r.consumerChannel.ConsumeWithContext(
		ctx,
		r.config.ActionChangedQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for delivery := range deliveries {
			var event applicationinteraction.ActionChangedEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				_ = delivery.Nack(false, false)
				continue
			}
			if err := handler(ctx, &event); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}
			_ = delivery.Ack(false)
		}
	}()
	return nil
}

func (r *RabbitMQ) ensureTopology() error {
	if err := r.publishChannel.ExchangeDeclare(
		r.config.InteractionExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}
	if _, err := r.publishChannel.QueueDeclare(
		r.config.ActionChangedQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}
	return r.publishChannel.QueueBind(
		r.config.ActionChangedQueue,
		r.config.ActionChangedRouting,
		r.config.InteractionExchange,
		false,
		nil,
	)
}

func normalizeRabbitMQConfig(cfg infraconfig.RabbitMQConfig) infraconfig.RabbitMQConfig {
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.InteractionExchange = strings.TrimSpace(cfg.InteractionExchange)
	cfg.ActionChangedQueue = strings.TrimSpace(cfg.ActionChangedQueue)
	cfg.ActionChangedRouting = strings.TrimSpace(cfg.ActionChangedRouting)
	if cfg.InteractionExchange == "" {
		cfg.InteractionExchange = defaultInteractionExchange
	}
	if cfg.ActionChangedQueue == "" {
		cfg.ActionChangedQueue = defaultActionChangedQueue
	}
	if cfg.ActionChangedRouting == "" {
		cfg.ActionChangedRouting = defaultActionChangedRouting
	}
	return cfg
}
