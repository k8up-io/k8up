package monitoring

import (
	"net/http"
	"sync"
	"time"

	"git.vshn.net/vshn/baas/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	applogger "github.com/spotahome/kooper/log"
)

// Interface check for MetricExporter
var _ MonitorEndpoint = (*MetricExporter)(nil)

// MetricExporter holds the web endpoint for prometheus
// this implements the interface MonitorEndpoint
type MetricExporter struct {
	log        log.Logger
	httpServer *http.Server
	mutex      *sync.Mutex
}

// MonitorEndpoint defines the monitoring handler
type MonitorEndpoint interface {
	Register(collector prometheus.Collector)
}

// Holds the singleton instance for the exporter
var instance *MetricExporter
var once sync.Once

func init() {
	viper.SetDefault("metricbind", ":8080")
}

// GetInstance initializes the instance exactly once and returns it
func GetInstance() *MetricExporter {

	once.Do(func() {
		instance = new()
	})
	return instance
}

func new() *MetricExporter {
	m := mux.NewRouter()
	m.Handle("/metrics", promhttp.Handler())

	tmp := &MetricExporter{
		log: &applogger.Std{},
		httpServer: &http.Server{
			Addr:           viper.GetString("metricbind"),
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Handler:        m,
		},
		mutex: &sync.Mutex{},
	}
	tmp.log.Infof("Starting prometheus endpoint")
	go func() {
		err := tmp.httpServer.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
	return tmp
}

// Register registers a prometheus collector
func (m *MetricExporter) Register(collector prometheus.Collector) {
	m.mutex.Lock()
	prometheus.MustRegister(collector)
	m.mutex.Unlock()
}
