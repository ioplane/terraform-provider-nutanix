package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestTransportErrorHasSafeTypedFieldsAndApprovedCause(t *testing.T) {
	t.Parallel()

	errorValue := newTransportError("prism.get_task", TransportFailureResponseRead, ErrResponseRead)
	var transportError *TransportError
	if !errors.As(errorValue, &transportError) {
		t.Fatalf("errors.As(%T) = false", errorValue)
	}
	if transportError.Operation() != "prism.get_task" || transportError.Kind() != TransportFailureResponseRead {
		t.Fatalf("typed fields = %q, %q", transportError.Operation(), transportError.Kind())
	}
	if !errors.Is(errorValue, ErrResponseRead) {
		t.Fatal("TransportError does not preserve approved stable cause")
	}
}

func TestTransportErrorDropsUnapprovedCauseAndRedactsEveryRendering(t *testing.T) {
	t.Parallel()

	rawCause := errors.New("network-cause-secret-canary")
	errorValue := newTransportError("prism.get_task", TransportFailureRequest, rawCause)
	if errors.Is(errorValue, rawCause) || errors.Unwrap(errorValue) != nil {
		t.Fatal("TransportError exposed an unapproved raw cause")
	}
	assertErrorRenderingsRedacted(t, errorValue, []string{"network-cause-secret-canary"})
}

func TestTransportErrorPreservesCallerAndStableKernelSentinels(t *testing.T) {
	t.Parallel()

	for _, cause := range []error{
		context.Canceled,
		context.DeadlineExceeded,
		ErrRedirectRefused,
		ErrInvalidRequest,
		ErrRequestFailed,
		ErrResponseRead,
		ErrResponseTooLarge,
		ErrResponseClose,
	} {
		cause := cause
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			errorValue := newTransportError("prism.get_task", TransportFailureRequest, cause)
			if !errors.Is(errorValue, cause) {
				t.Fatalf("errors.Is(%v) = false", cause)
			}
		})
	}
}

func TestHTTPErrorExposesOnlyBoundedBodyToNamespaceCode(t *testing.T) {
	t.Parallel()

	body := []byte(`{"vendorMessage":"vendor-body-secret-canary"}`)
	errorValue := newHTTPError(
		"prism.get_task",
		http.StatusBadGateway,
		"123e4567-e89b-12d3-a456-426614174000",
		body,
	)
	body[0] = 'x'

	var httpError *HTTPError
	if !errors.As(errorValue, &httpError) {
		t.Fatalf("errors.As(%T) = false", errorValue)
	}
	if httpError.Operation() != "prism.get_task" || httpError.StatusCode() != http.StatusBadGateway {
		t.Fatalf("typed fields = %q, %d", httpError.Operation(), httpError.StatusCode())
	}
	if httpError.CorrelationID() != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("correlation ID = %q", httpError.CorrelationID())
	}
	first := httpError.Body()
	if string(first) != `{"vendorMessage":"vendor-body-secret-canary"}` {
		t.Fatalf("bounded namespace body = %q", first)
	}
	first[0] = 'x'
	if string(httpError.Body()) != `{"vendorMessage":"vendor-body-secret-canary"}` {
		t.Fatal("HTTPError body getter exposed mutable storage")
	}
	if errors.Unwrap(httpError) != nil {
		t.Fatal("HTTPError unexpectedly unwraps vendor data")
	}
	assertErrorRenderingsRedacted(t, httpError, []string{"vendor-body-secret-canary"})
}

func TestHTTPErrorRejectsUnsafeCorrelationMetadata(t *testing.T) {
	t.Parallel()

	errorValue := newHTTPError("prism.get_task", http.StatusBadRequest, "correlation-secret-canary", []byte("safe"))
	if errorValue.CorrelationID() != "" {
		t.Fatalf("unsafe correlation ID = %q, want empty", errorValue.CorrelationID())
	}
	assertErrorRenderingsRedacted(t, errorValue, []string{"correlation-secret-canary"})
}

func assertErrorRenderingsRedacted(t *testing.T, err error, canaries []string) {
	t.Helper()
	for _, rendered := range []string{
		err.Error(),
		fmt.Sprint(err),
		fmt.Sprintf("%s", err),
		fmt.Sprintf("%v", err),
		fmt.Sprintf("%+v", err),
		fmt.Sprintf("%#v", err),
		fmt.Sprintf("%q", err),
	} {
		for _, canary := range canaries {
			if strings.Contains(rendered, canary) {
				t.Fatalf("error rendering leaked %q: %q", canary, rendered)
			}
		}
	}
}
