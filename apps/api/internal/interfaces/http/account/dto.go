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

// 账号登录响应
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

// 用户信息响应
type userProfileResponse struct {
	ID       int64  `json:"id"`
	Account  string `json:"account"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
}
