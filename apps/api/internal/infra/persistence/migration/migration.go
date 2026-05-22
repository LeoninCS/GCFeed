package migration

import (
	infraaccount "GCFeed/internal/infra/persistence/account"
	infraembedding "GCFeed/internal/infra/persistence/embedding"
	infraexposure "GCFeed/internal/infra/persistence/exposure"
	infrafeed "GCFeed/internal/infra/persistence/feed"
	infrainteraction "GCFeed/internal/infra/persistence/interaction"
	infrarelation "GCFeed/internal/infra/persistence/relation"
	infravideo "GCFeed/internal/infra/persistence/video"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&infraaccount.UserModel{},
		&infraembedding.VideoEmbeddingModel{},
		&infravideo.VideoModel{},
		&infravideo.VideoStatModel{},
		&infrafeed.InboxModel{},
		&infraexposure.ViewEventModel{},
		&infraexposure.ExposureModel{},
		&infrainteraction.ActionModel{},
		&infrainteraction.CommentModel{},
		&infrarelation.FollowModel{},
		&infrarelation.RelationStatModel{},
	); err != nil {
		return err
	}
	if err := infravideo.EnsureStats(db); err != nil {
		return err
	}
	return infrafeed.EnsureTimelineIndex(db)
}
