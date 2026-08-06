// Package capability implements neutral, explicitly registered capability probes.
package capability

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

const maximumCapabilityNameBytes = 128

var (
	// ErrInvalidRegistry identifies an invalid capability definition.
	ErrInvalidRegistry = errors.New("capability registry is invalid")
	// ErrInvalidCheck identifies an invalid capability check input.
	ErrInvalidCheck = errors.New("capability check input is invalid")
)

// Probe is an explicitly registered read-only capability operation.
// False with nil error means explicitly unsupported; every error is indeterminate.
type Probe interface {
	Check(context.Context) (bool, error)
}

// ProbeFunc adapts a function to Probe.
type ProbeFunc func(context.Context) (bool, error)

// Check invokes the adapted function.
func (function ProbeFunc) Check(ctx context.Context) (bool, error) {
	return function(ctx)
}

type flight struct {
	ready               chan struct{}
	supported           bool
	err                 error
	ownerContextFailure bool
}

// Registry caches only supported and explicitly unsupported probe results.
type Registry struct {
	probes map[string]Probe

	mu       sync.Mutex
	cache    map[string]bool
	inFlight map[string]*flight
}

// Format prevents probe implementations and cache state from entering diagnostics.
func (*Registry) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("capability.Registry(redacted)"))
}

// NewRegistry validates and copies the complete approved probe set.
func NewRegistry(probes map[string]Probe) (*Registry, error) {
	locked := make(map[string]Probe, len(probes))
	for name, probe := range probes {
		if !validCapabilityName(name) || nilInterface(probe) {
			return nil, ErrInvalidRegistry
		}
		locked[name] = probe
	}
	return &Registry{
		probes:   locked,
		cache:    make(map[string]bool),
		inFlight: make(map[string]*flight),
	}, nil
}

// Check returns true only for a documented supported capability.
func (r *Registry) Check(ctx context.Context, name string) (bool, error) {
	if ctx == nil || !validCapabilityName(name) {
		return false, ErrInvalidCheck
	}
	if r == nil || r.probes == nil || r.cache == nil || r.inFlight == nil {
		return false, ErrInvalidRegistry
	}
	for {
		if contextErr := ctx.Err(); contextErr != nil {
			return false, capabilityContextError(name, contextErr)
		}

		r.mu.Lock()
		if supported, cached := r.cache[name]; cached {
			r.mu.Unlock()
			return cachedCapabilityResult(name, supported)
		}
		probe, registered := r.probes[name]
		if !registered {
			r.mu.Unlock()
			return false, newCapabilityError(name, FailureIndeterminate, ErrCapabilityIndeterminate)
		}
		if current := r.inFlight[name]; current != nil {
			r.mu.Unlock()
			supported, err, retry := waitForFlight(ctx, name, current)
			if retry {
				continue
			}
			return supported, err
		}
		current := &flight{ready: make(chan struct{})}
		r.inFlight[name] = current
		r.mu.Unlock()

		supported, probeErr := probe.Check(ctx)
		resultSupported, cacheable, ownerContextFailure, resultErr := classifyProbeResult(
			ctx,
			name,
			supported,
			probeErr,
		)

		r.mu.Lock()
		current.supported = resultSupported
		current.err = resultErr
		current.ownerContextFailure = ownerContextFailure
		if cacheable {
			r.cache[name] = resultSupported
		}
		if r.inFlight[name] == current {
			delete(r.inFlight, name)
		}
		close(current.ready)
		r.mu.Unlock()
		return resultSupported, resultErr
	}
}

func waitForFlight(ctx context.Context, name string, current *flight) (bool, error, bool) {
	select {
	case <-ctx.Done():
		return false, capabilityContextError(name, ctx.Err()), false
	case <-current.ready:
		if contextErr := ctx.Err(); contextErr != nil {
			return false, capabilityContextError(name, contextErr), false
		}
		if current.ownerContextFailure {
			return false, nil, true
		}
		return current.supported, current.err, false
	}
}

func classifyProbeResult(
	ctx context.Context,
	name string,
	supported bool,
	probeErr error,
) (bool, bool, bool, error) {
	if contextErr := ctx.Err(); contextErr != nil {
		return false, false, true, capabilityContextError(name, contextErr)
	}
	if probeErr != nil {
		return false, false, false, newCapabilityError(name, FailureIndeterminate, probeErr)
	}
	if supported {
		return true, true, false, nil
	}
	return false, true, false, newCapabilityError(name, FailureUnsupported, ErrCapabilityUnsupported)
}

func cachedCapabilityResult(name string, supported bool) (bool, error) {
	if supported {
		return true, nil
	}
	return false, newCapabilityError(name, FailureUnsupported, ErrCapabilityUnsupported)
}

func capabilityContextError(name string, cause error) *CapabilityError {
	if cause == context.DeadlineExceeded {
		return newCapabilityError(name, FailureIndeterminate, context.DeadlineExceeded)
	}
	return newCapabilityError(name, FailureIndeterminate, context.Canceled)
}

func validCapabilityName(name string) bool {
	if name == "" || len(name) > maximumCapabilityNameBytes || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for index := 1; index < len(name); index++ {
		character := name[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._-", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
