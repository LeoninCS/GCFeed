package interfaceshttpaccount

// 账号注册请求
type RegisterRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

// 账号登录请求
type LoginByPasswordRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// 用户资料更新请求
type UpdateProfileRequest struct {
	Nickname  *string `json:"nickname"`
	AvatarURL *string `json:"avatar_url"`
	Bio       *string `json:"bio"`
}

// 账号登录响应
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

// 用户信息响应
type userProfileResponse struct {
	ID             int64  `json:"id"`
	Account        string `json:"account"`
	Nickname       string `json:"nickname"`
	AvatarURL      string `json:"avatar_url"`
	Bio            string `json:"bio"`
	Status         int    `json:"status"`
	Role           string `json:"role"`
	FollowingCount int    `json:"following_count"`
	FollowerCount  int    `json:"follower_count"`
	WorkCount      int    `json:"work_count"`
}

// 公开用户信息响应，隐藏账号、角色和状态等内部字段。
type publicUserProfileResponse struct {
	ID             int64  `json:"id"`
	Nickname       string `json:"nickname"`
	AvatarURL      string `json:"avatar_url"`
	Bio            string `json:"bio"`
	FollowingCount int    `json:"following_count"`
	FollowerCount  int    `json:"follower_count"`
	WorkCount      int    `json:"work_count"`
}
