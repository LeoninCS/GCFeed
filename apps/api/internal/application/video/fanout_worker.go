package applicationvideo

import (
	domainfeed "GCFeed/internal/domain/feed"
	"context"
	"time"
)

const defaultFollowerBatchSize = 500

type PublishedEventConsumer interface {
	ConsumeVideoPublished(ctx context.Context, handler func(context.Context, *PublishedEvent) error) error
}

type FanoutRepository interface {
	CountFollowers(ctx context.Context, authorID int64) (int, error)
	ListFollowerIDs(ctx context.Context, authorID int64, cursor int64, limit int) ([]int64, error)
	BatchGetFeedCards(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedCard, error)
	BatchGetFeedStats(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedStat, error)
}

type FollowingIndexCache interface {
	AddInboxItems(ctx context.Context, authorID int64, userIDs []int64, item *domainfeed.FeedPageItem, maxLen int64) error
	AddAuthorOutboxItem(ctx context.Context, authorID int64, item *domainfeed.FeedPageItem, maxLen int64) error
}

type FeedPreheater interface {
	PreheatFeedVideo(ctx context.Context, videoID int64, ttl time.Duration) error
}

type FeedCacheWriter interface {
	SetCards(ctx context.Context, cards map[int64]*domainfeed.FeedCard, ttl time.Duration) error
	SetStats(ctx context.Context, stats map[int64]*domainfeed.FeedStat, ttl time.Duration) error
}

type FanoutWorker struct {
	repo         FanoutRepository
	consumer     PublishedEventConsumer
	index        FollowingIndexCache
	preheater    FeedPreheater
	batchSize    int
	inboxMaxLen  int64
	outboxMaxLen int64
	preheatTTL   time.Duration
}

type FanoutWorkerOption func(*FanoutWorker)

func NewFanoutWorker(repo FanoutRepository, consumer PublishedEventConsumer, index FollowingIndexCache, preheater FeedPreheater, options ...FanoutWorkerOption) *FanoutWorker {
	worker := &FanoutWorker{
		repo:         repo,
		consumer:     consumer,
		index:        index,
		preheater:    preheater,
		batchSize:    defaultFollowerBatchSize,
		inboxMaxLen:  1000,
		outboxMaxLen: 500,
		preheatTTL:   15 * time.Minute,
	}
	for _, option := range options {
		option(worker)
	}
	return worker
}

type FeedPreheaterAdapter struct {
	source FanoutRepository
	cache  FeedCacheWriter
}

func NewFeedPreheater(source FanoutRepository, cache FeedCacheWriter) *FeedPreheaterAdapter {
	return &FeedPreheaterAdapter{source: source, cache: cache}
}

func (p *FeedPreheaterAdapter) PreheatFeedVideo(ctx context.Context, videoID int64, ttl time.Duration) error {
	if p == nil || p.source == nil || p.cache == nil || videoID <= 0 {
		return nil
	}
	cards, err := p.source.BatchGetFeedCards(ctx, []int64{videoID})
	if err != nil {
		return err
	}
	if len(cards) > 0 {
		if err := p.cache.SetCards(ctx, cards, ttl); err != nil {
			return err
		}
	}
	stats, err := p.source.BatchGetFeedStats(ctx, []int64{videoID})
	if err != nil {
		return err
	}
	if len(stats) > 0 {
		return p.cache.SetStats(ctx, stats, 15*time.Second)
	}
	return nil
}

func WithFanoutBatchSize(size int) FanoutWorkerOption {
	return func(w *FanoutWorker) {
		if size > 0 {
			w.batchSize = size
		}
	}
}

func (w *FanoutWorker) Start(ctx context.Context) error {
	if w == nil || w.consumer == nil {
		return nil
	}
	return w.consumer.ConsumeVideoPublished(ctx, w.HandleVideoPublished)
}

func (w *FanoutWorker) HandleVideoPublished(ctx context.Context, event *PublishedEvent) error {
	if event == nil || event.VideoID <= 0 || event.AuthorID <= 0 {
		return nil
	}
	if err := w.preheat(ctx, event); err != nil {
		return err
	}
	if w.repo == nil || w.index == nil {
		return nil
	}

	item := &domainfeed.FeedPageItem{
		VideoID:     event.VideoID,
		AuthorID:    event.AuthorID,
		PublishedAt: event.PublishedAt,
	}

	followerCount, err := w.repo.CountFollowers(ctx, event.AuthorID)
	if err != nil {
		return err
	}
	if followerCount >= domainfeed.BigCreatorFollowerThreshold {
		return w.index.AddAuthorOutboxItem(ctx, event.AuthorID, item, w.outboxMaxLen)
	}

	cursor := int64(0)
	for {
		followerIDs, err := w.repo.ListFollowerIDs(ctx, event.AuthorID, cursor, w.batchSize)
		if err != nil {
			return err
		}
		if len(followerIDs) == 0 {
			return nil
		}
		if err := w.index.AddInboxItems(ctx, event.AuthorID, followerIDs, item, w.inboxMaxLen); err != nil {
			return err
		}
		cursor = followerIDs[len(followerIDs)-1]
		if len(followerIDs) < w.batchSize {
			return nil
		}
	}
}

func (w *FanoutWorker) preheat(ctx context.Context, event *PublishedEvent) error {
	if w.preheater == nil {
		return nil
	}
	return w.preheater.PreheatFeedVideo(ctx, event.VideoID, w.preheatTTL)
}
