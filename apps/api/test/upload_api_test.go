package test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	interfaceshttpupload "GCFeed/internal/interfaces/http/upload"

	"github.com/gin-gonic/gin"
)

type stubUploadProcessor struct {
	validateCalls  int
	faststartCalls int
}

func (p *stubUploadProcessor) ValidateVideo(ctx context.Context, path string) error {
	p.validateCalls++
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

func (p *stubUploadProcessor) Faststart(ctx context.Context, path string) error {
	p.faststartCalls++
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte("faststart:"), data...), 0o644)
}

func TestUploadVideoValidationAndFaststart(t *testing.T) {
	router, root, processor := newUploadRouter(t)

	resp := performMultipartUpload(router, "/api/uploads", "video", "clip.mp4", sampleMP4Bytes())
	requireStatus(t, resp, http.StatusCreated)

	if processor.validateCalls != 1 || processor.faststartCalls != 1 {
		t.Fatalf("expected video validate and faststart once, got validate=%d faststart=%d", processor.validateCalls, processor.faststartCalls)
	}

	var payload struct {
		URL      string `json:"url"`
		Kind     string `json:"kind"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	decodeJSON(t, resp, &payload)
	if payload.Kind != "video" || filepath.Ext(payload.Filename) != ".mp4" {
		t.Fatalf("unexpected upload response: %+v", payload)
	}

	createdPath := filepath.Join(root, "video", payload.Filename)
	data, err := os.ReadFile(createdPath)
	if err != nil {
		t.Fatalf("read uploaded video: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("faststart:")) {
		t.Fatalf("expected faststart processor to rewrite video")
	}
}

func TestUploadValidationRejectsBadFiles(t *testing.T) {
	router, _, processor := newUploadRouter(t)

	badExt := performMultipartUpload(router, "/api/uploads", "video", "clip.exe", sampleMP4Bytes())
	requireStatus(t, badExt, http.StatusBadRequest)

	badMime := performMultipartUpload(router, "/api/uploads", "cover", "cover.jpg", []byte("plain text"))
	requireStatus(t, badMime, http.StatusBadRequest)

	cover := performMultipartUpload(router, "/api/uploads", "cover", "cover.png", samplePNGBytes())
	requireStatus(t, cover, http.StatusCreated)

	if processor.validateCalls != 0 || processor.faststartCalls != 0 {
		t.Fatalf("expected image uploads to skip video processor, got validate=%d faststart=%d", processor.validateCalls, processor.faststartCalls)
	}
}

func newUploadRouter(t *testing.T) (*gin.Engine, string, *stubUploadProcessor) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	processor := &stubUploadProcessor{}
	handler := interfaceshttpupload.NewWithProcessor(root, processor)

	router := gin.New()
	api := router.Group("/api")
	api.POST("/uploads", handler.Create)
	return router, root, processor
}

func performMultipartUpload(router *gin.Engine, path string, kind string, filename string, content []byte) *httptest.ResponseRecorder {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("kind", kind)
	part, _ := writer.CreateFormFile("file", filename)
	_, _ = io.Copy(part, bytes.NewReader(content))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func sampleMP4Bytes() []byte {
	data := make([]byte, 0, 128)
	data = append(data, 0x00, 0x00, 0x00, 0x18)
	data = append(data, []byte("ftypmp42")...)
	data = append(data, 0x00, 0x00, 0x00, 0x00)
	data = append(data, []byte("mp42isom")...)
	data = append(data, bytes.Repeat([]byte{0x00}, 80)...)
	return data
}

func samplePNGBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
	}
}
