package capability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestRegistryCachesSupportedAndExplicitUnsupportedOnce(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		supported bool
		wantIs    error
	}{
		{name: "supported", supported: true},
		{name: "explicit unsupported", wantIs: ErrCapabilityUnsupported},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			registry := mustRegistry(t, map[string]Probe{
				"pc_2024_3": ProbeFunc(func(context.Context) (bool, error) {
					calls.Add(1)
					return test.supported, nil
				}),
			})
			for index := 0; index < 3; index++ {
				supported, err := registry.Check(context.Background(), "pc_2024_3")
				if supported != test.supported {
					t.Fatalf("Check() supported = %t, want %t", supported, test.supported)
				}
				if test.wantIs == nil && err != nil {
					t.Fatalf("Check() error = %v", err)
				}
				if test.wantIs != nil && !errors.Is(err, test.wantIs) {
					t.Fatalf("Check() error = %v, want %v", err, test.wantIs)
				}
			}
			if calls.Load() != 1 {
				t.Fatalf("probe calls = %d, want 1", calls.Load())
			}
		})
	}
}

func TestRegistrySharesOneInFlightCacheableProbeAndCachesOnce(t *testing.T) {
	const callers = 24
	for _, test := range []struct {
		name      string
		supported bool
		wantIs    error
	}{
		{name: "supported", supported: true},
		{name: "explicit unsupported", wantIs: ErrCapabilityUnsupported},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := make(chan struct{})
				release := make(chan struct{})
				var calls atomic.Int32
				registry := mustRegistry(t, map[string]Probe{
					"pc_2024_3": ProbeFunc(func(context.Context) (bool, error) {
						if calls.Add(1) == 1 {
							close(started)
						}
						<-release
						return test.supported, nil
					}),
				})

				results := make(chan capabilityResult, callers)
				go checkCapability(registry, context.Background(), results)
				<-started
				synctest.Wait()
				for index := 1; index < callers; index++ {
					go checkCapability(registry, context.Background(), results)
				}
				synctest.Wait()
				close(release)
				synctest.Wait()

				for index := 0; index < callers; index++ {
					assertCapabilityResult(t, <-results, test.supported, test.wantIs)
				}
				if calls.Load() != 1 {
					t.Fatalf("concurrent probe calls = %d, want 1", calls.Load())
				}

				supported, err := registry.Check(context.Background(), "pc_2024_3")
				assertCapabilityResult(t, capabilityResult{supported: supported, err: err}, test.supported, test.wantIs)
				if calls.Load() != 1 {
					t.Fatalf("post-cache probe calls = %d, want 1", calls.Load())
				}
			})
		})
	}
}

func TestRegistryDoesNotCacheIndeterminateResults(t *testing.T) {
	t.Parallel()

	causes := []struct {
		name  string
		cause error
	}{
		{name: "http 401", cause: ErrCapabilityAuthentication},
		{name: "http 403", cause: ErrCapabilityAuthorization},
		{name: "http 408", cause: ErrCapabilityRequestTimeout},
		{name: "http 429", cause: ErrCapabilityThrottled},
		{name: "http 500", cause: ErrCapabilityServer},
		{name: "canceled", cause: context.Canceled},
		{name: "deadline", cause: context.DeadlineExceeded},
		{name: "unknown", cause: errors.New("probe-secret-canary")},
	}
	for _, test := range causes {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			registry := mustRegistry(t, map[string]Probe{
				"pc_2024_3": ProbeFunc(func(context.Context) (bool, error) {
					if calls.Add(1) == 1 {
						return true, test.cause
					}
					return true, nil
				}),
			})
			if supported, err := registry.Check(context.Background(), "pc_2024_3"); supported || err == nil {
				t.Fatalf("first Check() = %t, %v; want indeterminate", supported, err)
			} else if strings.Contains(fmt.Sprintf("%+v", err), "probe-secret-canary") {
				t.Fatalf("indeterminate error leaked probe text: %+v", err)
			}
			if supported, err := registry.Check(context.Background(), "pc_2024_3"); !supported || err != nil {
				t.Fatalf("second Check() = %t, %v; want fresh supported probe", supported, err)
			}
			if calls.Load() != 2 {
				t.Fatalf("probe calls = %d, want 2", calls.Load())
			}
		})
	}
}

func TestRegistrySharesOneInFlightIndeterminateProbe(t *testing.T) {
	for _, test := range []struct {
		name  string
		cause error
	}{
		{name: "throttled", cause: ErrCapabilityThrottled},
		{name: "probe context sentinel with active owner", cause: context.Canceled},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const callers = 24
				started := make(chan struct{})
				release := make(chan struct{})
				var calls atomic.Int32
				registry := mustRegistry(t, map[string]Probe{
					"pc_2024_3": ProbeFunc(func(context.Context) (bool, error) {
						if calls.Add(1) == 1 {
							close(started)
						}
						<-release
						return false, test.cause
					}),
				})

				results := make(chan error, callers)
				go func() {
					_, err := registry.Check(context.Background(), "pc_2024_3")
					results <- err
				}()
				<-started
				synctest.Wait()
				for index := 1; index < callers; index++ {
					go func() {
						_, err := registry.Check(context.Background(), "pc_2024_3")
						results <- err
					}()
				}
				synctest.Wait()
				close(release)
				synctest.Wait()

				for index := 0; index < callers; index++ {
					if err := <-results; !errors.Is(err, test.cause) {
						t.Fatalf("shared result %d = %v", index, err)
					}
				}
				if calls.Load() != 1 {
					t.Fatalf("shared probe calls = %d, want 1", calls.Load())
				}

				if _, err := registry.Check(context.Background(), "pc_2024_3"); !errors.Is(err, test.cause) {
					t.Fatalf("fresh indeterminate Check() = %v", err)
				}
				if calls.Load() != 2 {
					t.Fatalf("post-flight probe calls = %d, want 2", calls.Load())
				}
			})
		})
	}
}

func TestRegistryWaiterCancellationDoesNotLeakOrCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		registry := mustRegistry(t, map[string]Probe{
			"pc_2024_3": ProbeFunc(func(context.Context) (bool, error) {
				close(started)
				<-release
				return true, nil
			}),
		})
		leader := make(chan error, 1)
		go func() {
			_, err := registry.Check(context.Background(), "pc_2024_3")
			leader <- err
		}()
		<-started
		synctest.Wait()

		ctx, cancel := context.WithCancel(context.Background())
		waiter := make(chan error, 1)
		go func() {
			_, err := registry.Check(ctx, "pc_2024_3")
			waiter <- err
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		if err := <-waiter; !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter error = %v", err)
		}

		close(release)
		synctest.Wait()
		if err := <-leader; err != nil {
			t.Fatalf("leader error = %v", err)
		}
		if supported, err := registry.Check(context.Background(), "pc_2024_3"); !supported || err != nil {
			t.Fatalf("cached supported result = %t, %v", supported, err)
		}
	})
}

func TestRegistryOwnerContextFailureIsCallerSpecific(t *testing.T) {
	const waiters = 12
	for _, test := range []struct {
		name   string
		wantIs error
		setup  func() (context.Context, func(), func())
	}{
		{
			name:   "canceled",
			wantIs: context.Canceled,
			setup: func() (context.Context, func(), func()) {
				ctx, cancel := context.WithCancel(context.Background())
				return ctx, cancel, cancel
			},
		},
		{
			name:   "deadline",
			wantIs: context.DeadlineExceeded,
			setup: func() (context.Context, func(), func()) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
				return ctx, func() { time.Sleep(time.Hour) }, cancel
			},
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := make(chan struct{})
				var calls atomic.Int32
				registry := mustRegistry(t, map[string]Probe{
					"pc_2024_3": ProbeFunc(func(ctx context.Context) (bool, error) {
						if calls.Add(1) == 1 {
							close(started)
							<-ctx.Done()
						}
						return true, nil
					}),
				})
				ownerCtx, failOwner, cleanup := test.setup()
				defer cleanup()

				ownerResult := make(chan capabilityResult, 1)
				go checkCapability(registry, ownerCtx, ownerResult)
				<-started
				synctest.Wait()
				waiterResults := make(chan capabilityResult, waiters)
				for index := 0; index < waiters; index++ {
					go checkCapability(registry, context.Background(), waiterResults)
				}
				synctest.Wait()
				failOwner()
				synctest.Wait()

				assertCapabilityResult(t, <-ownerResult, false, test.wantIs)
				for index := 0; index < waiters; index++ {
					assertCapabilityResult(t, <-waiterResults, true, nil)
				}
				if calls.Load() != 2 {
					t.Fatalf("probe calls after owner failure = %d, want 2", calls.Load())
				}

				supported, err := registry.Check(context.Background(), "pc_2024_3")
				assertCapabilityResult(t, capabilityResult{supported: supported, err: err}, true, nil)
				if calls.Load() != 2 {
					t.Fatalf("post-cache probe calls = %d, want 2", calls.Load())
				}
			})
		})
	}
}

func TestRegistryRejectsInvalidDefinitionsAndUnknownCapability(t *testing.T) {
	t.Parallel()

	if _, err := NewRegistry(nil); err != nil {
		t.Fatalf("NewRegistry(nil) error = %v, want valid empty registry", err)
	}
	var typedNil *nilProbe
	for _, probes := range []map[string]Probe{
		{"Bad Name": ProbeFunc(func(context.Context) (bool, error) { return true, nil })},
		{"safe": nil},
		{"safe": typedNil},
	} {
		if _, err := NewRegistry(probes); !errors.Is(err, ErrInvalidRegistry) {
			t.Fatalf("NewRegistry(%v) error = %v", probes, err)
		}
	}
	registry := mustRegistry(t, nil)
	if supported, err := registry.Check(context.Background(), "missing"); supported || !errors.Is(err, ErrCapabilityIndeterminate) {
		t.Fatalf("unknown Check() = %t, %v", supported, err)
	}
	if supported, err := registry.Check(nil, "missing"); supported || !errors.Is(err, ErrInvalidCheck) { //nolint:staticcheck // nil-context rejection is the behavior under test.
		t.Fatalf("nil-context Check() = %t, %v", supported, err)
	}
}

func mustRegistry(t *testing.T, probes map[string]Probe) *Registry {
	t.Helper()
	registry, err := NewRegistry(probes)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

type capabilityResult struct {
	supported bool
	err       error
}

func checkCapability(registry *Registry, ctx context.Context, results chan<- capabilityResult) {
	supported, err := registry.Check(ctx, "pc_2024_3")
	results <- capabilityResult{supported: supported, err: err}
}

func assertCapabilityResult(t *testing.T, result capabilityResult, wantSupported bool, wantIs error) {
	t.Helper()
	if result.supported != wantSupported {
		t.Fatalf("Check() supported = %t, want %t", result.supported, wantSupported)
	}
	if wantIs == nil && result.err != nil {
		t.Fatalf("Check() error = %v, want nil", result.err)
	}
	if wantIs != nil && !errors.Is(result.err, wantIs) {
		t.Fatalf("Check() error = %v, want %v", result.err, wantIs)
	}
}

type nilProbe struct{}

func (*nilProbe) Check(context.Context) (bool, error) { return false, nil }
