package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net"
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
	// 同时兼容根路径与 /static/ 前缀：前端资源引用任一写法都能命中，
	// 避免路径不一致导致样式/脚本 404、桌面窗口白屏。
	mux.Handle("/", staticHandler(staticRoot))
	mux.Handle("/static/", http.StripPrefix("/static", staticHandler(staticRoot)))

	// 飞牛统一网关会把 /app/<appname>/ 前缀原样转发进来，这里剥掉前缀，
	// 让应用自身始终按根路径处理；同时把无尾斜杠的入口重定向到带斜杠，
	// 保证页面内相对路径（静态资源、API）解析正确。
	handler := withGatewayPrefix(requestLogger(mux), cfg.GatewayPrefix)

	server := &http.Server{
		Addr:              cfg.listenAddr(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)

	// 端口监听：局域网直连调试与端口访问方式。
	go func() {
		slog.Info("HTTP 监听", "addr", cfg.listenAddr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Unix Socket 监听：飞牛统一网关通过它转发，桌面入口即走这条通道，
	// 从而复用系统访问域名与登录态，彻底避开 https 页面嵌入 http 的混合内容拦截。
	if cfg.SocketPath != "" {
		_ = os.Remove(cfg.SocketPath)
		listener, err := net.Listen("unix", cfg.SocketPath)
		if err != nil {
			// Socket 建不起来不致命：端口方式仍可用，桌面图标则可能白屏。
			slog.Error("统一网关 Socket 监听失败，桌面入口可能不可用", "path", cfg.SocketPath, "err", err)
		} else {
			_ = os.Chmod(cfg.SocketPath, 0o666)
			slog.Info("统一网关 Socket 已就绪", "path", cfg.SocketPath, "prefix", cfg.GatewayPrefix)
			go func() {
				if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					slog.Error("Socket 服务异常退出", "err", err)
				}
			}()
		}
	}

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
	if cfg.SocketPath != "" {
		_ = os.Remove(cfg.SocketPath)
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

// withGatewayPrefix 适配飞牛统一网关的路径前缀。
//
// 网关会把发往 /app/<appname>/... 的请求原样转发给应用，因此这里剥掉前缀，
// 让内部路由始终按根路径匹配；同时把不带尾斜杠的入口重定向到带斜杠，
// 否则页面里的相对路径（static/... 、api/...）会被解析到父级目录而 404。
func withGatewayPrefix(next http.Handler, prefix string) http.Handler {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == prefix {
			target := prefix + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		if strings.HasPrefix(path, prefix+"/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimPrefix(path, prefix)
			next.ServeHTTP(w, r2)
			return
		}
		next.ServeHTTP(w, r)
	})
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
		// 飞牛桌面本身是独立源的 Web 应用（且常用非默认端口），
		// 显式允许被任意源 iframe 内嵌，避免桌面窗口白屏。
		w.Header().Set("Content-Security-Policy", "frame-ancestors *")
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
