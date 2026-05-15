package applicationvideo

import (
	domainvideo "GCFeed/internal/domain/video"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type PublishedEvent struct {
	EventID     string    `json:"event_id"`
	VideoID     int64     `json:"video_id"`
	AuthorID    int64     `json:"author_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	MediaURL    string    `json:"media_url"`
	CoverURL    string    `json:"cover_url"`
	PublishedAt time.Time `json:"published_at"`
	OccurredAt  time.Time `json:"occurred_at"`
}

func NewPublishedEvent(video *domainvideo.Video) *PublishedEvent {
	if video == nil || video.PublishedAt == nil {
		return nil
	}
	return &PublishedEvent{
		EventID:     newEventID(),
		VideoID:     video.ID,
		AuthorID:    video.AuthorID,
		Title:       strings.TrimSpace(video.Title),
		Description: strings.TrimSpace(video.Description),
		MediaURL:    strings.TrimSpace(video.MediaURL),
		CoverURL:    strings.TrimSpace(video.CoverURL),
		PublishedAt: video.PublishedAt.UTC(),
		OccurredAt:  time.Now().UTC(),
	}
}

func newEventID() string {
	content := make([]byte, 12)
	if _, err := rand.Read(content); err == nil {
		return hex.EncodeToString(content)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
