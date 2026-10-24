package cli

import (
	"encoding/json"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawMetrics_PrometheusSetsByteGauges(t *testing.T) {
	raw := &RawMetrics{
		DataTransferred:     1024,
		TotalBytesProcessed: 4096,
		Folder:              "data-pvc",
		hostname:            "my-namespace",
	}

	metrics := raw.prometheus()

	assert.Equal(t, float64(1024), testutil.ToFloat64(metrics.DataTransferred.WithLabelValues("data-pvc", "my-namespace")))
	assert.Equal(t, float64(4096), testutil.ToFloat64(metrics.TotalBytesProcessed.WithLabelValues("data-pvc", "my-namespace")))
	assert.Equal(t, 1, testutil.CollectAndCount(metrics.DataTransferred))
	assert.Equal(t, 1, testutil.CollectAndCount(metrics.TotalBytesProcessed))
}

func TestBackupStats_ToJSONIncludesByteFields(t *testing.T) {
	stats := &BackupStats{
		Name: "my-namespace",
		BackupMetrics: &RawMetrics{
			DataTransferred:     1024,
			TotalBytesProcessed: 4096,
		},
	}

	var payload struct {
		BackupMetrics map[string]any `json:"backup_metrics"`
	}
	require.NoError(t, json.Unmarshal(stats.ToJSON(), &payload))

	assert.Equal(t, float64(1024), payload.BackupMetrics["data_transferred"])
	assert.Equal(t, float64(4096), payload.BackupMetrics["total_bytes_processed"])
}
