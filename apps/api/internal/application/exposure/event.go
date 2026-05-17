package applicationexposure

import (
	domainexposure "GCFeed/internal/domain/exposure"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ViewEventRecordedEvent 是观看行为已落库事件，供用户画像和推荐画像 worker 消费。
type ViewEventRecordedEvent struct {
	EventID       string    `json:"event_id"`
	ViewEventID   int64     `json:"view_event_id"`
	UserID        int64     `json:"user_id"`
	VideoID       int64     `json:"video_id"`
	Scene         string    `json:"scene"`
	RequestID     string    `json:"request_id,omitempty"`
	EventType     string    `json:"event_type"`
	WatchMs       int       `json:"watch_ms"`
	Completed     bool      `json:"completed"`
	RecordedAt    time.Time `json:"recorded_at"`
	OccurredAt    time.Time `json:"occurred_at"`
	ExposureCount int       `json:"exposure_count,omitempty"`
}

func NewViewEventRecordedEvent(event *domainexposure.ViewEvent, exposure *domainexposure.Exposure) *ViewEventRecordedEvent {
	if event == nil {
		return nil
	}
	message := &ViewEventRecordedEvent{
		EventID:     newEventID(),
		ViewEventID: event.ID,
		UserID:      event.UserID,
		VideoID:     event.VideoID,
		Scene:       event.Scene,
		RequestID:   event.RequestID,
		EventType:   event.EventType,
		WatchMs:     event.WatchMs,
		Completed:   event.Completed,
		RecordedAt:  event.CreatedAt.UTC(),
		OccurredAt:  time.Now().UTC(),
	}
	if exposure != nil {
		message.ExposureCount = exposure.ExposureCount
	}
	return message
}

func newEventID() string {
	content := make([]byte, 12)
	if _, err := rand.Read(content); err == nil {
		return hex.EncodeToString(content)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
