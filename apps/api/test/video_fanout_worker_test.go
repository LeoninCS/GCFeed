package test

import (
	"context"
	"testing"
	"time"

	applicationvideo "GCFeed/internal/application/video"
	domainfeed "GCFeed/internal/domain/feed"
)

type memoryFanoutRepo struct {
	followerCount int
	followerIDs   []int64
}

func (r *memoryFanoutRepo) CountFollowers(ctx context.Context, authorID int64) (int, error) {
	return r.followerCount, nil
}

func (r *memoryFanoutRepo) ListFollowerIDs(ctx context.Context, authorID int64, cursor int64, limit int) ([]int64, error) {
	items := make([]int64, 0, limit)
	for _, followerID := range r.followerIDs {
		if followerID <= cursor {
			continue
		}
		items = append(items, followerID)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *memoryFanoutRepo) BatchGetFeedCards(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedCard, error) {
	cards := map[int64]*domainfeed.FeedCard{}
	for _, videoID := range videoIDs {
		cards[videoID] = &domainfeed.FeedCard{VideoID: videoID, AuthorID: 7, Title: "published"}
	}
	return cards, nil
}

func (r *memoryFanoutRepo) BatchGetFeedStats(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedStat, error) {
	stats := map[int64]*domainfeed.FeedStat{}
	for _, videoID := range videoIDs {
		stats[videoID] = &domainfeed.FeedStat{VideoID: videoID}
	}
	return stats, nil
}

type memoryFollowingIndex struct {
	inboxUsers    []int64
	inboxVideoID  int64
	outboxAuthor  int64
	outboxVideoID int64
}

func (i *memoryFollowingIndex) AddInboxItems(ctx context.Context, authorID int64, userIDs []int64, item *domainfeed.FeedPageItem, maxLen int64) error {
	i.inboxUsers = append(i.inboxUsers, userIDs...)
	i.inboxVideoID = item.VideoID
	return nil
}

func (i *memoryFollowingIndex) AddAuthorOutboxItem(ctx context.Context, authorID int64, item *domainfeed.FeedPageItem, maxLen int64) error {
	i.outboxAuthor = authorID
	i.outboxVideoID = item.VideoID
	return nil
}

type memoryFeedPreheater struct {
	videoID int64
}

func (p *memoryFeedPreheater) PreheatFeedVideo(ctx context.Context, videoID int64, ttl time.Duration) error {
	p.videoID = videoID
	return nil
}

func TestFanoutWorkerPushesSmallCreatorInbox(t *testing.T) {
	repo := &memoryFanoutRepo{
		followerCount: 3,
		followerIDs:   []int64{10, 11, 12},
	}
	index := &memoryFollowingIndex{}
	preheater := &memoryFeedPreheater{}
	worker := applicationvideo.NewFanoutWorker(repo, nil, index, preheater, applicationvideo.WithFanoutBatchSize(2))

	event := &applicationvideo.PublishedEvent{
		VideoID:     99,
		AuthorID:    7,
		Title:       "published",
		PublishedAt: time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := worker.HandleVideoPublished(context.Background(), event); err != nil {
		t.Fatalf("handle video published: %v", err)
	}
	if len(index.inboxUsers) != 3 || index.inboxUsers[0] != 10 || index.inboxUsers[2] != 12 || index.inboxVideoID != 99 {
		t.Fatalf("unexpected inbox fanout: %+v", index)
	}
	if index.outboxVideoID != 0 {
		t.Fatalf("unexpected outbox fanout: %+v", index)
	}
	if preheater.videoID != 99 {
		t.Fatalf("unexpected preheat video id: %d", preheater.videoID)
	}
}

func TestFanoutWorkerWritesBigCreatorOutbox(t *testing.T) {
	repo := &memoryFanoutRepo{
		followerCount: domainfeed.BigCreatorFollowerThreshold,
		followerIDs:   []int64{10, 11, 12},
	}
	index := &memoryFollowingIndex{}
	worker := applicationvideo.NewFanoutWorker(repo, nil, index, nil)

	event := &applicationvideo.PublishedEvent{
		VideoID:     100,
		AuthorID:    8,
		PublishedAt: time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := worker.HandleVideoPublished(context.Background(), event); err != nil {
		t.Fatalf("handle video published: %v", err)
	}
	if len(index.inboxUsers) != 0 {
		t.Fatalf("unexpected inbox fanout: %+v", index)
	}
	if index.outboxAuthor != 8 || index.outboxVideoID != 100 {
		t.Fatalf("unexpected outbox fanout: %+v", index)
	}
}
