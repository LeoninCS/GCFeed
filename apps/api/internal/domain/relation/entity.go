package domainrelation

import (
	"strings"
	"time"
)

const (
	FollowStatusActive   = 1
	FollowStatusCanceled = 2

	MaxIdempotencyKeyLength = 128
	MaxLimit                = 100
)

// Follow 表示一个用户对另一个用户的关注关系，取关使用软状态保留历史。
type Follow struct {
	ID             int64
	UserID         int64
	TargetUserID   int64
	Status         int
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RelationStat 保存用户维度的关注数和粉丝数。
type RelationStat struct {
	UserID         int64
	FollowingCount int
	FollowerCount  int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UserItem 是关注列表或粉丝列表中的用户展示数据。
type UserItem struct {
	UserID     int64
	Nickname   string
	AvatarURL  string
	Bio        string
	FollowedAt time.Time
}

// ListCursor 保存关系列表分页需要的排序字段。
type ListCursor struct {
	FollowedAt time.Time
	UserID     int64
}

// NewFollow 创建关注关系，负责基础 ID 和幂等键校验。
func NewFollow(userID int64, targetUserID int64, idempotencyKey string) (*Follow, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	if targetUserID <= 0 {
		return nil, ErrInvalidTargetUserID
	}
	if userID == targetUserID {
		return nil, ErrFollowSelfForbidden
	}

	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > MaxIdempotencyKeyLength {
		return nil, ErrIdempotencyKeyTooLong
	}

	return &Follow{
		UserID:         userID,
		TargetUserID:   targetUserID,
		Status:         FollowStatusActive,
		IdempotencyKey: idempotencyKey,
	}, nil
}

// RestoreFollow 从数据库记录恢复关注关系。
func RestoreFollow(id int64, userID int64, targetUserID int64, status int, idempotencyKey string, createdAt time.Time, updatedAt time.Time) *Follow {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if status == 0 {
		status = FollowStatusActive
	}

	return &Follow{
		ID:             id,
		UserID:         userID,
		TargetUserID:   targetUserID,
		Status:         status,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

// RestoreRelationStat 从数据库统计记录恢复领域统计对象。
func RestoreRelationStat(userID int64, followingCount int, followerCount int, createdAt time.Time, updatedAt time.Time) *RelationStat {
	return &RelationStat{
		UserID:         userID,
		FollowingCount: clampCount(followingCount),
		FollowerCount:  clampCount(followerCount),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}

// RestoreUserItem 从查询结果恢复列表展示用户。
func RestoreUserItem(userID int64, nickname string, avatarURL string, bio string, followedAt time.Time) *UserItem {
	return &UserItem{
		UserID:     userID,
		Nickname:   strings.TrimSpace(nickname),
		AvatarURL:  strings.TrimSpace(avatarURL),
		Bio:        strings.TrimSpace(bio),
		FollowedAt: followedAt,
	}
}

// Active 判断当前关系是否处于关注中。
func (f *Follow) Active() bool {
	return f.Status == FollowStatusActive
}

func clampCount(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
