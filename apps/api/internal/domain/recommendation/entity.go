package domainrecommendation

import (
	"math"
	"strings"
	"time"
)

const MaxLimit = 100
const MaxSceneLength = 32
const MaxRequestIDLength = 64

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

type Exposure struct {
	ID             int64
	UserID         int64
	VideoID        int64
	FirstExposedAt time.Time
	LastExposedAt  time.Time
	ExposureCount  int
	LastScene      string
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

func (c *Cursor) Valid() bool {
	return c != nil && c.VideoID > 0 && !c.PublishedAt.IsZero() && !math.IsNaN(c.RankScore) && !math.IsInf(c.RankScore, 0)
}
