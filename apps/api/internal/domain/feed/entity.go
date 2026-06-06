package domainfeed

import (
	"strings"
	"time"
)

const MaxLimit = 100

// BigCreatorFollowerThreshold 定义大 V 阈值，粉丝数达到该值的作者走关注流拉模式。
const BigCreatorFollowerThreshold = 10000

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
	Liked           bool
	Favorited       bool
	HotScore        int
	PublishedAt     time.Time
}

// FeedPageItem 是 Feed 页缓存中的轻量条目，只保存排序和组装所需字段。
type FeedPageItem struct {
	VideoID     int64
	AuthorID    int64
	PublishedAt time.Time
	HotScore    int
}

// FeedCard 保存视频卡片中相对稳定的展示字段。
type FeedCard struct {
	VideoID         int64
	AuthorID        int64
	AuthorNickname  string
	AuthorAvatarURL string
	Title           string
	Description     string
	MediaURL        string
	CoverURL        string
	PublishedAt     time.Time
}

// FeedStat 保存视频卡片中的高频计数字段。
type FeedStat struct {
	VideoID       int64
	LikeCount     int
	CommentCount  int
	FavoriteCount int
}

// ViewerActionState 保存当前用户对一批视频的互动状态。
type ViewerActionState struct {
	VideoID   int64
	Liked     bool
	Favorited bool
}

// TimelineCursor 保存时间线分页所需的排序字段。
type TimelineCursor struct {
	PublishedAt time.Time
	VideoID     int64
}

// HotCursor 保存热榜分页所需的排序字段。
type HotCursor struct {
	HotScore    int
	PublishedAt time.Time
	VideoID     int64
	WindowEnd   time.Time
	Offset      int
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
		HotScore:        ScoreHotFeedItem(likeCount, commentCount, favoriteCount),
		PublishedAt:     publishedAt,
	}
}

// ScoreHotFeedItem 计算热榜排序分：评论权重最高，收藏次之，点赞提供基础热度。
func ScoreHotFeedItem(likeCount int, commentCount int, favoriteCount int) int {
	return likeCount*3 + commentCount*5 + favoriteCount*4
}
