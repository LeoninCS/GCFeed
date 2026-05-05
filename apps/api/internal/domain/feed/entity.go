package domainfeed

import (
	"strings"
	"time"
)

const MaxLimit = 100

// Scene 表示不同 Feed 场景，应用层通过场景选择对应策略。
type Scene string

const (
	SceneTimeline  Scene = "timeline"
	SceneRecommend Scene = "recommend"
	SceneFollowing Scene = "following"
	SceneHot       Scene = "hot"

	DefaultScene = SceneTimeline
)

// FeedItem 是 Feed 页面需要展示的一条视频卡片数据。
type FeedItem struct {
	VideoID         int64
	AuthorID        int64
	AuthorNickname  string
	AuthorAvatarURL string
	Title           string
	Description     string
	MediaURL        string
	CoverURL        string
	LikeCount       int
	CommentCount    int
	FavoriteCount   int
	PublishedAt     time.Time
}

// TimelineCursor 保存时间线分页所需的排序字段。
type TimelineCursor struct {
	PublishedAt time.Time
	VideoID     int64
}

// NormalizeScene 统一 scene 参数格式，空值使用默认 Feed 场景。
func NormalizeScene(scene Scene) Scene {
	value := strings.TrimSpace(strings.ToLower(string(scene)))
	if value == "" {
		return DefaultScene
	}
	return Scene(value)
}

// RestoreFeedItem 从查询结果恢复 FeedItem，并清洗展示用字符串。
func RestoreFeedItem(videoID int64, authorID int64, authorNickname string, authorAvatarURL string, title string, description string, mediaURL string, coverURL string, likeCount int, commentCount int, favoriteCount int, publishedAt time.Time) *FeedItem {
	return &FeedItem{
		VideoID:         videoID,
		AuthorID:        authorID,
		AuthorNickname:  strings.TrimSpace(authorNickname),
		AuthorAvatarURL: strings.TrimSpace(authorAvatarURL),
		Title:           strings.TrimSpace(title),
		Description:     strings.TrimSpace(description),
		MediaURL:        strings.TrimSpace(mediaURL),
		CoverURL:        strings.TrimSpace(coverURL),
		LikeCount:       likeCount,
		CommentCount:    commentCount,
		FavoriteCount:   favoriteCount,
		PublishedAt:     publishedAt,
	}
}
