package interfaceshttpplayback

import "time"

type playbackConfigResponse struct {
	ID           int64     `json:"id"`
	Platform     string    `json:"platform"`
	NetworkType  string    `json:"network_type"`
	PreloadCount int       `json:"preload_count"`
	BufferMs     int       `json:"buffer_ms"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type preloadVideoResponse struct {
	VideoID  int64  `json:"video_id"`
	MediaURL string `json:"media_url"`
	CoverURL string `json:"cover_url"`
}

type preloadVideosResponse struct {
	Items []preloadVideoResponse `json:"items"`
}

type createQoSReportRequest struct {
	UserID       int64 `json:"user_id,omitempty"`
	VideoID      int64 `json:"video_id"`
	FirstFrameMs *int  `json:"first_frame_ms,omitempty"`
	StutterCount int   `json:"stutter_count"`
	WatchMs      int   `json:"watch_ms"`
}

type qosReportResponse struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	VideoID      int64     `json:"video_id"`
	FirstFrameMs *int      `json:"first_frame_ms,omitempty"`
	StutterCount int       `json:"stutter_count"`
	WatchMs      int       `json:"watch_ms"`
	CreatedAt    time.Time `json:"created_at"`
}
