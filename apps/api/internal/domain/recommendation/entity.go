package domainrecommendation

import (
	"math"
	"strings"
	"time"
)

const MaxLimit = 100
const MaxSceneLength = 32
const MaxRequestIDLength = 64
const RecentExposureWindow = 7 * 24 * time.Hour

const ExposureDecisionReasonFresh = "fresh"
const ExposureDecisionReasonRecentlyExposed = "recently_exposed"

type CandidateRequest struct {
	UserID    int64
	Scene     string
	RequestID string
	Cursor    *Cursor
	Limit     int
}

type Cursor struct {
	RankScore   float64
	PublishedAt time.Time
	VideoID     int64
}

type Candidate struct {
	VideoID        int64
	AuthorID       int64
	RankScore      float64
	Similarity     float64
	HotScore       int
	FreshnessScore float64
	Reason         string
	PublishedAt    time.Time
}

type ExposureWrite struct {
	UserID    int64
	VideoID   int64
	Scene     string
	RequestID string
}

type ExposureDecisionRequest struct {
	UserID    int64
	Scene     string
	RequestID string
	VideoIDs  []int64
}

type Exposure struct {
	ID             int64
	UserID         int64
	VideoID        int64
	FirstExposedAt time.Time
	LastExposedAt  time.Time
	ExposureCount  int
	LastScene      string
}

type ExposureDecision struct {
	VideoID       int64
	Allowed       bool
	Reason        string
	LastExposedAt *time.Time
}

func NewCandidateRequest(userID int64, scene string, requestID string, cursor *Cursor, limit int) (*CandidateRequest, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	scene = strings.TrimSpace(strings.ToLower(scene))
	requestID = strings.TrimSpace(requestID)
	if scene == "" {
		return nil, ErrEmptyScene
	}
	if len(scene) > MaxSceneLength {
		return nil, ErrSceneTooLong
	}
	if len(requestID) > MaxRequestIDLength {
		return nil, ErrRequestIDTooLong
	}
	if limit <= 0 || limit > MaxLimit {
		return nil, ErrInvalidLimit
	}
	if cursor != nil && !cursor.Valid() {
		return nil, ErrInvalidCursor
	}
	return &CandidateRequest{
		UserID:    userID,
		Scene:     scene,
		RequestID: requestID,
		Cursor:    cursor,
		Limit:     limit,
	}, nil
}

func NewExposureDecisionRequest(userID int64, scene string, requestID string, videoIDs []int64) (*ExposureDecisionRequest, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	scene = strings.TrimSpace(strings.ToLower(scene))
	requestID = strings.TrimSpace(requestID)
	if scene == "" {
		return nil, ErrEmptyScene
	}
	if len(scene) > MaxSceneLength {
		return nil, ErrSceneTooLong
	}
	if len(requestID) > MaxRequestIDLength {
		return nil, ErrRequestIDTooLong
	}
	deduped := make([]int64, 0, len(videoIDs))
	seen := map[int64]struct{}{}
	for _, videoID := range videoIDs {
		if videoID <= 0 {
			return nil, ErrInvalidVideoID
		}
		if _, exists := seen[videoID]; exists {
			continue
		}
		seen[videoID] = struct{}{}
		deduped = append(deduped, videoID)
	}
	return &ExposureDecisionRequest{
		UserID:    userID,
		Scene:     scene,
		RequestID: requestID,
		VideoIDs:  deduped,
	}, nil
}

func NewExposureWrite(userID int64, videoID int64, scene string, requestID string) (*ExposureWrite, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	if videoID <= 0 {
		return nil, ErrInvalidVideoID
	}
	scene = strings.TrimSpace(strings.ToLower(scene))
	requestID = strings.TrimSpace(requestID)
	if scene == "" {
		return nil, ErrEmptyScene
	}
	if len(scene) > MaxSceneLength {
		return nil, ErrSceneTooLong
	}
	if len(requestID) > MaxRequestIDLength {
		return nil, ErrRequestIDTooLong
	}
	return &ExposureWrite{
		UserID:    userID,
		VideoID:   videoID,
		Scene:     scene,
		RequestID: requestID,
	}, nil
}

func RestoreCandidate(videoID int64, authorID int64, rankScore float64, similarity float64, hotScore int, freshnessScore float64, reason string, publishedAt time.Time) *Candidate {
	return &Candidate{
		VideoID:        videoID,
		AuthorID:       authorID,
		RankScore:      rankScore,
		Similarity:     similarity,
		HotScore:       hotScore,
		FreshnessScore: freshnessScore,
		Reason:         strings.TrimSpace(reason),
		PublishedAt:    publishedAt,
	}
}

func RestoreExposure(id int64, userID int64, videoID int64, firstExposedAt time.Time, lastExposedAt time.Time, exposureCount int, lastScene string) *Exposure {
	return &Exposure{
		ID:             id,
		UserID:         userID,
		VideoID:        videoID,
		FirstExposedAt: firstExposedAt,
		LastExposedAt:  lastExposedAt,
		ExposureCount:  exposureCount,
		LastScene:      strings.TrimSpace(lastScene),
	}
}

func RestoreExposureDecision(videoID int64, allowed bool, reason string, lastExposedAt *time.Time) *ExposureDecision {
	var exposedAt *time.Time
	if lastExposedAt != nil {
		value := *lastExposedAt
		exposedAt = &value
	}
	return &ExposureDecision{
		VideoID:       videoID,
		Allowed:       allowed,
		Reason:        strings.TrimSpace(reason),
		LastExposedAt: exposedAt,
	}
}

func (c *Cursor) Valid() bool {
	return c != nil && c.VideoID > 0 && !c.PublishedAt.IsZero() && !math.IsNaN(c.RankScore) && !math.IsInf(c.RankScore, 0)
}
