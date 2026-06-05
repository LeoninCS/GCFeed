package inframessage

import (
	domainmessage "GCFeed/internal/domain/message"
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

// New 创建消息仓储实现。
func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create 保存新消息；事件 ID 或幂等键命中时返回既有消息。
func (r *Repository) Create(ctx context.Context, message *domainmessage.Message, idempotencyKey string) (*domainmessage.Message, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	model := MessageModel{
		UserID:         message.UserID,
		Type:           message.Type,
		Title:          message.Title,
		Content:        message.Content,
		EventID:        optionalString(message.EventID),
		IdempotencyKey: optionalString(idempotencyKey),
		IsRead:         false,
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		return restore(model), true, nil
	}
	if !isDuplicateKeyError(err) {
		return nil, false, err
	}

	existing, findErr := r.findExisting(ctx, message.UserID, message.EventID, idempotencyKey)
	if findErr != nil {
		return nil, false, findErr
	}
	return restore(existing), false, nil
}

// ListByUser 按创建时间和 ID 倒序读取当前用户消息。
func (r *Repository) ListByUser(ctx context.Context, userID int64, cursor *domainmessage.Cursor, limit int) ([]*domainmessage.Message, error) {
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID)

	if cursor != nil {
		query = query.Where(
			"(created_at < ? OR (created_at = ? AND id < ?))",
			cursor.CreatedAt,
			cursor.CreatedAt,
			cursor.MessageID,
		)
	}

	var models []MessageModel
	if err := query.Order("created_at DESC").Order("id DESC").Limit(limit).Find(&models).Error; err != nil {
		return nil, err
	}

	items := make([]*domainmessage.Message, 0, len(models))
	for _, model := range models {
		items = append(items, restore(model))
	}
	return items, nil
}

// CountUnread 统计当前用户未读消息。
func (r *Repository) CountUnread(ctx context.Context, userID int64) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&MessageModel{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).
		Error
	return int(count), err
}

// MarkRead 将当前用户指定消息标为已读；空列表表示全部消息。
func (r *Repository) MarkRead(ctx context.Context, userID int64, messageIDs []int64) (int, error) {
	now := time.Now().UTC()
	query := r.db.WithContext(ctx).
		Model(&MessageModel{}).
		Where("user_id = ? AND is_read = ?", userID, false)
	if len(messageIDs) > 0 {
		query = query.Where("id IN ?", messageIDs)
	}

	result := query.Updates(map[string]any{
		"is_read": true,
		"read_at": now,
	})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

func (r *Repository) findExisting(ctx context.Context, userID int64, eventID string, idempotencyKey string) (MessageModel, error) {
	var model MessageModel
	query := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID)

	eventID = strings.TrimSpace(eventID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	switch {
	case eventID != "" && idempotencyKey != "":
		query = query.Where("event_id = ? OR idempotency_key = ?", eventID, idempotencyKey)
	case eventID != "":
		query = query.Where("event_id = ?", eventID)
	case idempotencyKey != "":
		query = query.Where("idempotency_key = ?", idempotencyKey)
	default:
		return model, domainmessage.ErrMessageNotFound
	}

	err := query.Order("id DESC").Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model, domainmessage.ErrMessageNotFound
		}
		return model, err
	}
	return model, nil
}

func restore(model MessageModel) *domainmessage.Message {
	return domainmessage.Restore(
		model.ID,
		model.UserID,
		model.Type,
		model.Title,
		model.Content,
		stringValue(model.EventID),
		model.IsRead,
		model.CreatedAt,
		model.ReadAt,
	)
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
