package service

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("process-events")

