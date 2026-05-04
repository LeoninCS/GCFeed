package interfaceshttprelation

import "time"

// followResponse 是关注或取关后的关系状态响应。
type followResponse struct {
	UserID         int64 `json:"user_id"`
	TargetUserID   int64 `json:"target_user_id"`
	Status         int   `json:"status"`
	Following      bool  `json:"following"`
	FollowingCount int   `json:"following_count"`
	FollowerCount  int   `json:"follower_count"`
}

// relationUserResponse 是关注列表和粉丝列表中的用户项。
type relationUserResponse struct {
	UserID     int64     `json:"user_id"`
	Nickname   string    `json:"nickname"`
	AvatarURL  string    `json:"avatar_url"`
	Bio        string    `json:"bio"`
	FollowedAt time.Time `json:"followed_at"`
}

// relationListResponse 是关系列表游标分页响应。
type relationListResponse struct {
	Items      []relationUserResponse `json:"items"`
	NextCursor string                 `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
}
