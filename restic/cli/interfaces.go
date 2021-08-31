package cli

import "github.com/prometheus/client_golang/prometheus"

type StatsHandler interface {
	SendPrometheus(PrometheusProvider) error
	SendWebhook(WebhookProvider) error
}

type PrometheusProvider interface {
	ToProm() []prometheus.Collector
}

type WebhookProvider interface {
	ToJSON() []byte
}
