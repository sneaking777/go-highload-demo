// Package app собирает все компоненты сервиса уведомлений и управляет его жизненным циклом.
package app

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/go-highload-demo/internal/config"
	"github.com/go-highload-demo/internal/handler"
	"github.com/go-highload-demo/internal/model"
	"github.com/go-highload-demo/internal/sender"
	"github.com/go-highload-demo/internal/service"
)

// App объединяет все компоненты сервиса и управляет HTTP-сервером и worker pool.
type App struct {
	cfg      *config.Config
	server   *http.Server
	svc      *service.NotificationService
	stopOnce sync.Once
	stopErr  error
}

// New создаёт новый App, связывая конфигурацию, репозиторий и rate limiter
// с бизнес-логикой, обработчиками и HTTP-маршрутами.
func New(cfg *config.Config, repo service.Repository, limiter service.RateLimiter) *App {
	svc := service.New(repo, limiter, cfg.Worker.PoolSize, cfg.Worker.QueueSize)

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

	return &App{
		cfg: cfg,
		server: &http.Server{
			Addr:    cfg.Server.Addr,
			Handler: mux,
		},
		svc: svc,
	}
}

// Start запускает worker pool и HTTP-сервер в отдельной горутине, возвращает адрес.
func (a *App) Start() (string, error) {
	a.svc.Start(context.Background())

	ln, err := net.Listen("tcp", a.cfg.Server.Addr)
	if err != nil {
		return "", err
	}
	go a.server.Serve(ln)
	return ln.Addr().String(), nil
}

// Shutdown выполняет graceful shutdown HTTP-сервера и worker pool.
// Безопасен для повторного вызова.
func (a *App) Shutdown(ctx context.Context) error {
	a.stopOnce.Do(func() {
		a.stopErr = a.server.Shutdown(ctx)
		a.svc.Stop()
	})
	return a.stopErr
}
