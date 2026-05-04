package interfaceshttpinteraction

import "time"

// createCommentRequest 是创建评论的 JSON 请求体。
type createCommentRequest struct {
	Content string `json:"content"`
}

// actionResponse 是点赞/收藏状态变更后的响应。
type actionResponse struct {
	VideoID       int64  `json:"video_id"`
	ActionType    string `json:"action_type"`
	Active        bool   `json:"active"`
	LikeCount     int    `json:"like_count"`
	FavoriteCount int    `json:"favorite_count"`
}

// commentResponse 是评论详情响应，创建评论时会额外返回 CommentCount。
type commentResponse struct {
	ID            int64     `json:"id"`
	VideoID       int64     `json:"video_id"`
	UserID        int64     `json:"user_id"`
	UserNickname  string    `json:"user_nickname"`
	UserAvatarURL string    `json:"user_avatar_url"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
	CommentCount  int       `json:"comment_count,omitempty"`
}

// commentListResponse 是评论游标分页响应。
type commentListResponse struct {
	Items      []commentResponse `json:"items"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

// deleteCommentResponse 是删除评论后的状态响应。
type deleteCommentResponse struct {
	CommentID    int64 `json:"comment_id"`
	Status       int   `json:"status"`
	CommentCount int   `json:"comment_count"`
}
