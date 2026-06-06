package inframetrics

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gcfeed",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests handled by the API.",
		},
		[]string{"method", "route", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gcfeed",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "route", "status"},
	)

	FeedRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gcfeed",
			Name:      "feed_requests_total",
			Help:      "Total feed requests by scene and result.",
		},
		[]string{"scene", "result"},
	)

	FeedRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gcfeed",
			Name:      "feed_request_duration_seconds",
			Help:      "Feed request duration in seconds by scene.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"scene", "result"},
	)

	FeedItemsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gcfeed",
			Name:      "feed_items_total",
			Help:      "Total feed items returned by scene.",
		},
		[]string{"scene"},
	)

	FeedCacheRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gcfeed",
			Name:      "feed_cache_requests_total",
			Help:      "Feed cache reads by cache area and result.",
		},
		[]string{"area", "result"},
	)

	FeedCacheWritesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gcfeed",
			Name:      "feed_cache_writes_total",
			Help:      "Feed cache writes by cache area and result.",
		},
		[]string{"area", "result"},
	)

	VideoUploadTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gcfeed",
			Name:      "video_upload_total",
			Help:      "Upload requests by kind and result.",
		},
		[]string{"kind", "result"},
	)

	VideoUploadDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gcfeed",
			Name:      "video_upload_duration_seconds",
			Help:      "Upload request processing duration in seconds.",
			Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"kind", "result"},
	)

	VideoProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gcfeed",
			Name:      "video_processing_duration_seconds",
			Help:      "Video processing step duration in seconds.",
			Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"step", "result"},
	)

	WorkerJobsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "gcfeed",
			Name:      "worker_jobs_total",
			Help:      "Worker jobs handled by job name and result.",
		},
		[]string{"job", "result"},
	)

	WorkerJobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "gcfeed",
			Name:      "worker_job_duration_seconds",
			Help:      "Worker job processing duration in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"job", "result"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		FeedRequestsTotal,
		FeedRequestDuration,
		FeedItemsTotal,
		FeedCacheRequestsTotal,
		FeedCacheWritesTotal,
		VideoUploadTotal,
		VideoUploadDuration,
		VideoProcessingDuration,
		WorkerJobsTotal,
		WorkerJobDuration,
	)
}

// HTTPMiddleware records request count and latency with stable route labels.
func HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		duration := time.Since(start).Seconds()

		HTTPRequestsTotal.WithLabelValues(method, route, status).Inc()
		HTTPRequestDuration.WithLabelValues(method, route, status).Observe(duration)
	}
}

func Handler() http.Handler {
	return promhttp.Handler()
}

func RunServer(ctx context.Context, addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: Handler(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func ObserveFeed(scene string, duration time.Duration, itemCount int, err error) {
	scene = normalizeLabel(scene, "unknown")
	result := resultLabel(err)
	FeedRequestsTotal.WithLabelValues(scene, result).Inc()
	FeedRequestDuration.WithLabelValues(scene, result).Observe(duration.Seconds())
	if err == nil && itemCount > 0 {
		FeedItemsTotal.WithLabelValues(scene).Add(float64(itemCount))
	}
}

func ObserveCacheRead(area string, requested int, hit int, err error) {
	area = normalizeLabel(area, "unknown")
	if err != nil {
		FeedCacheRequestsTotal.WithLabelValues(area, "error").Inc()
		return
	}
	if hit > 0 {
		FeedCacheRequestsTotal.WithLabelValues(area, "hit").Add(float64(hit))
	}
	miss := requested - hit
	if miss > 0 {
		FeedCacheRequestsTotal.WithLabelValues(area, "miss").Add(float64(miss))
	}
}

func ObserveCacheWrite(area string, count int, err error) {
	area = normalizeLabel(area, "unknown")
	if count <= 0 {
		count = 1
	}
	FeedCacheWritesTotal.WithLabelValues(area, resultLabel(err)).Add(float64(count))
}

func ObserveUpload(kind string, duration time.Duration, err error) {
	kind = normalizeLabel(kind, "unknown")
	result := resultLabel(err)
	VideoUploadTotal.WithLabelValues(kind, result).Inc()
	VideoUploadDuration.WithLabelValues(kind, result).Observe(duration.Seconds())
}

func ObserveVideoProcessing(step string, duration time.Duration, err error) {
	step = normalizeLabel(step, "unknown")
	VideoProcessingDuration.WithLabelValues(step, resultLabel(err)).Observe(duration.Seconds())
}

func ObserveWorkerJob(job string, duration time.Duration, err error) {
	job = normalizeLabel(job, "unknown")
	result := resultLabel(err)
	WorkerJobsTotal.WithLabelValues(job, result).Inc()
	WorkerJobDuration.WithLabelValues(job, result).Observe(duration.Seconds())
}

func resultLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func normalizeLabel(value string, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fallback
	}
	return value
}
