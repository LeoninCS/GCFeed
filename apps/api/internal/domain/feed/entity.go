package domainfeed

import (
	"strings"
	"time"
)

const (
	EventTypeImpression = "IMPRESSION"
	EventTypeView       = "VIEW"
	EventTypeComplete   = "COMPLETE"

	MaxLimit                = 100
	MaxVisitorIDLength      = 128
	MaxIdempotencyKeyLength = 128
	DefaultRefreshScene     = "time"
)

type FeedItem struct {
	VideoID       int64
	AuthorID      int64
	Title         string
	MediaURL      string
	CoverURL      string
	LikeCount     int
	CommentCount  int
	FavoriteCount int
	PublishedAt   time.Time
}

type TimeCursor struct {
	PublishedAt time.Time
	VideoID     int64
}

type ViewEvent struct {
	ID             int64
	UserID         *int64
	VisitorID      string
	VideoID        int64
	EventType      string
	WatchMS        int
	IdempotencyKey string
	CreatedAt      time.Time
}

func NewViewEvent(userID *int64, visitorID string, videoID int64, eventType string, watchMS int, idempotencyKey string) (*ViewEvent, error) {
	visitorID = strings.TrimSpace(visitorID)
	eventType = strings.ToUpper(strings.TrimSpace(eventType))
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	if userID != nil && *userID <= 0 {
		userID = nil
	}
	if len(visitorID) > MaxVisitorIDLength {
		return nil, ErrVisitorIDTooLong
	}
	if videoID <= 0 {
		return nil, ErrInvalidVideoID
	}
	if eventType == "" {
		return nil, ErrEmptyEventType
	}
	if !isValidEventType(eventType) {
		return nil, ErrInvalidEventType
	}
	if watchMS < 0 {
		return nil, ErrInvalidWatchMS
	}
	if len(idempotencyKey) > MaxIdempotencyKeyLength {
		return nil, ErrIdempotencyKeyTooLong
	}

	return &ViewEvent{
		UserID:         userID,
		VisitorID:      visitorID,
		VideoID:        videoID,
		EventType:      eventType,
		WatchMS:        watchMS,
		IdempotencyKey: idempotencyKey,
	}, nil
}

func RestoreViewEvent(id int64, userID *int64, visitorID string, videoID int64, eventType string, watchMS int, idempotencyKey string, createdAt time.Time) *ViewEvent {
	visitorID = strings.TrimSpace(visitorID)
	eventType = strings.ToUpper(strings.TrimSpace(eventType))
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	return &ViewEvent{
		ID:             id,
		UserID:         userID,
		VisitorID:      visitorID,
		VideoID:        videoID,
		EventType:      eventType,
		WatchMS:        watchMS,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      createdAt,
	}
}

func RestoreFeedItem(videoID int64, authorID int64, title string, mediaURL string, coverURL string, likeCount int, commentCount int, favoriteCount int, publishedAt time.Time) *FeedItem {
	return &FeedItem{
		VideoID:       videoID,
		AuthorID:      authorID,
		Title:         strings.TrimSpace(title),
		MediaURL:      strings.TrimSpace(mediaURL),
		CoverURL:      strings.TrimSpace(coverURL),
		LikeCount:     likeCount,
		CommentCount:  commentCount,
		FavoriteCount: favoriteCount,
		PublishedAt:   publishedAt,
	}
}

func isValidEventType(value string) bool {
	switch value {
	case EventTypeImpression, EventTypeView, EventTypeComplete:
		return true
	default:
		return false
	}
}
