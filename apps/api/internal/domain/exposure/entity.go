package domainexposure

import (
	"strings"
	"time"
)

const (
	EventTypeExposed  = "exposed"
	EventTypePlay     = "play"
	EventTypeComplete = "complete"
	EventTypeSkip     = "skip"

	MaxSceneLength     = 32
	MaxRequestIDLength = 64
)

// ViewEvent 保存一次客户端观看行为，适合做行为流水和后续推荐特征。
type ViewEvent struct {
	ID        int64
	UserID    int64
	VideoID   int64
	Scene     string
	RequestID string
	EventType string
	WatchMs   int
	Completed bool
	CreatedAt time.Time
}

// Exposure 保存用户看过某个视频的聚合事实，供推荐系统在线去重查询。
type Exposure struct {
	ID             int64
	UserID         int64
	VideoID        int64
	FirstExposedAt time.Time
	LastExposedAt  time.Time
	ExposureCount  int
	LastScene      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewViewEvent 创建观看事件并完成基础参数清洗。
func NewViewEvent(userID int64, videoID int64, scene string, requestID string, eventType string, watchMs int, completed bool) (*ViewEvent, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	if videoID <= 0 {
		return nil, ErrInvalidVideoID
	}

	scene = strings.TrimSpace(strings.ToLower(scene))
	requestID = strings.TrimSpace(requestID)
	eventType = strings.TrimSpace(strings.ToLower(eventType))
	if scene == "" {
		return nil, ErrEmptyScene
	}
	if len(scene) > MaxSceneLength {
		return nil, ErrSceneTooLong
	}
	if len(requestID) > MaxRequestIDLength {
		return nil, ErrRequestIDTooLong
	}
	if watchMs < 0 {
		return nil, ErrWatchMsNegative
	}
	if !isSupportedEventType(eventType) {
		return nil, ErrInvalidEventType
	}

	return &ViewEvent{
		UserID:    userID,
		VideoID:   videoID,
		Scene:     scene,
		RequestID: requestID,
		EventType: eventType,
		WatchMs:   watchMs,
		Completed: completed || eventType == EventTypeComplete,
	}, nil
}

// RestoreViewEvent 从数据库恢复观看事件。
func RestoreViewEvent(id int64, userID int64, videoID int64, scene string, requestID string, eventType string, watchMs int, completed bool, createdAt time.Time) *ViewEvent {
	return &ViewEvent{
		ID:        id,
		UserID:    userID,
		VideoID:   videoID,
		Scene:     strings.TrimSpace(scene),
		RequestID: strings.TrimSpace(requestID),
		EventType: strings.TrimSpace(eventType),
		WatchMs:   watchMs,
		Completed: completed,
		CreatedAt: createdAt,
	}
}

// RestoreExposure 从数据库恢复曝光聚合事实。
func RestoreExposure(id int64, userID int64, videoID int64, firstExposedAt time.Time, lastExposedAt time.Time, exposureCount int, lastScene string, createdAt time.Time, updatedAt time.Time) *Exposure {
	return &Exposure{
		ID:             id,
		UserID:         userID,
		VideoID:        videoID,
		FirstExposedAt: firstExposedAt,
		LastExposedAt:  lastExposedAt,
		ExposureCount:  exposureCount,
		LastScene:      strings.TrimSpace(lastScene),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

// CountsAsExposure 判断当前事件是否写入曝光聚合索引。
func (e *ViewEvent) CountsAsExposure() bool {
	return e != nil && e.EventType == EventTypeExposed
}

func isSupportedEventType(eventType string) bool {
	switch eventType {
	case EventTypeExposed, EventTypePlay, EventTypeComplete, EventTypeSkip:
		return true
	default:
		return false
	}
}
