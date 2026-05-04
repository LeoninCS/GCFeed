package interfaceshttpaccount

import (
	applicationaccount "GCFeed/internal/application/account"
	domainaccount "GCFeed/internal/domain/account"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *applicationaccount.Service
}

// New 注入账号应用服务，Handler 只处理 HTTP 输入输出。
func New(service *applicationaccount.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Register 处理用户注册请求，成功后返回新用户资料。
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 具体注册规则在应用层和领域层执行，HTTP 层只传递请求字段。
	profile, err := h.service.Register(c.Request.Context(), req.Account, req.Password, req.Nickname)
	if err != nil {
		if isBadRequestError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domainaccount.ErrAccountAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "account already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, profileResponse(profile))
}

// Login 处理账号密码登录，成功后返回 Bearer token。
func (h *Handler) Login(c *gin.Context) {
	var req LoginByPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 登录失败统一映射为 401，避免暴露账号是否存在。
	token, err := h.service.Login(c.Request.Context(), req.Account, req.Password)
	if err != nil {
		if isBadRequestError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, domainaccount.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, tokenResponse{
		AccessToken:      token.AccessToken,
		TokenType:        token.TokenType,
		ExpiresInSeconds: token.ExpiresInSeconds,
	})
}

// Logout 当前项目使用无状态 JWT，服务端无需清理会话数据。
func (h *Handler) Logout(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Me 读取当前登录用户资料，用户 ID 来自 JWT 中间件写入的上下文。
func (h *Handler) Me(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	profile, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		writeProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, profileResponse(profile))
}

// UpdateMe 更新当前登录用户资料，请求体支持部分字段更新。
func (h *Handler) UpdateMe(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	profile, err := h.service.UpdateProfile(c.Request.Context(), userID, req.Nickname, req.AvatarURL, req.Bio)
	if err != nil {
		writeProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, profileResponse(profile))
}

// profileResponse 将应用层 Profile 转成对外 JSON 结构。
func profileResponse(profile *applicationaccount.Profile) userProfileResponse {
	return userProfileResponse{
		ID:        profile.ID,
		Account:   profile.Account,
		Nickname:  profile.Nickname,
		AvatarURL: profile.AvatarURL,
		Bio:       profile.Bio,
		Status:    profile.Status,
		Role:      profile.Role,
	}
}

// userIDFromContext 从 JWT 中间件写入的上下文中读取登录用户 ID。
func userIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
}

// writeProfileError 统一账号资料相关接口的错误响应。
func writeProfileError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, domainaccount.ErrUserNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// isBadRequestError 判断哪些领域错误属于客户端请求参数问题。
func isBadRequestError(err error) bool {
	return errors.Is(err, domainaccount.ErrEmptyAccount) ||
		errors.Is(err, domainaccount.ErrEmptyPassword) ||
		errors.Is(err, domainaccount.ErrEmptyNickname) ||
		errors.Is(err, domainaccount.ErrInvalidUserID) ||
		errors.Is(err, domainaccount.ErrEmptyProfileUpdate)
}
