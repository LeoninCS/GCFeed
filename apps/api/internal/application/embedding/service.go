package applicationembedding

import (
	applicationvideo "GCFeed/internal/application/video"
	domainembedding "GCFeed/internal/domain/embedding"
	"context"
	"encoding/json"
	"errors"
)

var ErrSaveVideoEmbeddingFailed = errors.New("failed to save video embedding")
var ErrMarshalEmbeddingFailed = errors.New("failed to marshal embedding")

type Service struct {
	repo       domainembedding.Repository
	vectorizer domainembedding.Vectorizer
}

type GenerateVideoEmbeddingResult struct {
	Embedding        *domainembedding.VideoEmbedding
	CreatedOrUpdated bool
}

func New(repo domainembedding.Repository, vectorizer domainembedding.Vectorizer) *Service {
	if vectorizer == nil {
		vectorizer = domainembedding.NewHashNgramVectorizer()
	}
	return &Service{
		repo:       repo,
		vectorizer: vectorizer,
	}
}

// GenerateForPublishedVideo 根据视频发布事件生成并保存视频内容向量。
func (s *Service) GenerateForPublishedVideo(ctx context.Context, event *applicationvideo.PublishedEvent) (*GenerateVideoEmbeddingResult, error) {
	if event == nil || event.VideoID <= 0 {
		return &GenerateVideoEmbeddingResult{}, nil
	}

	text := domainembedding.BuildVideoText(event.Title, event.Description)
	vector := s.vectorizer.Vectorize(text)
	content, err := json.Marshal(vector)
	if err != nil {
		return nil, ErrMarshalEmbeddingFailed
	}

	embedding := domainembedding.NewVideoEmbedding(
		event.VideoID,
		s.vectorizer.Model(),
		vector,
		domainembedding.TextHash(text),
		string(content),
	)
	if err := s.repo.SaveVideoEmbedding(ctx, embedding); err != nil {
		return nil, ErrSaveVideoEmbeddingFailed
	}

	return &GenerateVideoEmbeddingResult{
		Embedding:        embedding,
		CreatedOrUpdated: true,
	}, nil
}
