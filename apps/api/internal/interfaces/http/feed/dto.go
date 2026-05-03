package interfaceshttpfeed

import "time"

type viewEventRequest struct {
	VisitorID string `json:"visitor_id"`
	VideoID   int64  `json:"video_id"`
	EventType string `json:"event_type"`
	WatchMS   int    `json:"watch_ms"`
}

type timeFeedResponse struct {
	Items      []feedItemResponse `json:"items"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

type feedItemResponse struct {
	VideoID       int64     `json:"video_id"`
	AuthorID      int64     `json:"author_id"`
	Title         string    `json:"title"`
	MediaURL      string    `json:"media_url"`
	CoverURL      string    `json:"cover_url"`
	LikeCount     int       `json:"like_count"`
	CommentCount  int       `json:"comment_count"`
	FavoriteCount int       `json:"favorite_count"`
	PublishedAt   time.Time `json:"published_at"`
}

type viewEventResponse struct {
	ID        int64     `json:"id"`
	VisitorID string    `json:"visitor_id,omitempty"`
	VideoID   int64     `json:"video_id"`
	EventType string    `json:"event_type"`
	WatchMS   int       `json:"watch_ms"`
	CreatedAt time.Time `json:"created_at"`
}
