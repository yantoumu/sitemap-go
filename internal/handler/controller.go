package handler

import (
	"context"
	"sitemap-go/internal/service"
)

type Controller struct {
	collector service.CollectorService
	processor service.ProcessorService
	storage   service.StorageService
	monitor   service.MonitorService
	notify    service.NotificationService
}

type ControllerConfig struct {
	BatchSize    int
	MaxWorkers   int
	TimeoutMs    int
	EnableNotify bool
}

type ControllerInterface interface {
	Start(ctx context.Context) error
	Stop() error
	ProcessSites(ctx context.Context, domains []string) error
	GetStatus(ctx context.Context) (*StatusResponse, error)
}

type StatusResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
	Health    map[string]bool        `json:"health"`
}

func NewController(
	collector service.CollectorService,
	processor service.ProcessorService,
	storage service.StorageService,
	monitor service.MonitorService,
	notify service.NotificationService,
) *Controller {
	return &Controller{
		collector: collector,
		processor: processor,
		storage:   storage,
		monitor:   monitor,
		notify:    notify,
	}
}