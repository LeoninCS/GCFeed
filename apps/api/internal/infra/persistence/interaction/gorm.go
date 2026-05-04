package infrainteraction

import (
	domainaccount "GCFeed/internal/domain/account"
	domaininteraction "GCFeed/internal/domain/interaction"
	domainvideo "GCFeed/internal/domain/video"
	infravideo "GCFeed/internal/infra/persistence/video"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

type commentWithUserModel struct {
	ID             int64
	VideoID        int64
	UserID         int64
	UserNickname   string
	UserAvatarURL  string
	Content        string
	Status         int
	IdempotencyKey *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// SetAction 写入点赞或收藏状态，并在同一事务内维护视频统计计数。
func (r *Repository) SetAction(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string) (*domaininteraction.Action, int, error) {
	actionType, err := domaininteraction.NormalizeActionType(actionType)
	if err != nil {
		return nil, 0, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	var action ActionModel
	var count int
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先锁定公开视频，保证互动只发生在可互动的视频上。
		if err := lockPublishedVideo(tx, videoID); err != nil {
			return err
		}

		// 锁定用户在该视频上的同类行为记录，避免并发请求同时改计数。
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND video_id = ? AND action_type = ?", userID, videoID, actionType).
			Take(&action).
			Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}

		delta := 0
		nextStatus := actionStatusFromActive(active)
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			// 首次 DELETE 会创建取消态记录，保证后续 PUT/DELETE 都有稳定幂等基准。
			action = ActionModel{
				UserID:         userID,
				VideoID:        videoID,
				ActionType:     actionType,
				Status:         nextStatus,
				IdempotencyKey: idempotencyKeyPtr(idempotencyKey),
			}
			if err := tx.Create(&action).Error; err != nil {
				return err
			}
			if active {
				delta = 1
			}
		} else {
			// 同一幂等键直接返回当前计数，避免客户端重试重复变更统计。
			if idempotencyKey != "" && idempotencyKeyValue(action.IdempotencyKey) == idempotencyKey {
				currentCount, err := currentActionCount(tx, videoID, actionType)
				if err != nil {
					return err
				}
				count = currentCount
				return nil
			}

			// 只有状态真正变化时才更新 video_stat，重复 PUT 或 DELETE 保持计数稳定。
			previousStatus := action.Status
			previousIdempotencyKey := idempotencyKeyValue(action.IdempotencyKey)
			if action.Status != nextStatus {
				if active {
					delta = 1
				} else {
					delta = -1
				}
			}
			action.Status = nextStatus
			action.IdempotencyKey = idempotencyKeyPtr(idempotencyKey)
			if previousStatus != nextStatus || previousIdempotencyKey != idempotencyKey {
				if err := tx.Save(&action).Error; err != nil {
					return err
				}
			}
		}

		if delta == 0 {
			currentCount, err := currentActionCount(tx, videoID, actionType)
			if err != nil {
				return err
			}
			count = currentCount
			return nil
		}

		count, err = updateActionStat(tx, videoID, actionType, delta)
		return err
	})
	if err != nil {
		return nil, 0, mapVideoError(err)
	}
	return restoreAction(action), count, nil
}

// actionStatusFromActive 将接口目标状态转换为数据库状态枚举。
func actionStatusFromActive(active bool) int {
	if active {
		return domaininteraction.ActionStatusActive
	}
	return domaininteraction.ActionStatusCanceled
}

// CreateComment 创建评论，并在同一事务内增加视频评论数。
func (r *Repository) CreateComment(ctx context.Context, comment *domaininteraction.Comment) (*domaininteraction.Comment, int, error) {
	var model CommentModel
	var count int
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 评论只能写入已发布视频，锁定视频行可以避免状态变化时写入脏数据。
		if err := lockPublishedVideo(tx, comment.VideoID); err != nil {
			return err
		}

		model = CommentModel{
			VideoID:        comment.VideoID,
			UserID:         comment.UserID,
			Content:        comment.Content,
			Status:         comment.Status,
			IdempotencyKey: idempotencyKeyPtr(comment.IdempotencyKey),
		}
		if err := tx.Create(&model).Error; err != nil {
			// 唯一键冲突通常表示同一幂等键已创建过评论，交给外层加载已有结果。
			if isDuplicateKeyError(err) && comment.IdempotencyKey != "" {
				return domaininteraction.ErrCommentNotFound
			}
			return err
		}

		nextCount, err := updateVideoStatCounter(tx, model.VideoID, "comment_count", 1)
		if err != nil {
			return err
		}
		count = nextCount
		return nil
	})
	if err != nil {
		if errors.Is(err, domaininteraction.ErrCommentNotFound) && comment.IdempotencyKey != "" {
			existing, existingCount, loadErr := r.FindCommentByUserAndIdempotencyKey(ctx, comment.UserID, comment.IdempotencyKey)
			if loadErr == nil {
				return existing, existingCount, nil
			}
		}
		return nil, 0, mapVideoError(err)
	}

	created, err := r.FindCommentByID(ctx, model.ID)
	if err != nil {
		return nil, 0, err
	}
	return created, count, nil
}

// FindCommentByUserAndIdempotencyKey 根据用户和幂等键查找已创建评论。
func (r *Repository) FindCommentByUserAndIdempotencyKey(ctx context.Context, userID int64, idempotencyKey string) (*domaininteraction.Comment, int, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return nil, 0, domaininteraction.ErrCommentNotFound
	}

	var model commentWithUserModel
	result := r.db.WithContext(ctx).
		Table("interaction_comment AS c").
		Select(commentWithUserSelect()).
		Joins("LEFT JOIN account AS a ON a.id = c.user_id").
		Where("c.user_id = ? AND c.idempotency_key = ?", userID, idempotencyKey).
		Limit(1).
		Find(&model)
	if result.Error != nil {
		return nil, 0, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, 0, domaininteraction.ErrCommentNotFound
	}

	count, err := r.commentCount(ctx, model.VideoID)
	if err != nil {
		return nil, 0, err
	}
	return restoreComment(model), count, nil
}

// FindCommentByID 查询评论详情，同时补齐评论用户昵称和头像。
func (r *Repository) FindCommentByID(ctx context.Context, commentID int64) (*domaininteraction.Comment, error) {
	var model commentWithUserModel
	err := r.db.WithContext(ctx).
		Table("interaction_comment AS c").
		Select(commentWithUserSelect()).
		Joins("LEFT JOIN account AS a ON a.id = c.user_id").
		Where("c.id = ?", commentID).
		Take(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domaininteraction.ErrCommentNotFound
		}
		return nil, err
	}
	return restoreComment(model), nil
}

// ListComments 按 created_at 和 id 倒序查询视频评论，支持稳定游标分页。
func (r *Repository) ListComments(ctx context.Context, videoID int64, cursor *domaininteraction.CommentCursor, limit int) ([]*domaininteraction.Comment, error) {
	var models []commentWithUserModel
	query := r.db.WithContext(ctx).
		Table("interaction_comment AS c").
		Select(commentWithUserSelect()).
		Joins("LEFT JOIN account AS a ON a.id = c.user_id").
		Where("c.video_id = ? AND c.status = ?", videoID, domaininteraction.CommentStatusNormal)

	if cursor != nil {
		// 游标条件和排序字段一致，保证翻页时没有重复项。
		query = query.Where(
			"(c.created_at < ? OR (c.created_at = ? AND c.id < ?))",
			cursor.CreatedAt,
			cursor.CreatedAt,
			cursor.CommentID,
		)
	}

	if err := query.Order("c.created_at DESC").Order("c.id DESC").Limit(limit).Scan(&models).Error; err != nil {
		return nil, err
	}

	comments := make([]*domaininteraction.Comment, 0, len(models))
	for _, model := range models {
		comments = append(comments, restoreComment(model))
	}
	return comments, nil
}

// DeleteComment 软删除评论，并根据操作者身份校验删除权限。
func (r *Repository) DeleteComment(ctx context.Context, commentID int64, userID int64, role string) (*domaininteraction.Comment, int, error) {
	role = strings.TrimSpace(role)
	var model CommentModel
	var count int
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 锁定评论行，避免重复删除时并发扣减 comment_count。
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", commentID).
			Take(&model).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domaininteraction.ErrCommentNotFound
			}
			return err
		}

		var video infravideo.VideoModel
		// 读取视频作者用于权限判断：评论作者、视频作者、管理员都可删除。
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", model.VideoID).
			Take(&video).
			Error; err != nil {
			return mapVideoError(err)
		}

		if model.UserID != userID && video.AuthorID != userID && role != domainaccount.RoleAdmin {
			return domaininteraction.ErrCommentPermissionDenied
		}

		// 已删除评论直接返回当前计数，保持 DELETE 幂等。
		if model.Status == domaininteraction.CommentStatusDeleted {
			currentCount, err := currentVideoStatCounter(tx, model.VideoID, "comment_count")
			if err != nil {
				return err
			}
			count = currentCount
			return nil
		}

		model.Status = domaininteraction.CommentStatusDeleted
		if err := tx.Save(&model).Error; err != nil {
			return err
		}

		nextCount, err := updateVideoStatCounter(tx, model.VideoID, "comment_count", -1)
		if err != nil {
			return err
		}
		count = nextCount
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	comment, err := r.FindCommentByID(ctx, model.ID)
	if err != nil {
		return nil, 0, err
	}
	return comment, count, nil
}

// commentCount 读取视频当前评论数，用于幂等评论创建返回一致响应。
func (r *Repository) commentCount(ctx context.Context, videoID int64) (int, error) {
	var stat infravideo.VideoStatModel
	err := r.db.WithContext(ctx).Where("video_id = ?", videoID).Take(&stat).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, domaininteraction.ErrVideoNotFound
		}
		return 0, err
	}
	return stat.CommentCount, nil
}

// lockPublishedVideo 校验并锁定已发布视频，互动写入前都会经过这里。
func lockPublishedVideo(tx *gorm.DB, videoID int64) error {
	var video infravideo.VideoModel
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND status = ?", videoID, domainvideo.StatusPublished).
		Take(&video).
		Error
	return mapVideoError(err)
}

// updateActionStat 根据行为类型选择要更新的统计字段。
func updateActionStat(tx *gorm.DB, videoID int64, actionType string, delta int) (int, error) {
	if actionType == domaininteraction.ActionTypeLike {
		return updateVideoStatCounter(tx, videoID, "like_count", delta)
	}
	return updateVideoStatCounter(tx, videoID, "favorite_count", delta)
}

// currentActionCount 根据行为类型读取当前统计值。
func currentActionCount(tx *gorm.DB, videoID int64, actionType string) (int, error) {
	if actionType == domaininteraction.ActionTypeLike {
		return currentVideoStatCounter(tx, videoID, "like_count")
	}
	return currentVideoStatCounter(tx, videoID, "favorite_count")
}

// updateVideoStatCounter 锁定 video_stat 后更新计数，避免并发写丢失。
func updateVideoStatCounter(tx *gorm.DB, videoID int64, field string, delta int) (int, error) {
	var stat infravideo.VideoStatModel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("video_id = ?", videoID).Take(&stat).Error; err != nil {
		return 0, mapVideoError(err)
	}

	switch field {
	case "like_count":
		stat.LikeCount = clampCount(stat.LikeCount + delta)
	case "favorite_count":
		stat.FavoriteCount = clampCount(stat.FavoriteCount + delta)
	case "comment_count":
		stat.CommentCount = clampCount(stat.CommentCount + delta)
	default:
		return 0, domaininteraction.ErrInvalidActionType
	}

	if err := tx.Save(&stat).Error; err != nil {
		return 0, err
	}
	return statCounter(stat, field), nil
}

func currentVideoStatCounter(tx *gorm.DB, videoID int64, field string) (int, error) {
	var stat infravideo.VideoStatModel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("video_id = ?", videoID).Take(&stat).Error; err != nil {
		return 0, mapVideoError(err)
	}
	return statCounter(stat, field), nil
}

// statCounter 根据字段名从统计模型中取出对应计数。
func statCounter(stat infravideo.VideoStatModel, field string) int {
	switch field {
	case "like_count":
		return stat.LikeCount
	case "favorite_count":
		return stat.FavoriteCount
	case "comment_count":
		return stat.CommentCount
	default:
		return 0
	}
}

// clampCount 防止并发或脏数据导致计数变成负数。
func clampCount(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

// restoreAction 把数据库互动行为转换为领域对象。
func restoreAction(model ActionModel) *domaininteraction.Action {
	return domaininteraction.RestoreAction(
		model.ID,
		model.UserID,
		model.VideoID,
		model.ActionType,
		model.Status,
		idempotencyKeyValue(model.IdempotencyKey),
		model.CreatedAt,
		model.UpdatedAt,
	)
}

// restoreComment 把评论联表查询结果转换为领域对象。
func restoreComment(model commentWithUserModel) *domaininteraction.Comment {
	return domaininteraction.RestoreComment(
		model.ID,
		model.VideoID,
		model.UserID,
		model.UserNickname,
		model.UserAvatarURL,
		model.Content,
		model.Status,
		idempotencyKeyValue(model.IdempotencyKey),
		model.CreatedAt,
		model.UpdatedAt,
	)
}

// commentWithUserSelect 统一评论查询字段，并附带用户昵称和头像。
func commentWithUserSelect() string {
	return "c.id, c.video_id, c.user_id, a.nickname AS user_nickname, a.avatar_url AS user_avatar_url, c.content, c.status, c.idempotency_key, c.created_at, c.updated_at"
}

// idempotencyKeyPtr 将空幂等键存为 NULL，减少唯一索引冲突。
func idempotencyKeyPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// idempotencyKeyValue 将数据库可空字段还原为领域层字符串。
func idempotencyKeyValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// mapVideoError 把 GORM 找不到记录转换为互动领域的视频不存在错误。
func mapVideoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domaininteraction.ErrVideoNotFound
	}
	return err
}

// isDuplicateKeyError 兼容 GORM 标准错误和 MySQL 1062 唯一键冲突。
func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

var _ domaininteraction.Repository = (*Repository)(nil)
