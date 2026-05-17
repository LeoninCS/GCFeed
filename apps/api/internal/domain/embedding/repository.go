package domainembedding

import "context"

// Repository 定义 embedding 模块需要的持久化能力。
type Repository interface {
	SaveVideoEmbedding(ctx context.Context, embedding *VideoEmbedding) error
	FindVideoEmbedding(ctx context.Context, videoID int64, model string) (*VideoEmbedding, error)
}
