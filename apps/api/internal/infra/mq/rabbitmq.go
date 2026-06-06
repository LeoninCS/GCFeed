package inframq

import (
	applicationexposure "GCFeed/internal/application/exposure"
	applicationinteraction "GCFeed/internal/application/interaction"
	applicationvideo "GCFeed/internal/application/video"
	infraconfig "GCFeed/internal/infra/config"
	inframetrics "GCFeed/internal/infra/metrics"
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
const defaultVideoExchange = "gcfeed.video"
const defaultVideoPublishedQueue = "gcfeed.video.published"
const defaultVideoEmbeddingQueue = "gcfeed.video.embedding"
const defaultVideoPublishedRouting = "video.published"
const defaultExposureExchange = "gcfeed.exposure"
const defaultViewEventRecordedQueue = "gcfeed.exposure.view_event_recorded"
const defaultViewEventRecordedRouting = "exposure.view_event_recorded"

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

func (r *RabbitMQ) PublishVideoPublished(ctx context.Context, event *applicationvideo.PublishedEvent) error {
	if event == nil {
		return nil
	}
	content, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return r.publishChannel.PublishWithContext(
		ctx,
		r.config.VideoExchange,
		r.config.VideoPublishedRouting,
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

func (r *RabbitMQ) PublishViewEventRecorded(ctx context.Context, event *applicationexposure.ViewEventRecordedEvent) error {
	if event == nil {
		return nil
	}
	content, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return r.publishChannel.PublishWithContext(
		ctx,
		r.config.ExposureExchange,
		r.config.ViewEventRecordedRouting,
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
			start := time.Now()
			var event applicationinteraction.ActionChangedEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				inframetrics.ObserveWorkerJob("mq_action_changed_decode", time.Since(start), err)
				_ = delivery.Nack(false, false)
				continue
			}
			if err := handler(ctx, &event); err != nil {
				inframetrics.ObserveWorkerJob("mq_action_changed_consume", time.Since(start), err)
				_ = delivery.Nack(false, true)
				continue
			}
			inframetrics.ObserveWorkerJob("mq_action_changed_consume", time.Since(start), nil)
			_ = delivery.Ack(false)
		}
	}()
	return nil
}

func (r *RabbitMQ) ConsumeVideoPublished(ctx context.Context, handler func(context.Context, *applicationvideo.PublishedEvent) error) error {
	return r.consumeVideoPublishedQueue(ctx, r.config.VideoPublishedQueue, handler)
}

func (r *RabbitMQ) ConsumeVideoPublishedForEmbedding(ctx context.Context, handler func(context.Context, *applicationvideo.PublishedEvent) error) error {
	return r.consumeVideoPublishedQueue(ctx, r.config.VideoEmbeddingQueue, handler)
}

func (r *RabbitMQ) consumeVideoPublishedQueue(ctx context.Context, queue string, handler func(context.Context, *applicationvideo.PublishedEvent) error) error {
	deliveries, err := r.consumerChannel.ConsumeWithContext(
		ctx,
		queue,
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
			start := time.Now()
			var event applicationvideo.PublishedEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				inframetrics.ObserveWorkerJob("mq_video_published_decode", time.Since(start), err)
				_ = delivery.Nack(false, false)
				continue
			}
			if err := handler(ctx, &event); err != nil {
				inframetrics.ObserveWorkerJob("mq_video_published_consume", time.Since(start), err)
				_ = delivery.Nack(false, true)
				continue
			}
			inframetrics.ObserveWorkerJob("mq_video_published_consume", time.Since(start), nil)
			_ = delivery.Ack(false)
		}
	}()
	return nil
}

func (r *RabbitMQ) ConsumeViewEventRecorded(ctx context.Context, handler func(context.Context, *applicationexposure.ViewEventRecordedEvent) error) error {
	deliveries, err := r.consumerChannel.ConsumeWithContext(
		ctx,
		r.config.ViewEventRecordedQueue,
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
			start := time.Now()
			var event applicationexposure.ViewEventRecordedEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				inframetrics.ObserveWorkerJob("mq_view_event_decode", time.Since(start), err)
				_ = delivery.Nack(false, false)
				continue
			}
			if err := handler(ctx, &event); err != nil {
				inframetrics.ObserveWorkerJob("mq_view_event_consume", time.Since(start), err)
				_ = delivery.Nack(false, true)
				continue
			}
			inframetrics.ObserveWorkerJob("mq_view_event_consume", time.Since(start), nil)
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
	if err := r.publishChannel.QueueBind(
		r.config.ActionChangedQueue,
		r.config.ActionChangedRouting,
		r.config.InteractionExchange,
		false,
		nil,
	); err != nil {
		return err
	}
	if err := r.publishChannel.ExchangeDeclare(
		r.config.VideoExchange,
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
		r.config.VideoPublishedQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}
	if err := r.publishChannel.QueueBind(
		r.config.VideoPublishedQueue,
		r.config.VideoPublishedRouting,
		r.config.VideoExchange,
		false,
		nil,
	); err != nil {
		return err
	}
	if _, err := r.publishChannel.QueueDeclare(
		r.config.VideoEmbeddingQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}
	if err := r.publishChannel.QueueBind(
		r.config.VideoEmbeddingQueue,
		r.config.VideoPublishedRouting,
		r.config.VideoExchange,
		false,
		nil,
	); err != nil {
		return err
	}
	if err := r.publishChannel.ExchangeDeclare(
		r.config.ExposureExchange,
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
		r.config.ViewEventRecordedQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}
	return r.publishChannel.QueueBind(
		r.config.ViewEventRecordedQueue,
		r.config.ViewEventRecordedRouting,
		r.config.ExposureExchange,
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
	if cfg.VideoExchange == "" {
		cfg.VideoExchange = defaultVideoExchange
	}
	if cfg.VideoPublishedQueue == "" {
		cfg.VideoPublishedQueue = defaultVideoPublishedQueue
	}
	cfg.VideoEmbeddingQueue = strings.TrimSpace(cfg.VideoEmbeddingQueue)
	if cfg.VideoEmbeddingQueue == "" {
		cfg.VideoEmbeddingQueue = defaultVideoEmbeddingQueue
	}
	if cfg.VideoPublishedRouting == "" {
		cfg.VideoPublishedRouting = defaultVideoPublishedRouting
	}
	cfg.ExposureExchange = strings.TrimSpace(cfg.ExposureExchange)
	cfg.ViewEventRecordedQueue = strings.TrimSpace(cfg.ViewEventRecordedQueue)
	cfg.ViewEventRecordedRouting = strings.TrimSpace(cfg.ViewEventRecordedRouting)
	if cfg.ExposureExchange == "" {
		cfg.ExposureExchange = defaultExposureExchange
	}
	if cfg.ViewEventRecordedQueue == "" {
		cfg.ViewEventRecordedQueue = defaultViewEventRecordedQueue
	}
	if cfg.ViewEventRecordedRouting == "" {
		cfg.ViewEventRecordedRouting = defaultViewEventRecordedRouting
	}
	return cfg
}
