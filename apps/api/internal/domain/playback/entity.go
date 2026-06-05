package domainplayback

import (
	"strings"
	"time"
)

const (
	PlatformWeb    = "Web"
	NetworkDefault = "DEFAULT"

	DefaultPreloadCount = 3
	DefaultBufferMs     = 1200
	MaxPreloadLimit     = 10

	MaxPlatformLength       = 16
	MaxNetworkTypeLength    = 16
	MaxMediaURLLength       = 512
	MaxCoverURLLength       = 512
	MaxIdempotencyKeyLength = 128
)

// Config 表示端侧播放参数。
type Config struct {
	ID           int64
	Platform     string
	NetworkType  string
	PreloadCount int
	BufferMs     int
	UpdatedAt    time.Time
}

// PreloadVideo 表示客户端可提前加载的视频资源。
type PreloadVideo struct {
	VideoID  int64
	MediaURL string
	CoverURL string
}

// QoSReport 表示一次播放质量上报。
type QoSReport struct {
	ID             int64
	UserID         int64
	VideoID        int64
	FirstFrameMs   *int
	StutterCount   int
	WatchMs        int
	IdempotencyKey string
	CreatedAt      time.Time
}

// RestoreConfig 从持久化记录恢复播放配置。
func RestoreConfig(id int64, platform string, networkType string, preloadCount int, bufferMs int, updatedAt time.Time) *Config {
	return &Config{
		ID:           id,
		Platform:     NormalizePlatform(platform),
		NetworkType:  NormalizeNetworkType(networkType),
		PreloadCount: preloadCount,
		BufferMs:     bufferMs,
		UpdatedAt:    updatedAt,
	}
}

// DefaultConfig 返回兜底播放配置。
func DefaultConfig(platform string, networkType string) *Config {
	now := time.Now().UTC()
	return &Config{
		Platform:     NormalizePlatform(platform),
		NetworkType:  NormalizeNetworkType(networkType),
		PreloadCount: DefaultPreloadCount,
		BufferMs:     DefaultBufferMs,
		UpdatedAt:    now,
	}
}

// RestorePreloadVideo 从视频记录恢复预加载建议。
func RestorePreloadVideo(videoID int64, mediaURL string, coverURL string) *PreloadVideo {
	return &PreloadVideo{
		VideoID:  videoID,
		MediaURL: strings.TrimSpace(mediaURL),
		CoverURL: strings.TrimSpace(coverURL),
	}
}

// NewQoSReport 创建播放质量上报领域对象。
func NewQoSReport(userID int64, videoID int64, firstFrameMs *int, stutterCount int, watchMs int, idempotencyKey string) (*QoSReport, error) {
	if userID < 0 {
		return nil, ErrInvalidUserID
	}
	if videoID <= 0 {
		return nil, ErrInvalidVideoID
	}
	if firstFrameMs != nil && *firstFrameMs < 0 {
		return nil, ErrInvalidFirstFrameMs
	}
	if stutterCount < 0 {
		return nil, ErrInvalidStutterCount
	}
	if watchMs < 0 {
		return nil, ErrInvalidWatchMs
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > MaxIdempotencyKeyLength {
		return nil, ErrIdempotencyKeyTooLong
	}

	return &QoSReport{
		UserID:         userID,
		VideoID:        videoID,
		FirstFrameMs:   firstFrameMs,
		StutterCount:   stutterCount,
		WatchMs:        watchMs,
		IdempotencyKey: idempotencyKey,
	}, nil
}

// RestoreQoSReport 从持久化记录恢复播放质量上报。
func RestoreQoSReport(id int64, userID int64, videoID int64, firstFrameMs *int, stutterCount int, watchMs int, idempotencyKey string, createdAt time.Time) *QoSReport {
	return &QoSReport{
		ID:             id,
		UserID:         userID,
		VideoID:        videoID,
		FirstFrameMs:   firstFrameMs,
		StutterCount:   stutterCount,
		WatchMs:        watchMs,
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
		CreatedAt:      createdAt,
	}
}

// NormalizePlatform 统一端类型，空值按 Web 处理。
func NormalizePlatform(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return PlatformWeb
	}
	return value
}

// NormalizeNetworkType 统一网络类型，空值按默认网络处理。
func NormalizeNetworkType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return NetworkDefault
	}
	switch strings.ToUpper(value) {
	case "WIFI", "WI-FI":
		return "WiFi"
	case "3G":
		return "3G"
	case "4G":
		return "4G"
	case "5G":
		return "5G"
	case NetworkDefault:
		return NetworkDefault
	default:
		return strings.ToUpper(value)
	}
}
