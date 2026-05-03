package applicationvideo

import (
	domainvideo "GCFeed/internal/domain/video"
	"context"
	"errors"
	"strings"
)

var ErrLoadVideoFailed = errors.New("failed to load video")
var ErrSaveVideoFailed = errors.New("failed to save video")
var ErrUpdateVideoFailed = errors.New("failed to update video")

type Service struct {
	repo domainvideo.Repository
}

type CreateResult struct {
	Video   *domainvideo.Video
	Created bool
}

func NewService(repo domainvideo.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreatePublished(ctx context.Context, authorID int64, title, description, mediaURL, coverURL, idempotencyKey string) (*CreateResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > domainvideo.MaxIdempotencyKeyLength {
		return nil, domainvideo.ErrIdempotencyKeyTooLong
	}

	if idempotencyKey != "" {
		existing, err := s.repo.FindByAuthorAndIdempotencyKey(ctx, authorID, idempotencyKey)
		if err == nil {
			return &CreateResult{Video: existing, Created: false}, nil
		}
		if !errors.Is(err, domainvideo.ErrVideoNotFound) {
			return nil, ErrLoadVideoFailed
		}
	}

	video, err := domainvideo.NewPublished(authorID, title, description, mediaURL, coverURL, idempotencyKey)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, video); err != nil {
		if idempotencyKey != "" && errors.Is(err, domainvideo.ErrDuplicateIdempotencyKey) {
			existing, loadErr := s.repo.FindByAuthorAndIdempotencyKey(ctx, authorID, idempotencyKey)
			if loadErr == nil {
				return &CreateResult{Video: existing, Created: false}, nil
			}
			return nil, ErrLoadVideoFailed
		}
		return nil, ErrSaveVideoFailed
	}

	return &CreateResult{Video: video, Created: true}, nil
}

func (s *Service) Get(ctx context.Context, videoID int64) (*domainvideo.Video, error) {
	if videoID <= 0 {
		return nil, domainvideo.ErrInvalidVideoID
	}

	video, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		if errors.Is(err, domainvideo.ErrVideoNotFound) {
			return nil, domainvideo.ErrVideoNotFound
		}
		return nil, ErrLoadVideoFailed
	}
	return video, nil
}

func (s *Service) ListByAuthor(ctx context.Context, authorID int64, limit, offset int) ([]*domainvideo.Video, error) {
	if authorID <= 0 {
		return nil, domainvideo.ErrInvalidAuthorID
	}
	if limit <= 0 {
		return nil, domainvideo.ErrInvalidLimit
	}
	if offset < 0 {
		return nil, domainvideo.ErrInvalidOffset
	}
	if limit > 100 {
		limit = 100
	}

	videos, err := s.repo.ListByAuthor(ctx, authorID, limit, offset)
	if err != nil {
		return nil, ErrLoadVideoFailed
	}
	return videos, nil
}

func (s *Service) Delete(ctx context.Context, authorID, videoID int64) error {
	if authorID <= 0 {
		return domainvideo.ErrInvalidAuthorID
	}
	if videoID <= 0 {
		return domainvideo.ErrInvalidVideoID
	}

	video, err := s.repo.FindByIDAnyStatus(ctx, videoID)
	if err != nil {
		if errors.Is(err, domainvideo.ErrVideoNotFound) {
			return domainvideo.ErrVideoNotFound
		}
		return ErrLoadVideoFailed
	}
	alreadyDeleted := video.Status == domainvideo.StatusDeleted
	if err := video.DeleteBy(authorID); err != nil {
		return err
	}
	if alreadyDeleted {
		return nil
	}
	if err := s.repo.UpdateStatus(ctx, video); err != nil {
		if errors.Is(err, domainvideo.ErrVideoNotFound) {
			return domainvideo.ErrVideoNotFound
		}
		return ErrUpdateVideoFailed
	}
	return nil
}
