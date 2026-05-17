package domainexposure

import "context"

// Repository 定义曝光模块需要的持久化能力。
type Repository interface {
	SaveViewEvent(ctx context.Context, event *ViewEvent) (*ViewEvent, *Exposure, error)
}
