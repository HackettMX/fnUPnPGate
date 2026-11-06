package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// static 目录在前端构建时原样内嵌进二进制，因此运行时不需要任何外部文件。
//
//go:embed static
var embeddedStatic embed.FS

// version 由构建时通过 -ldflags "-X main.version=x.y.z" 注入。
var version = "dev"

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("配置无效，启动中止", "err", err)
		os.Exit(1)
	}
	setupLogger(cfg.LogLevel)

	slog.Info("启动 fnPortGate", "version", version, "addr", cfg.listenAddr(), "data_dir", cfg.DataDir)

	store := NewStore(cfg.configPath())
	manager := NewUPnPManager(cfg.DiscoveryTimeout, cfg.DiscoveryTTL)

	api := &API{cfg: cfg, store: store, mgr: manager, started: time.Now()}
	mux := http.NewServeMux()
	api.register(mux)

	staticRoot, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		slog.Error("内嵌前端资源不可用", "err", err)
		os.Exit(1)
	}
	mux.Handle("/", staticHandler(staticRoot))

	server := &http.Server{
		Addr:              cfg.listenAddr(),
		Handler:           requestLogger(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		slog.Error("HTTP 服务异常退出", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("收到退出信号，正在关闭服务")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Warn("关闭服务时出错", "err", err)
	}
	slog.Info("服务已退出")
}

func setupLogger(level string) {
	var lvl slog.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		lvl = slog.LevelDebug
	case "WARN", "WARNING":
		lvl = slog.LevelWarn
	case "ERROR":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}

// staticHandler 提供内嵌前端资源，并为入口页面加上禁用缓存的头，
// 保证升级后用户拿到的始终是新版本界面。
func staticHandler(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fileServer.ServeHTTP(w, r)
	})
}

// requestLogger 记录 API 请求的耗时，便于在 NAS 日志里定位路由器响应慢的问题。
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		slog.Debug("api",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"cost", time.Since(start).String(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
