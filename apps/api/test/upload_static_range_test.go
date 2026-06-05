package test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUploadStaticRangeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	uploadDir := t.TempDir()
	videoDir := filepath.Join(uploadDir, "video")
	if err := os.MkdirAll(videoDir, 0o755); err != nil {
		t.Fatalf("create video dir: %v", err)
	}

	content := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(videoDir, "range.mp4"), content, 0o644); err != nil {
		t.Fatalf("write test video: %v", err)
	}

	router := gin.New()
	router.Static("/uploads", uploadDir)

	req := httptest.NewRequest(http.MethodGet, "/uploads/video/range.mp4", nil)
	req.Header.Set("Range", "bytes=0-15")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	requireStatus(t, resp, http.StatusPartialContent)
	if got := resp.Header().Get("Content-Range"); got != "bytes 0-15/32" {
		t.Fatalf("expected content range bytes 0-15/32, got %q", got)
	}
	if got := resp.Body.String(); got != string(content[:16]) {
		t.Fatalf("expected ranged body %q, got %q", string(content[:16]), got)
	}
}
