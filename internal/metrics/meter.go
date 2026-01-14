package metrics

import "go.opentelemetry.io/otel/metric"

type Meter interface {
	metric.Meter
}
