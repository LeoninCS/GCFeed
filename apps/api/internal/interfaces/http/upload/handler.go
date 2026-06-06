package interfaceshttpupload

import (
	inframetrics "GCFeed/internal/infra/metrics"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxUploadBytes = 1024 << 20
const maxVideoBytes = 512 << 20
const maxImageBytes = 20 << 20
const maxVideoDurationSeconds = 10 * 60
const maxVideoDimension = 3840
const sniffBytes = 512

var allowedVideoExt = map[string]struct{}{
	".mp4":  {},
	".mov":  {},
	".webm": {},
}

var allowedImageExt = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".webp": {},
}

var allowedVideoMIME = map[string]struct{}{
	"application/octet-stream": {},
	"video/mp4":                {},
	"video/quicktime":          {},
	"video/webm":               {},
}

var allowedImageMIME = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

var allowedVideoCodec = map[string]struct{}{
	"h264": {},
	"h265": {},
	"hevc": {},
	"vp8":  {},
	"vp9":  {},
	"av1":  {},
}

var allowedAudioCodec = map[string]struct{}{
	"aac":    {},
	"mp3":    {},
	"opus":   {},
	"vorbis": {},
}

var errInvalidUploadExtension = errors.New("unsupported upload extension")
var errInvalidUploadMIME = errors.New("unsupported upload content type")
var errUploadTooLarge = errors.New("upload file is too large")
var errVideoTooLong = errors.New("video duration is too long")
var errVideoTooLargeDimension = errors.New("video resolution is too large")
var errUnsupportedVideoCodec = errors.New("video codec is unsupported")
var errInvalidVideoMetadata = errors.New("video metadata is invalid")
var errVideoToolUnavailable = errors.New("video tool is unavailable")
var errFaststartFailed = errors.New("video faststart failed")

// UploadProcessor 负责视频元数据校验和 MP4 faststart 处理。
type UploadProcessor interface {
	ValidateVideo(ctx context.Context, path string) error
	Faststart(ctx context.Context, path string) error
}

// Handler 管理本地上传目录，当前项目把文件保存到 ./uploads。
type Handler struct {
	root      string
	processor UploadProcessor
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
	return NewWithProcessor(root, defaultUploadProcessor{})
}

// NewWithProcessor 创建可注入视频处理器的上传 Handler，便于测试和替换实现。
func NewWithProcessor(root string, processor UploadProcessor) *Handler {
	if processor == nil {
		processor = defaultUploadProcessor{}
	}
	return &Handler{root: root, processor: processor}
}

// Create 接收 multipart/form-data 文件，并按 kind 保存到不同子目录。
func (h *Handler) Create(c *gin.Context) {
	start := time.Now()
	kind := "unknown"
	var resultErr error
	defer func() {
		inframetrics.ObserveUpload(kind, time.Since(start), resultErr)
	}()

	// MaxBytesReader 在读取请求体前限制上传大小，避免大文件撑爆内存或磁盘。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	file, err := c.FormFile("file")
	if err != nil {
		resultErr = err
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	normalizedKind, ok := normalizeKind(c.PostForm("kind"))
	if !ok {
		resultErr = errors.New("invalid upload kind")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload kind"})
		return
	}
	kind = normalizedKind

	validation, err := validateUploadFile(file, kind)
	if err != nil {
		resultErr = err
		writeUploadValidationError(c, err)
		return
	}

	// 文件名加入时间戳和随机后缀，降低不同用户上传同名文件的冲突概率。
	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), randomSuffix(), validation.Ext)
	targetDir := filepath.Join(h.root, kind)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		resultErr = err
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare upload directory"})
		return
	}

	targetPath := filepath.Join(targetDir, filename)
	if err := c.SaveUploadedFile(file, targetPath); err != nil {
		resultErr = err
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
		return
	}

	if kind == "video" {
		if err := h.processor.ValidateVideo(c.Request.Context(), targetPath); err != nil {
			resultErr = err
			_ = os.Remove(targetPath)
			writeUploadProcessingError(c, err)
			return
		}
		if shouldFaststart(validation.Ext) {
			if err := h.processor.Faststart(c.Request.Context(), targetPath); err != nil {
				resultErr = err
				_ = os.Remove(targetPath)
				writeUploadProcessingError(c, err)
				return
			}
		}
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

type uploadValidation struct {
	Ext  string
	MIME string
}

func validateUploadFile(file *multipart.FileHeader, kind string) (*uploadValidation, error) {
	if err := validateUploadSize(file.Size, kind); err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(filepath.Base(file.Filename)))
	if ext == "" {
		if kind == "file" {
			ext = ".bin"
		} else {
			return nil, errInvalidUploadExtension
		}
	}

	if err := validateUploadExtension(ext, kind); err != nil {
		return nil, err
	}
	if kind == "file" {
		return &uploadValidation{Ext: ext}, nil
	}

	mimeType, err := sniffUploadMIME(file)
	if err != nil {
		return nil, err
	}
	if err := validateUploadMIME(mimeType, kind); err != nil {
		return nil, err
	}

	return &uploadValidation{Ext: ext, MIME: mimeType}, nil
}

func validateUploadSize(size int64, kind string) error {
	if size <= 0 {
		return errInvalidUploadMIME
	}
	if kind == "video" && size > maxVideoBytes {
		return errUploadTooLarge
	}
	if (kind == "cover" || kind == "avatar") && size > maxImageBytes {
		return errUploadTooLarge
	}
	if size > maxUploadBytes {
		return errUploadTooLarge
	}
	return nil
}

func validateUploadExtension(ext string, kind string) error {
	switch kind {
	case "video":
		if _, ok := allowedVideoExt[ext]; !ok {
			return errInvalidUploadExtension
		}
	case "cover", "avatar":
		if _, ok := allowedImageExt[ext]; !ok {
			return errInvalidUploadExtension
		}
	}
	return nil
}

func sniffUploadMIME(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", errInvalidUploadMIME
	}
	defer file.Close()

	header := make([]byte, sniffBytes)
	n, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", errInvalidUploadMIME
	}
	if n == 0 {
		return "", errInvalidUploadMIME
	}
	return http.DetectContentType(header[:n]), nil
}

func validateUploadMIME(mimeType string, kind string) error {
	switch kind {
	case "video":
		if _, ok := allowedVideoMIME[mimeType]; !ok {
			return errInvalidUploadMIME
		}
	case "cover", "avatar":
		if _, ok := allowedImageMIME[mimeType]; !ok {
			return errInvalidUploadMIME
		}
	}
	return nil
}

func shouldFaststart(ext string) bool {
	ext = strings.ToLower(strings.TrimSpace(ext))
	return ext == ".mp4" || ext == ".mov"
}

type defaultUploadProcessor struct{}

func (defaultUploadProcessor) ValidateVideo(ctx context.Context, path string) error {
	start := time.Now()
	metadata, err := probeVideo(ctx, path)
	if err != nil {
		inframetrics.ObserveVideoProcessing("probe", time.Since(start), err)
		return err
	}
	err = validateVideoMetadata(metadata)
	inframetrics.ObserveVideoProcessing("probe", time.Since(start), err)
	return err
}

func (defaultUploadProcessor) Faststart(ctx context.Context, path string) error {
	start := time.Now()
	err := faststartVideo(ctx, path)
	inframetrics.ObserveVideoProcessing("faststart", time.Since(start), err)
	return err
}

type probeResult struct {
	Streams []probeStream `json:"streams"`
	Format  probeFormat   `json:"format"`
}

type probeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type probeFormat struct {
	Duration string `json:"duration"`
}

func probeVideo(ctx context.Context, path string) (*probeResult, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		probeCtx,
		"ffprobe",
		"-v",
		"error",
		"-show_entries",
		"stream=codec_type,codec_name,width,height:format=duration",
		"-of",
		"json",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errVideoToolUnavailable
		}
		return nil, errInvalidVideoMetadata
	}

	var result probeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, errInvalidVideoMetadata
	}
	return &result, nil
}

func validateVideoMetadata(metadata *probeResult) error {
	if metadata == nil {
		return errInvalidVideoMetadata
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(metadata.Format.Duration), 64)
	if err != nil || duration <= 0 {
		return errInvalidVideoMetadata
	}
	if duration > maxVideoDurationSeconds {
		return errVideoTooLong
	}

	var hasVideo bool
	for _, stream := range metadata.Streams {
		codecType := strings.ToLower(strings.TrimSpace(stream.CodecType))
		codecName := strings.ToLower(strings.TrimSpace(stream.CodecName))
		switch codecType {
		case "video":
			hasVideo = true
			if stream.Width <= 0 || stream.Height <= 0 {
				return errInvalidVideoMetadata
			}
			if stream.Width > maxVideoDimension || stream.Height > maxVideoDimension {
				return errVideoTooLargeDimension
			}
			if _, ok := allowedVideoCodec[codecName]; !ok {
				return errUnsupportedVideoCodec
			}
		case "audio":
			if codecName != "" {
				if _, ok := allowedAudioCodec[codecName]; !ok {
					return errUnsupportedVideoCodec
				}
			}
		}
	}
	if !hasVideo {
		return errInvalidVideoMetadata
	}
	return nil
}

func faststartVideo(ctx context.Context, path string) error {
	tmp, err := createFaststartTempFile(path)
	if err != nil {
		return errFaststartFailed
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	ffmpegCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ffmpegCtx,
		"ffmpeg",
		"-y",
		"-i",
		path,
		"-map",
		"0",
		"-c",
		"copy",
		"-movflags",
		"+faststart",
		"-f",
		"mp4",
		tmpPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errVideoToolUnavailable
		}
		return fmt.Errorf("%w: %s", errFaststartFailed, strings.TrimSpace(stderr.String()))
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return errFaststartFailed
	}
	return nil
}

func createFaststartTempFile(path string) (*os.File, error) {
	targetDir := filepath.Dir(path)
	return os.CreateTemp(targetDir, "*.faststart.mp4")
}

func writeUploadValidationError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func writeUploadProcessingError(c *gin.Context, err error) {
	if errors.Is(err, errVideoToolUnavailable) || errors.Is(err, errFaststartFailed) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

// randomSuffix 生成文件名随机后缀，随机失败时用时间戳兜底。
func randomSuffix() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
