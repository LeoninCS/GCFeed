package infrarecommendation

import (
	domainembedding "GCFeed/internal/domain/embedding"
	domainexposure "GCFeed/internal/domain/exposure"
	domainrecommendation "GCFeed/internal/domain/recommendation"
	domainvideo "GCFeed/internal/domain/video"
	infraexposure "GCFeed/internal/infra/persistence/exposure"
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const hotScoreExpression = "COALESCE(vs.like_count, 0) * 3 + COALESCE(vs.comment_count, 0) * 5 + COALESCE(vs.favorite_count, 0) * 4"
const positiveEventWindow = 30 * 24 * time.Hour

type Repository struct {
	db *gorm.DB
}

type candidateModel struct {
	VideoID     int64
	AuthorID    int64
	HotScore    int
	PublishedAt time.Time
}

type videoVectorModel struct {
	VideoID       int64
	EmbeddingJSON string
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListCandidatePool(ctx context.Context, userID int64, limit int) ([]*domainrecommendation.Candidate, error) {
	if limit <= 0 {
		return []*domainrecommendation.Candidate{}, nil
	}

	var models []candidateModel
	err := r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, v.author_id, ("+hotScoreExpression+") AS hot_score, v.published_at").
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Joins(
			"LEFT JOIN exposures AS e ON e.user_id = ? AND e.video_id = v.id AND e.last_exposed_at >= ?",
			userID,
			time.Now().Add(-domainrecommendation.RecentExposureWindow),
		).
		Where("v.status = ? AND v.published_at IS NOT NULL AND e.video_id IS NULL", domainvideo.StatusPublished).
		Order("hot_score DESC").
		Order("v.published_at DESC").
		Order("v.id DESC").
		Limit(limit).
		Scan(&models).
		Error
	if err != nil {
		return nil, err
	}

	candidates := make([]*domainrecommendation.Candidate, 0, len(models))
	for _, model := range models {
		candidates = append(candidates, domainrecommendation.RestoreCandidate(
			model.VideoID,
			model.AuthorID,
			0,
			0,
			model.HotScore,
			0,
			"",
			model.PublishedAt,
		))
	}
	return candidates, nil
}

func (r *Repository) LoadUserInterestVector(ctx context.Context, userID int64) ([]float64, bool, error) {
	rows, err := r.db.WithContext(ctx).
		Table("video_view_events AS ev").
		Select("ve.embedding_json, ev.event_type, ev.watch_ms, ev.completed").
		Joins("JOIN video_embedding AS ve ON ve.video_id = ev.video_id AND ve.model = ?", domainembedding.HashNgramModel).
		Where("ev.user_id = ? AND ev.created_at >= ? AND ev.event_type IN ?", userID, time.Now().Add(-positiveEventWindow), positiveEventTypes()).
		Order("ev.created_at DESC").
		Limit(200).
		Rows()
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var sum []float64
	var totalWeight float64
	for rows.Next() {
		var embeddingJSON string
		var eventType string
		var watchMs int
		var completed bool
		if err := rows.Scan(&embeddingJSON, &eventType, &watchMs, &completed); err != nil {
			return nil, false, err
		}
		vector, err := decodeVector(embeddingJSON)
		if err != nil || len(vector) == 0 {
			continue
		}
		if len(sum) == 0 {
			sum = make([]float64, len(vector))
		}
		if len(vector) != len(sum) {
			continue
		}
		weight := eventWeight(eventType, watchMs, completed)
		for i := range vector {
			sum[i] += vector[i] * weight
		}
		totalWeight += weight
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(sum) == 0 || totalWeight == 0 {
		return nil, false, nil
	}
	for i := range sum {
		sum[i] = sum[i] / totalWeight
	}
	return sum, true, nil
}

func (r *Repository) LoadVideoVectors(ctx context.Context, videoIDs []int64) (map[int64][]float64, error) {
	vectors := map[int64][]float64{}
	if len(videoIDs) == 0 {
		return vectors, nil
	}

	var models []videoVectorModel
	err := r.db.WithContext(ctx).
		Table("video_embedding").
		Select("video_id, embedding_json").
		Where("video_id IN ? AND model = ?", videoIDs, domainembedding.HashNgramModel).
		Scan(&models).
		Error
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		vector, err := decodeVector(model.EmbeddingJSON)
		if err != nil {
			continue
		}
		vectors[model.VideoID] = vector
	}
	return vectors, nil
}

func (r *Repository) ListRecentExposures(ctx context.Context, userID int64, videoIDs []int64, since time.Time) ([]*domainrecommendation.Exposure, error) {
	if len(videoIDs) == 0 {
		return []*domainrecommendation.Exposure{}, nil
	}

	var models []infraexposure.ExposureModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND video_id IN ? AND last_exposed_at >= ?", userID, videoIDs, since).
		Find(&models).
		Error
	if err != nil {
		return nil, err
	}
	exposures := make([]*domainrecommendation.Exposure, 0, len(models))
	for _, model := range models {
		exposures = append(exposures, restoreExposure(model))
	}
	return exposures, nil
}

func (r *Repository) SaveExposures(ctx context.Context, writes []*domainrecommendation.ExposureWrite) ([]*domainrecommendation.Exposure, error) {
	if len(writes) == 0 {
		return []*domainrecommendation.Exposure{}, nil
	}

	exposures := make([]*domainrecommendation.Exposure, 0, len(writes))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, write := range writes {
			if write == nil {
				continue
			}
			if err := ensurePublishedVideo(tx, write.VideoID); err != nil {
				return err
			}
			event := infraexposure.ViewEventModel{
				UserID:    write.UserID,
				VideoID:   write.VideoID,
				Scene:     write.Scene,
				RequestID: stringPtr(write.RequestID),
				EventType: domainexposure.EventTypeExposed,
				WatchMs:   0,
				Completed: false,
			}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
			model := infraexposure.ExposureModel{
				UserID:         write.UserID,
				VideoID:        write.VideoID,
				FirstExposedAt: event.CreatedAt,
				LastExposedAt:  event.CreatedAt,
				ExposureCount:  1,
				LastScene:      write.Scene,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "user_id"},
					{Name: "video_id"},
				},
				DoUpdates: clause.Assignments(map[string]any{
					"last_exposed_at": gorm.Expr("VALUES(last_exposed_at)"),
					"exposure_count":  gorm.Expr("exposure_count + 1"),
					"last_scene":      gorm.Expr("VALUES(last_scene)"),
					"updated_at":      gorm.Expr("VALUES(updated_at)"),
				}),
			}).Create(&model).Error; err != nil {
				return err
			}

			var saved infraexposure.ExposureModel
			if err := tx.Where("user_id = ? AND video_id = ?", write.UserID, write.VideoID).Take(&saved).Error; err != nil {
				return err
			}
			exposures = append(exposures, restoreExposure(saved))
		}
		return nil
	})
	if err != nil {
		return nil, mapRecommendationError(err)
	}
	return exposures, nil
}

func decodeVector(content string) ([]float64, error) {
	var vector []float64
	if err := json.Unmarshal([]byte(content), &vector); err != nil {
		return nil, err
	}
	return vector, nil
}

func positiveEventTypes() []string {
	return []string{
		domainexposure.EventTypePlay,
		domainexposure.EventTypeComplete,
	}
}

func eventWeight(eventType string, watchMs int, completed bool) float64 {
	switch eventType {
	case domainexposure.EventTypeComplete:
		return 3
	case domainexposure.EventTypePlay:
		weight := 1 + float64(watchMs)/30000
		if weight > 2 {
			weight = 2
		}
		if completed {
			weight += 1
		}
		return weight
	default:
		return 1
	}
}

func ensurePublishedVideo(tx *gorm.DB, videoID int64) error {
	var item struct {
		ID int64
	}
	err := tx.Table("video").
		Select("id").
		Where("id = ? AND status = ?", videoID, domainvideo.StatusPublished).
		Take(&item).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domainrecommendation.ErrVideoNotFound
		}
		return err
	}
	return nil
}

func restoreExposure(model infraexposure.ExposureModel) *domainrecommendation.Exposure {
	return domainrecommendation.RestoreExposure(
		model.ID,
		model.UserID,
		model.VideoID,
		model.FirstExposedAt,
		model.LastExposedAt,
		model.ExposureCount,
		model.LastScene,
	)
}

func mapRecommendationError(err error) error {
	if errors.Is(err, domainrecommendation.ErrVideoNotFound) {
		return err
	}
	return err
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
