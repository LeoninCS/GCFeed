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

type Handler struct {
	root string
}

type uploadResponse struct {
	URL      string `json:"url"`
	Kind     string `json:"kind"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

func NewHandler(root string) *Handler {
	return &Handler{root: root}
}

func (h *Handler) Create(c *gin.Context) {
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

func randomSuffix() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
