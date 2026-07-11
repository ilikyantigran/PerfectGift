// Package metrics exposes a couple of trivial Prometheus counters for the
// log-server, served at /metrics via promhttp.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// IngestedRecords counts log records accepted by /api/ingest.
var IngestedRecords = promauto.NewCounter(prometheus.CounterOpts{
	Name: "logserver_ingested_records_total",
	Help: "Total log records accepted by the ingest endpoint.",
})

// PrunedRecords counts rows removed by the retention pruner.
var PrunedRecords = promauto.NewCounter(prometheus.CounterOpts{
	Name: "logserver_pruned_records_total",
	Help: "Total log records deleted by the retention pruner.",
})
