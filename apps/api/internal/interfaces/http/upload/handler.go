package interfaceshttpupload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxUploadBytes = 1024 << 20

// Handler 管理本地上传目录，当前项目把文件保存到 ./uploads。
type Handler struct {
	root string
}

// uploadResponse 是上传成功后返回给前端的文件访问地址和元信息。
type uploadResponse struct {
	URL      string `json:"url"`
	Kind     string `json:"kind"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// New 创建上传 Handler，root 是文件保存根目录。
func New(root string) *Handler {
	return &Handler{root: root}
}

// Create 接收 multipart/form-data 文件，并按 kind 保存到不同子目录。
func (h *Handler) Create(c *gin.Context) {
	// MaxBytesReader 在读取请求体前限制上传大小，避免大文件撑爆内存或磁盘。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	kind, ok := normalizeKind(c.PostForm("kind"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload kind"})
		return
	}

	ext := strings.ToLower(filepath.Ext(filepath.Base(file.Filename)))
	if ext == "" {
		ext = ".bin"
	}

	// 文件名加入时间戳和随机后缀，降低不同用户上传同名文件的冲突概率。
	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), randomSuffix(), ext)
	targetDir := filepath.Join(h.root, kind)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare upload directory"})
		return
	}

	targetPath := filepath.Join(targetDir, filename)
	if err := c.SaveUploadedFile(file, targetPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
		return
	}

	c.JSON(http.StatusCreated, uploadResponse{
		URL:      "/uploads/" + kind + "/" + filename,
		Kind:     kind,
		Filename: filename,
		Size:     file.Size,
	})
}

// normalizeKind 规范化文件分类，分类会影响保存目录和访问 URL。
func normalizeKind(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "video":
		return "video", true
	case "cover":
		return "cover", true
	case "avatar":
		return "avatar", true
	case "":
		return "file", true
	case "file":
		return "file", true
	default:
		return "", false
	}
}

// randomSuffix 生成文件名随机后缀，随机失败时用时间戳兜底。
func randomSuffix() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
