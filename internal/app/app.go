// Package app собирает все компоненты сервиса уведомлений и управляет его жизненным циклом.
package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/pprof"
	"sync"

	"github.com/go-highload-demo/internal/broker"
	"github.com/go-highload-demo/internal/config"
	"github.com/go-highload-demo/internal/handler"
	"github.com/go-highload-demo/internal/model"
	"github.com/go-highload-demo/internal/sender"
	"github.com/go-highload-demo/internal/service"
)

// App объединяет все компоненты сервиса и управляет HTTP-сервером и брокером.
type App struct {
	cfg      *config.Config
	server   *http.Server
	svc      *service.NotificationService
	brk      broker.Broker
	stopOnce sync.Once
	stopErr  error
}

// New создаёт новый App, связывая конфигурацию, репозиторий, rate limiter
// и брокер с бизнес-логикой, обработчиками и HTTP-маршрутами.
func New(cfg *config.Config, repo service.Repository, limiter service.RateLimiter, brk broker.Broker) *App {
	svc := service.New(repo, limiter, brk)

	svc.RegisterSender(model.ChannelEmail, sender.NewEmailSender())
	svc.RegisterSender(model.ChannelPush, sender.NewPushSender())
	svc.RegisterSender(model.ChannelSMS, sender.NewSMSSender())
	svc.RegisterSender(model.ChannelWebhook, sender.NewWebhookSender())

	h := handler.New(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /notifications", h.CreateNotification)
	mux.HandleFunc("GET /notifications/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		h.GetNotification(w, r, id)
	})

	// Health check эндпоинты
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// pprof эндпоинты для профилирования
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	return &App{
		cfg: cfg,
		server: &http.Server{
			Addr:    cfg.Server.Addr,
			Handler: mux,
		},
		svc: svc,
		brk: brk,
	}
}

// Start запускает брокер и HTTP-сервер, возвращает адрес.
func (a *App) Start() (string, error) {
	if err := a.brk.Run(context.Background(), a.svc.ProcessNotification); err != nil {
		return "", err
	}

	ln, err := net.Listen("tcp", a.cfg.Server.Addr)
	if err != nil {
		return "", err
	}
	go a.server.Serve(ln)
	return ln.Addr().String(), nil
}

// Shutdown выполняет graceful shutdown HTTP-сервера и брокера.
// Безопасен для повторного вызова.
func (a *App) Shutdown(ctx context.Context) error {
	a.stopOnce.Do(func() {
		a.stopErr = a.server.Shutdown(ctx)
		a.brk.Shutdown()
	})
	return a.stopErr
}
