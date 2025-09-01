package tracing

type SpanKind string

const (
	KindClient   SpanKind = "CLIENT"
	KindServer   SpanKind = "SERVER"
	KindProducer SpanKind = "PRODUCER"
	KindConsumer SpanKind = "CONSUMER"
)

type ZipkinSpan struct {
	TraceID       string            `json:"traceId"`
	ID            string            `json:"id"`
	ParentID      string            `json:"parentId,omitempty"`
	Name          string            `json:"name"`
	Timestamp     int64             `json:"timestamp"` // microseconds
	Duration      int64             `json:"duration"`  // microseconds
	LocalEndpoint *Endpoint         `json:"localEndpoint"`
	Tags          map[string]string `json:"tags,omitempty"`
	Annotations   []Annotation      `json:"annotations,omitempty"`
	Kind          SpanKind          `json:"kind,omitempty"`
}

type Endpoint struct {
	ServiceName string `json:"serviceName"`
	IPv4        string `json:"ipv4,omitempty"`
}

type Annotation struct {
	Timestamp int64  `json:"timestamp"` // microseconds
	Value     string `json:"value"`
}
