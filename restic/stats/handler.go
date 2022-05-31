package stats

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"

	"github.com/k8up-io/k8up/v2/restic/cli"
)

const (
	subsystem = "restic_backup"
)

var _ cli.StatsHandler = &Handler{}

type Handler struct {
	promURL      string
	promHostname string
	webhookURL   string
	log          logr.Logger
}

func NewHandler(promURL, promHostname, webhookURL string, log logr.Logger) *Handler {
	return &Handler{
		promHostname: promHostname,
		promURL:      promURL,
		webhookURL:   webhookURL,
		log:          log.WithName("statsHandler"),
	}
}

func (h *Handler) SendPrometheus(promStats cli.PrometheusProvider) error {
	if h.promURL == "" {
		return nil
	}

	promLogger := h.log.WithName("promStats")

	promLogger.Info("sending prometheus stats", "url", h.promURL)

	for _, stat := range promStats.ToProm() {
		err := h.updatePrometheus(stat)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) updatePrometheus(collector prometheus.Collector) error {
	return push.New(h.promURL, subsystem).Collector(collector).
		Grouping("instance", h.promHostname).
		Add()
}

func (h *Handler) SendWebhook(hook cli.WebhookProvider) error {
	if h.webhookURL == "" {
		return nil
	}

	webhookLogger := h.log.WithName("webhookStats")

	webhookLogger.Info("sending webhooks", "url", h.webhookURL)

	data := hook.ToJSON()

	if len(data) <= 0 {
		return fmt.Errorf("webhook data is empty")
	}

	postBody := bytes.NewReader(data)

	resp, err := http.Post(h.webhookURL, "application/json", postBody)
	if err != nil || !strings.HasPrefix(resp.Status, "200") {
		httpCode := ""
		if resp == nil {
			httpCode = "http status unavailable"
		} else {
			httpCode = resp.Status
		}
		return fmt.Errorf("could not send webhook: %v http status code: %v", err, httpCode)
	}
	return nil
}
