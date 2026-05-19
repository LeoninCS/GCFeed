package applicationrecommendation

import (
	domainembedding "GCFeed/internal/domain/embedding"
	domainrecommendation "GCFeed/internal/domain/recommendation"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

const defaultLimit = 10
const candidatePoolMultiplier = 8
const minCandidatePoolSize = 50
const maxCandidatePoolSize = 500

var ErrLoadRecommendationFailed = errors.New("failed to load recommendations")
var ErrSaveRecommendationExposureFailed = errors.New("failed to save recommendation exposure")

type Service struct {
	repo domainrecommendation.Repository
	now  func() time.Time
}

type Option func(*Service)

type CandidateRequest struct {
	UserID    int64
	Scene     string
	RequestID string
	Cursor    string
	Limit     int
}

type CandidateResult struct {
	UserID     int64
	Scene      string
	RequestID  string
	Candidates []*domainrecommendation.Candidate
	NextCursor string
	HasMore    bool
}

type ExposureInput struct {
	UserID    int64
	VideoID   int64
	Scene     string
	RequestID string
}

type ExposureResult struct {
	Exposures []*domainrecommendation.Exposure
}

type cursorPayload struct {
	RankScore   float64 `json:"rank_score"`
	PublishedAt string  `json:"published_at"`
	VideoID     int64   `json:"video_id"`
}

func New(repo domainrecommendation.Repository, options ...Option) *Service {
	service := &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC().Truncate(time.Minute) },
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func (s *Service) Recommend(ctx context.Context, input CandidateRequest) (*CandidateResult, error) {
	limit := normalizeLimit(input.Limit)
	cursor, err := parseCursor(input.Cursor)
	if err != nil {
		return nil, err
	}
	req, err := domainrecommendation.NewCandidateRequest(input.UserID, input.Scene, input.RequestID, cursor, limit)
	if err != nil {
		return nil, err
	}

	poolLimit := candidatePoolLimit(limit)
	pool, err := s.repo.ListCandidatePool(ctx, req.UserID, poolLimit)
	if err != nil {
		return nil, ErrLoadRecommendationFailed
	}

	ranked, err := s.rankCandidates(ctx, req.UserID, pool)
	if err != nil {
		return nil, ErrLoadRecommendationFailed
	}
	ranked = filterByCursor(ranked, req.Cursor)

	hasMore := len(ranked) > limit
	if hasMore {
		ranked = ranked[:limit]
	}

	nextCursor := ""
	if len(ranked) > 0 {
		nextCursor = encodeCursor(&domainrecommendation.Cursor{
			RankScore:   ranked[len(ranked)-1].RankScore,
			PublishedAt: ranked[len(ranked)-1].PublishedAt,
			VideoID:     ranked[len(ranked)-1].VideoID,
		})
	}
	ranked = interleaveByAuthor(ranked)

	return &CandidateResult{
		UserID:     req.UserID,
		Scene:      req.Scene,
		RequestID:  req.RequestID,
		Candidates: ranked,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *Service) SaveExposures(ctx context.Context, inputs []ExposureInput) (*ExposureResult, error) {
	writes := make([]*domainrecommendation.ExposureWrite, 0, len(inputs))
	seen := map[int64]struct{}{}
	for _, input := range inputs {
		write, err := domainrecommendation.NewExposureWrite(input.UserID, input.VideoID, input.Scene, input.RequestID)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[write.VideoID]; exists {
			continue
		}
		seen[write.VideoID] = struct{}{}
		writes = append(writes, write)
	}
	if len(writes) == 0 {
		return &ExposureResult{Exposures: []*domainrecommendation.Exposure{}}, nil
	}

	exposures, err := s.repo.SaveExposures(ctx, writes)
	if err != nil {
		if errors.Is(err, domainrecommendation.ErrVideoNotFound) {
			return nil, err
		}
		return nil, ErrSaveRecommendationExposureFailed
	}
	return &ExposureResult{Exposures: exposures}, nil
}

func (s *Service) rankCandidates(ctx context.Context, userID int64, pool []*domainrecommendation.Candidate) ([]*domainrecommendation.Candidate, error) {
	if len(pool) == 0 {
		return []*domainrecommendation.Candidate{}, nil
	}

	videoIDs := make([]int64, 0, len(pool))
	for _, candidate := range pool {
		if candidate != nil && candidate.VideoID > 0 {
			videoIDs = append(videoIDs, candidate.VideoID)
		}
	}
	vectors, err := s.repo.LoadVideoVectors(ctx, videoIDs)
	if err != nil {
		return nil, err
	}
	userVector, hasUserVector, err := s.repo.LoadUserInterestVector(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := s.now()
	ranked := make([]*domainrecommendation.Candidate, 0, len(pool))
	for _, candidate := range pool {
		if candidate == nil {
			continue
		}
		value := *candidate
		value.FreshnessScore = freshnessScore(now, value.PublishedAt)
		value.Similarity = 0
		if hasUserVector {
			if vector := vectors[value.VideoID]; len(vector) > 0 {
				similarity, err := domainembedding.CosineSimilarity(userVector, vector)
				if err == nil {
					value.Similarity = similarity
				}
			}
		}
		value.RankScore = rankScore(value.Similarity, value.HotScore, value.FreshnessScore, hasUserVector)
		value.Reason = recommendationReason(hasUserVector, value.Similarity, value.HotScore)
		ranked = append(ranked, &value)
	}

	sortCandidates(ranked)
	return ranked, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > domainrecommendation.MaxLimit {
		return domainrecommendation.MaxLimit
	}
	return limit
}

func candidatePoolLimit(limit int) int {
	poolLimit := limit * candidatePoolMultiplier
	if poolLimit < minCandidatePoolSize {
		poolLimit = minCandidatePoolSize
	}
	if poolLimit > maxCandidatePoolSize {
		poolLimit = maxCandidatePoolSize
	}
	return poolLimit
}

func rankScore(similarity float64, hotScore int, freshness float64, hasUserVector bool) float64 {
	hot := math.Log1p(float64(maxInt(hotScore, 0))) / 10
	if hasUserVector {
		return similarity*0.70 + hot*0.20 + freshness*0.10
	}
	return hot*0.65 + freshness*0.35
}

func freshnessScore(now time.Time, publishedAt time.Time) float64 {
	if publishedAt.IsZero() {
		return 0
	}
	hours := now.Sub(publishedAt).Hours()
	if hours < 0 {
		hours = 0
	}
	return 1 / (1 + hours/72)
}

func recommendationReason(hasUserVector bool, similarity float64, hotScore int) string {
	if hasUserVector && similarity > 0.05 {
		return "interest_match"
	}
	if hotScore > 0 {
		return "hot"
	}
	return "fresh"
}

func sortCandidates(candidates []*domainrecommendation.Candidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.RankScore != right.RankScore {
			return left.RankScore > right.RankScore
		}
		if !left.PublishedAt.Equal(right.PublishedAt) {
			return left.PublishedAt.After(right.PublishedAt)
		}
		return left.VideoID > right.VideoID
	})
}

func interleaveByAuthor(candidates []*domainrecommendation.Candidate) []*domainrecommendation.Candidate {
	if len(candidates) <= 2 {
		return candidates
	}
	output := make([]*domainrecommendation.Candidate, 0, len(candidates))
	delayed := make([]*domainrecommendation.Candidate, 0)
	var previousAuthorID int64
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if previousAuthorID != 0 && candidate.AuthorID == previousAuthorID {
			delayed = append(delayed, candidate)
			continue
		}
		output = append(output, candidate)
		previousAuthorID = candidate.AuthorID
	}
	for len(delayed) > 0 {
		progressed := false
		remaining := make([]*domainrecommendation.Candidate, 0)
		for _, candidate := range delayed {
			if len(output) > 0 && output[len(output)-1].AuthorID == candidate.AuthorID {
				remaining = append(remaining, candidate)
				continue
			}
			output = append(output, candidate)
			progressed = true
		}
		if !progressed {
			output = append(output, remaining...)
			break
		}
		delayed = remaining
	}
	return output
}

func filterByCursor(candidates []*domainrecommendation.Candidate, cursor *domainrecommendation.Cursor) []*domainrecommendation.Candidate {
	if cursor == nil {
		return candidates
	}
	filtered := make([]*domainrecommendation.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if candidate.RankScore < cursor.RankScore ||
			(sameScore(candidate.RankScore, cursor.RankScore) && candidate.PublishedAt.Before(cursor.PublishedAt)) ||
			(sameScore(candidate.RankScore, cursor.RankScore) && candidate.PublishedAt.Equal(cursor.PublishedAt) && candidate.VideoID < cursor.VideoID) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func sameScore(left float64, right float64) bool {
	return math.Abs(left-right) < 0.000000001
}

func parseCursor(raw string) (*domainrecommendation.Cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	content, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		content, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, domainrecommendation.ErrInvalidCursor
		}
	}
	var payload cursorPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, domainrecommendation.ErrInvalidCursor
	}
	publishedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.PublishedAt))
	if err != nil {
		return nil, domainrecommendation.ErrInvalidCursor
	}
	cursor := &domainrecommendation.Cursor{
		RankScore:   payload.RankScore,
		PublishedAt: publishedAt,
		VideoID:     payload.VideoID,
	}
	if !cursor.Valid() {
		return nil, domainrecommendation.ErrInvalidCursor
	}
	return cursor, nil
}

func encodeCursor(cursor *domainrecommendation.Cursor) string {
	if cursor == nil || !cursor.Valid() {
		return ""
	}
	content, err := json.Marshal(cursorPayload{
		RankScore:   cursor.RankScore,
		PublishedAt: cursor.PublishedAt.UTC().Format(time.RFC3339Nano),
		VideoID:     cursor.VideoID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
