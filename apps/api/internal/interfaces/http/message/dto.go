package interfaceshttpmessage

import "time"

type createMessageRequest struct {
	UserID         int64  `json:"user_id"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	EventID        string `json:"event_id"`
	ActorID        int64  `json:"actor_id"`
	ActorNickname  string `json:"actor_nickname"`
	ActorAvatarURL string `json:"actor_avatar_url"`
}

type markReadRequest struct {
	MessageIDs []int64 `json:"message_ids"`
}

type messageResponse struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Type           string     `json:"type"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	EventID        string     `json:"event_id,omitempty"`
	ActorID        int64      `json:"actor_id,omitempty"`
	ActorNickname  string     `json:"actor_nickname,omitempty"`
	ActorAvatarURL string     `json:"actor_avatar_url,omitempty"`
	IsRead         bool       `json:"is_read"`
	CreatedAt      time.Time  `json:"created_at"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
}

type messageListResponse struct {
	Items      []messageResponse `json:"items"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

type unreadStatResponse struct {
	UnreadCount int `json:"unread_count"`
}

type markReadResponse struct {
	UpdatedCount int `json:"updated_count"`
}
