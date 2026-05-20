package interfaceshttprecommendation

import "time"

type candidateRequest struct {
	UserID    int64  `json:"user_id"`
	Scene     string `json:"scene"`
	RequestID string `json:"request_id"`
	Cursor    string `json:"cursor"`
	Limit     *int   `json:"limit"`
}

type candidateResponse struct {
	UserID     int64                   `json:"user_id"`
	Scene      string                  `json:"scene"`
	RequestID  string                  `json:"request_id,omitempty"`
	Candidates []candidateItemResponse `json:"candidates"`
	NextCursor string                  `json:"next_cursor"`
	HasMore    bool                    `json:"has_more"`
}

type candidateItemResponse struct {
	VideoID        int64     `json:"video_id"`
	AuthorID       int64     `json:"author_id"`
	RankScore      float64   `json:"rank_score"`
	Similarity     float64   `json:"similarity"`
	HotScore       int       `json:"hot_score"`
	FreshnessScore float64   `json:"freshness_score"`
	Reason         string    `json:"reason"`
	PublishedAt    time.Time `json:"published_at"`
}

type exposuresRequest struct {
	UserID    int64   `json:"user_id"`
	Scene     string  `json:"scene"`
	RequestID string  `json:"request_id"`
	VideoIDs  []int64 `json:"video_ids"`
}

type exposureDecisionsRequest struct {
	UserID    int64   `json:"user_id"`
	Scene     string  `json:"scene"`
	RequestID string  `json:"request_id"`
	VideoIDs  []int64 `json:"video_ids"`
}

type exposureDecisionsResponse struct {
	UserID    int64                          `json:"user_id"`
	Scene     string                         `json:"scene"`
	RequestID string                         `json:"request_id,omitempty"`
	Decisions []exposureDecisionItemResponse `json:"decisions"`
}

type exposureDecisionItemResponse struct {
	VideoID       int64      `json:"video_id"`
	Allowed       bool       `json:"allowed"`
	Reason        string     `json:"reason"`
	LastExposedAt *time.Time `json:"last_exposed_at,omitempty"`
}

type exposuresResponse struct {
	Exposures []exposureItemResponse `json:"exposures"`
}

type exposureItemResponse struct {
	UserID         int64     `json:"user_id"`
	VideoID        int64     `json:"video_id"`
	FirstExposedAt time.Time `json:"first_exposed_at"`
	LastExposedAt  time.Time `json:"last_exposed_at"`
	ExposureCount  int       `json:"exposure_count"`
	LastScene      string    `json:"last_scene"`
}
