package interfaceshttpvideo

import "time"

type CreateVideoRequest struct {
	Title    string `json:"title"`
	MediaURL string `json:"media_url"`
	CoverURL string `json:"cover_url"`
}

type videoResponse struct {
	ID            int64      `json:"id"`
	AuthorID      int64      `json:"author_id"`
	Title         string     `json:"title"`
	MediaURL      string     `json:"media_url"`
	CoverURL      string     `json:"cover_url"`
	Status        int        `json:"status"`
	LikeCount     int        `json:"like_count"`
	CommentCount  int        `json:"comment_count"`
	FavoriteCount int        `json:"favorite_count"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type videoListResponse struct {
	Items  []videoResponse `json:"items"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}
