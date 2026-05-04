package test

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	applicationrelation "GCFeed/internal/application/relation"
	domainrelation "GCFeed/internal/domain/relation"
	infrajwt "GCFeed/internal/infra/jwt"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	interfaceshttprelation "GCFeed/internal/interfaces/http/relation"

	"github.com/gin-gonic/gin"
)

type relationFollowAPIResponse struct {
	UserID         int64 `json:"user_id"`
	TargetUserID   int64 `json:"target_user_id"`
	Status         int   `json:"status"`
	Following      bool  `json:"following"`
	FollowingCount int   `json:"following_count"`
	FollowerCount  int   `json:"follower_count"`
}

type relationListAPIResponse struct {
	Items      []relationUserAPIResponse `json:"items"`
	NextCursor string                    `json:"next_cursor"`
	HasMore    bool                      `json:"has_more"`
}

type relationUserAPIResponse struct {
	UserID     int64     `json:"user_id"`
	Nickname   string    `json:"nickname"`
	AvatarURL  string    `json:"avatar_url"`
	Bio        string    `json:"bio"`
	FollowedAt time.Time `json:"followed_at"`
}

type memoryRelationUser struct {
	ID        int64
	Nickname  string
	AvatarURL string
	Bio       string
	Active    bool
}

// memoryRelationRepo 是关系测试用内存仓储，模拟关注状态、计数和分页。
type memoryRelationRepo struct {
	mu       sync.Mutex
	nextID   int64
	users    map[int64]memoryRelationUser
	follows  map[string]*domainrelation.Follow
	stats    map[int64]*domainrelation.RelationStat
	clockSeq int64
}

func newMemoryRelationRepo() *memoryRelationRepo {
	return &memoryRelationRepo{
		nextID: 1,
		users: map[int64]memoryRelationUser{
			42: {ID: 42, Nickname: "viewer", AvatarURL: "https://example.com/42.jpg", Bio: "viewer bio", Active: true},
			77: {ID: 77, Nickname: "creator", AvatarURL: "https://example.com/77.jpg", Bio: "creator bio", Active: true},
			88: {ID: 88, Nickname: "maker", AvatarURL: "https://example.com/88.jpg", Bio: "maker bio", Active: true},
			99: {ID: 99, Nickname: "guest", AvatarURL: "https://example.com/99.jpg", Bio: "guest bio", Active: true},
		},
		follows: map[string]*domainrelation.Follow{},
		stats:   map[int64]*domainrelation.RelationStat{},
	}
}

// SetFollow 模拟关注/取关事务，并维护双方统计。
func (r *memoryRelationRepo) SetFollow(ctx context.Context, userID int64, targetUserID int64, active bool, idempotencyKey string) (*domainrelation.Follow, *domainrelation.RelationStat, *domainrelation.RelationStat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.userActive(userID) || !r.userActive(targetUserID) {
		return nil, nil, nil, domainrelation.ErrTargetUserNotFound
	}

	key := memoryRelationKey(userID, targetUserID)
	follow, exists := r.follows[key]
	if exists && idempotencyKey != "" && follow.IdempotencyKey == strings.TrimSpace(idempotencyKey) {
		return cloneFollow(follow), cloneStat(r.ensureStat(userID)), cloneStat(r.ensureStat(targetUserID)), nil
	}

	nextStatus := domainrelation.FollowStatusCanceled
	if active {
		nextStatus = domainrelation.FollowStatusActive
	}

	delta := 0
	now := r.nextTime()
	if !exists {
		follow = &domainrelation.Follow{
			ID:             r.nextID,
			UserID:         userID,
			TargetUserID:   targetUserID,
			Status:         nextStatus,
			IdempotencyKey: strings.TrimSpace(idempotencyKey),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		r.nextID++
		r.follows[key] = follow
		if active {
			delta = 1
		}
	} else {
		if follow.Status != nextStatus {
			if active {
				delta = 1
			} else {
				delta = -1
			}
		}
		follow.Status = nextStatus
		follow.IdempotencyKey = strings.TrimSpace(idempotencyKey)
		follow.UpdatedAt = now
	}

	if delta != 0 {
		userStat := r.ensureStat(userID)
		targetStat := r.ensureStat(targetUserID)
		userStat.FollowingCount = clampMemoryCount(userStat.FollowingCount + delta)
		targetStat.FollowerCount = clampMemoryCount(targetStat.FollowerCount + delta)
	}
	return cloneFollow(follow), cloneStat(r.ensureStat(userID)), cloneStat(r.ensureStat(targetUserID)), nil
}

// ListFollowing 模拟关注列表游标分页。
func (r *memoryRelationRepo) ListFollowing(ctx context.Context, userID int64, cursor *domainrelation.ListCursor, limit int) ([]*domainrelation.UserItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*domainrelation.UserItem, 0)
	for _, follow := range r.follows {
		if follow.UserID != userID || follow.Status != domainrelation.FollowStatusActive {
			continue
		}
		if cursor != nil && !beforeRelationCursor(follow.UpdatedAt, follow.TargetUserID, cursor) {
			continue
		}
		user := r.users[follow.TargetUserID]
		items = append(items, domainrelation.RestoreUserItem(user.ID, user.Nickname, user.AvatarURL, user.Bio, follow.UpdatedAt))
	}
	sortRelationItems(items)
	return limitRelationItems(items, limit), nil
}

// ListFollowers 模拟粉丝列表游标分页。
func (r *memoryRelationRepo) ListFollowers(ctx context.Context, userID int64, cursor *domainrelation.ListCursor, limit int) ([]*domainrelation.UserItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*domainrelation.UserItem, 0)
	for _, follow := range r.follows {
		if follow.TargetUserID != userID || follow.Status != domainrelation.FollowStatusActive {
			continue
		}
		if cursor != nil && !beforeRelationCursor(follow.UpdatedAt, follow.UserID, cursor) {
			continue
		}
		user := r.users[follow.UserID]
		items = append(items, domainrelation.RestoreUserItem(user.ID, user.Nickname, user.AvatarURL, user.Bio, follow.UpdatedAt))
	}
	sortRelationItems(items)
	return limitRelationItems(items, limit), nil
}

// TestRelationFollowFlow 覆盖关注、幂等重放、取关和重复取关。
func TestRelationFollowFlow(t *testing.T) {
	router, jwtManager := newRelationRouter(t)
	token := signTestToken(t, jwtManager, 42)

	followResponse := performVideoJSONRequest(router, http.MethodPut, "/api/users/me/following/77", "", token, "follow-1")
	requireStatus(t, followResponse, http.StatusOK)

	var followed relationFollowAPIResponse
	decodeJSON(t, followResponse, &followed)
	if followed.UserID != 42 || followed.TargetUserID != 77 || !followed.Following || followed.FollowingCount != 1 || followed.FollowerCount != 1 {
		t.Fatalf("unexpected follow response: %+v", followed)
	}

	replayResponse := performVideoJSONRequest(router, http.MethodPut, "/api/users/me/following/77", "", token, "follow-1")
	requireStatus(t, replayResponse, http.StatusOK)

	var replayed relationFollowAPIResponse
	decodeJSON(t, replayResponse, &replayed)
	if !replayed.Following || replayed.FollowingCount != 1 || replayed.FollowerCount != 1 {
		t.Fatalf("unexpected replay response: %+v", replayed)
	}

	unfollowResponse := performVideoJSONRequest(router, http.MethodDelete, "/api/users/me/following/77", "", token, "unfollow-1")
	requireStatus(t, unfollowResponse, http.StatusOK)

	var unfollowed relationFollowAPIResponse
	decodeJSON(t, unfollowResponse, &unfollowed)
	if unfollowed.Following || unfollowed.FollowingCount != 0 || unfollowed.FollowerCount != 0 {
		t.Fatalf("unexpected unfollow response: %+v", unfollowed)
	}

	repeatUnfollowResponse := performVideoJSONRequest(router, http.MethodDelete, "/api/users/me/following/77", "", token, "unfollow-2")
	requireStatus(t, repeatUnfollowResponse, http.StatusOK)

	var repeatUnfollow relationFollowAPIResponse
	decodeJSON(t, repeatUnfollowResponse, &repeatUnfollow)
	if repeatUnfollow.Following || repeatUnfollow.FollowingCount != 0 || repeatUnfollow.FollowerCount != 0 {
		t.Fatalf("unexpected repeat unfollow response: %+v", repeatUnfollow)
	}
}

// TestRelationListFlow 覆盖关注列表、粉丝列表和游标分页。
func TestRelationListFlow(t *testing.T) {
	router, jwtManager := newRelationRouter(t)
	viewerToken := signTestToken(t, jwtManager, 42)
	creatorToken := signTestToken(t, jwtManager, 77)
	makerToken := signTestToken(t, jwtManager, 88)

	requireStatus(t, performVideoJSONRequest(router, http.MethodPut, "/api/users/me/following/77", "", viewerToken, "follow-77"), http.StatusOK)
	requireStatus(t, performVideoJSONRequest(router, http.MethodPut, "/api/users/me/following/88", "", viewerToken, "follow-88"), http.StatusOK)
	requireStatus(t, performVideoJSONRequest(router, http.MethodPut, "/api/users/me/following/77", "", makerToken, "maker-follow-77"), http.StatusOK)
	requireStatus(t, performVideoJSONRequest(router, http.MethodPut, "/api/users/me/following/42", "", creatorToken, "creator-follow-42"), http.StatusOK)

	firstFollowingResponse := performJSONRequest(router, http.MethodGet, "/api/users/me/following?limit=1", "", viewerToken)
	requireStatus(t, firstFollowingResponse, http.StatusOK)

	var firstFollowing relationListAPIResponse
	decodeJSON(t, firstFollowingResponse, &firstFollowing)
	if len(firstFollowing.Items) != 1 || firstFollowing.Items[0].UserID != 88 || !firstFollowing.HasMore || firstFollowing.NextCursor == "" {
		t.Fatalf("unexpected first following page: %+v", firstFollowing)
	}

	secondFollowingResponse := performJSONRequest(router, http.MethodGet, "/api/users/me/following?limit=1&cursor="+firstFollowing.NextCursor, "", viewerToken)
	requireStatus(t, secondFollowingResponse, http.StatusOK)

	var secondFollowing relationListAPIResponse
	decodeJSON(t, secondFollowingResponse, &secondFollowing)
	if len(secondFollowing.Items) != 1 || secondFollowing.Items[0].UserID != 77 || secondFollowing.HasMore {
		t.Fatalf("unexpected second following page: %+v", secondFollowing)
	}

	followersResponse := performJSONRequest(router, http.MethodGet, "/api/users/me/followers?limit=10", "", creatorToken)
	requireStatus(t, followersResponse, http.StatusOK)

	var followers relationListAPIResponse
	decodeJSON(t, followersResponse, &followers)
	if len(followers.Items) != 2 || followers.Items[0].UserID != 88 || followers.Items[1].UserID != 42 {
		t.Fatalf("unexpected followers page: %+v", followers)
	}
}

// TestRelationValidation 覆盖未登录、参数错误、自关注和目标用户缺失。
func TestRelationValidation(t *testing.T) {
	router, jwtManager := newRelationRouter(t)
	token := signTestToken(t, jwtManager, 42)

	unauthorizedResponse := performJSONRequest(router, http.MethodPut, "/api/users/me/following/77", "", "")
	requireStatus(t, unauthorizedResponse, http.StatusUnauthorized)

	badTargetResponse := performJSONRequest(router, http.MethodPut, "/api/users/me/following/0", "", token)
	requireStatus(t, badTargetResponse, http.StatusBadRequest)

	selfFollowResponse := performJSONRequest(router, http.MethodPut, "/api/users/me/following/42", "", token)
	requireStatus(t, selfFollowResponse, http.StatusBadRequest)

	missingTargetResponse := performJSONRequest(router, http.MethodPut, "/api/users/me/following/404", "", token)
	requireStatus(t, missingTargetResponse, http.StatusNotFound)

	badLimitResponse := performJSONRequest(router, http.MethodGet, "/api/users/me/following?limit=0", "", token)
	requireStatus(t, badLimitResponse, http.StatusBadRequest)

	badCursorResponse := performJSONRequest(router, http.MethodGet, "/api/users/me/followers?cursor=bad", "", token)
	requireStatus(t, badCursorResponse, http.StatusBadRequest)
}

func newRelationRouter(t *testing.T) (*gin.Engine, *infrajwt.Manager) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	jwtManager, err := infrajwt.NewManager("test-secret", "15m")
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}

	repo := newMemoryRelationRepo()
	service := applicationrelation.New(repo)
	handler := interfaceshttprelation.New(service)
	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)

	api := router.Group("/api")
	users := api.Group("/users")
	users.PUT("/me/following/:targetUserId", authMiddleware, handler.Follow)
	users.DELETE("/me/following/:targetUserId", authMiddleware, handler.Unfollow)
	users.GET("/me/following", authMiddleware, handler.ListFollowing)
	users.GET("/me/followers", authMiddleware, handler.ListFollowers)

	return router, jwtManager
}

func (r *memoryRelationRepo) userActive(userID int64) bool {
	user, exists := r.users[userID]
	return exists && user.Active
}

func (r *memoryRelationRepo) ensureStat(userID int64) *domainrelation.RelationStat {
	stat, exists := r.stats[userID]
	if exists {
		return stat
	}
	stat = &domainrelation.RelationStat{UserID: userID}
	r.stats[userID] = stat
	return stat
}

func (r *memoryRelationRepo) nextTime() time.Time {
	r.clockSeq++
	return time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC).Add(time.Duration(r.clockSeq) * time.Second)
}

func memoryRelationKey(userID int64, targetUserID int64) string {
	return int64String(userID) + ":" + int64String(targetUserID)
}

func beforeRelationCursor(followedAt time.Time, userID int64, cursor *domainrelation.ListCursor) bool {
	return followedAt.Before(cursor.FollowedAt) || (followedAt.Equal(cursor.FollowedAt) && userID < cursor.UserID)
}

func sortRelationItems(items []*domainrelation.UserItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].FollowedAt.Equal(items[j].FollowedAt) {
			return items[i].UserID > items[j].UserID
		}
		return items[i].FollowedAt.After(items[j].FollowedAt)
	})
}

func limitRelationItems(items []*domainrelation.UserItem, limit int) []*domainrelation.UserItem {
	if limit > len(items) {
		limit = len(items)
	}
	return items[:limit]
}

func cloneFollow(follow *domainrelation.Follow) *domainrelation.Follow {
	cloned := *follow
	return &cloned
}

func cloneStat(stat *domainrelation.RelationStat) *domainrelation.RelationStat {
	cloned := *stat
	return &cloned
}

var _ domainrelation.Repository = (*memoryRelationRepo)(nil)
