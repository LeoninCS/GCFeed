package domainrecommendation

import (
	"context"
	"time"
)

type Repository interface {
	ListCandidatePool(ctx context.Context, userID int64, limit int) ([]*Candidate, error)
	LoadUserInterestVector(ctx context.Context, userID int64) ([]float64, bool, error)
	LoadVideoVectors(ctx context.Context, videoIDs []int64) (map[int64][]float64, error)
	ListRecentExposures(ctx context.Context, userID int64, videoIDs []int64, since time.Time) ([]*Exposure, error)
	SaveExposures(ctx context.Context, exposures []*ExposureWrite) ([]*Exposure, error)
}
