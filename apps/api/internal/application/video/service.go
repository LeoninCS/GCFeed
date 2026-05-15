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
	repo      domainvideo.Repository
	publisher PublishedEventPublisher
}

type PublishedEventPublisher interface {
	PublishVideoPublished(ctx context.Context, event *PublishedEvent) error
}

type Option func(*Service)

// CreateResult 同时表达“返回哪个视频”和“这次请求是否真的创建了新记录”。
type CreateResult struct {
	Video   *domainvideo.Video
	Created bool
}

func New(repo domainvideo.Repository, options ...Option) *Service {
	service := &Service{repo: repo}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithPublishedEventPublisher(publisher PublishedEventPublisher) Option {
	return func(s *Service) {
		s.publisher = publisher
	}
}

// CreatePublished 创建已发布视频；Idempotency-Key 命中时返回已有视频。
func (s *Service) CreatePublished(ctx context.Context, authorID int64, title, description, mediaURL, coverURL, idempotencyKey string) (*CreateResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > domainvideo.MaxIdempotencyKeyLength {
		return nil, domainvideo.ErrIdempotencyKeyTooLong
	}

	if idempotencyKey != "" {
		// 客户端重试同一次创建请求时，先通过作者和幂等键找回原视频。
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
			// 并发创建可能先查不到、后插入冲突；冲突后再查一次即可返回一致结果。
			existing, loadErr := s.repo.FindByAuthorAndIdempotencyKey(ctx, authorID, idempotencyKey)
			if loadErr == nil {
				return &CreateResult{Video: existing, Created: false}, nil
			}
			return nil, ErrLoadVideoFailed
		}
		return nil, ErrSaveVideoFailed
	}
	s.publishCreatedVideo(ctx, video)

	return &CreateResult{Video: video, Created: true}, nil
}

func (s *Service) publishCreatedVideo(ctx context.Context, video *domainvideo.Video) {
	if s.publisher == nil {
		return
	}
	event := NewPublishedEvent(video)
	if event == nil {
		return
	}
	_ = s.publisher.PublishVideoPublished(ctx, event)
}

// Get 只返回已发布视频，删除或下线的视频在公开详情里表现为找不到。
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

// ListByAuthor 查询某个作者公开发布的视频列表，使用 offset 分页。
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
		// 后端限制最大页大小，避免一次请求拉取过多数据。
		limit = 100
	}

	videos, err := s.repo.ListByAuthor(ctx, authorID, limit, offset)
	if err != nil {
		return nil, ErrLoadVideoFailed
	}
	return videos, nil
}

// Delete 执行视频软删除，只有作者本人可以删除自己的视频。
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
	// 软删除接口保持幂等：已经删除的视频再次删除仍然返回成功。
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
