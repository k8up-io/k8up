package stats

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_updatePrometheusFormat(t *testing.T) {
	tests := map[string]struct {
		format          string
		wantContentType string
		wantBody        string
	}{
		"default is protobuf": {
			wantContentType: "application/vnd.google.protobuf",
		},
		"protobuf": {
			format:          "protobuf",
			wantContentType: "application/vnd.google.protobuf",
		},
		"text": {
			format:          "text",
			wantContentType: "text/plain",
			wantBody:        "restic_backup_test_metric 42",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var contentType, body string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contentType = r.Header.Get("Content-Type")
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				body = string(b)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "restic_backup_test_metric"})
			gauge.Set(42)
			h := NewHandler(srv.URL, tt.format, "cluster", "host", "", logr.Discard())
			require.NoError(t, h.updatePrometheus(gauge))

			assert.True(t, strings.HasPrefix(contentType, tt.wantContentType), "unexpected content type %q", contentType)
			assert.Contains(t, body, tt.wantBody)
		})
	}
}
