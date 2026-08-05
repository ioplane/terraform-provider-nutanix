package transport

import (
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const attemptLogMessage = "Nutanix API request attempt completed"

type attemptEvent struct {
	operation     string
	method        string
	pathTemplate  string
	attempt       int
	status        int
	duration      time.Duration
	correlationID string
}

func (e attemptEvent) fields() map[string]any {
	fields := map[string]any{
		"operation":     e.operation,
		"method":        e.method,
		"path_template": e.pathTemplate,
		"attempt":       e.attempt,
		"status":        e.status,
		"duration":      e.duration.String(),
	}
	if correlationID := validCorrelationIDOrEmpty(e.correlationID); correlationID != "" {
		fields["correlation_id"] = correlationID
	}
	return fields
}

type eventSink interface {
	EmitAttempt(context.Context, attemptEvent)
}

type tflogEventSink struct{}

func (tflogEventSink) EmitAttempt(ctx context.Context, event attemptEvent) {
	if ctx == nil {
		ctx = context.Background()
	}
	tflog.Debug(ctx, attemptLogMessage, event.fields())
}

func statusFromResponse(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}
