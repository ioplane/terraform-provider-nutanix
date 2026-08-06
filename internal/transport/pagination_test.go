package transport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestPaginationStartsAtPageZeroAndUsesOperationLimit(t *testing.T) {
	t.Parallel()

	var pages []int
	var limits []int
	var visited []string
	err := WalkPages(context.Background(), PaginationOptions{
		StartPage:   0,
		Limit:       2,
		PageCeiling: 4,
		ItemCeiling: 8,
	}, func(_ context.Context, page, limit int) (Page[string], error) {
		pages = append(pages, page)
		limits = append(limits, limit)
		total := int64(3)
		if page == 0 {
			return Page[string]{Items: []string{"one", "two"}, TotalAvailableResults: &total}, nil
		}
		return Page[string]{Items: []string{"three"}, TotalAvailableResults: &total}, nil
	}, func(_ context.Context, item string) error {
		visited = append(visited, item)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkPages() error = %v", err)
	}
	if fmt.Sprint(pages) != "[0 1]" || fmt.Sprint(limits) != "[2 2]" {
		t.Fatalf("fetch calls pages=%v limits=%v, want [0 1] and [2 2]", pages, limits)
	}
	if fmt.Sprint(visited) != "[one two three]" {
		t.Fatalf("visited = %v", visited)
	}
}

func TestPaginationCurrentTotalIsAuthoritative(t *testing.T) {
	t.Parallel()

	t.Run("decrease above visited visits whole current page then terminates", func(t *testing.T) {
		t.Parallel()
		var calls int
		var visited []int
		err := WalkPages(context.Background(), paginationTestOptions(), func(_ context.Context, page, _ int) (Page[int], error) {
			calls++
			total := int64(4)
			if page == 0 {
				return Page[int]{Items: []int{1, 2}, TotalAvailableResults: &total}, nil
			}
			total = 3
			return Page[int]{Items: []int{3, 4}, TotalAvailableResults: &total}, nil
		}, func(_ context.Context, item int) error {
			visited = append(visited, item)
			return nil
		})
		if err != nil || calls != 2 || fmt.Sprint(visited) != "[1 2 3 4]" {
			t.Fatalf("decreasing total result err=%v calls=%d visited=%v", err, calls, visited)
		}
	})

	t.Run("increase on next page", func(t *testing.T) {
		t.Parallel()
		var calls int
		var visited []int
		err := WalkPages(context.Background(), paginationTestOptions(), func(_ context.Context, page, _ int) (Page[int], error) {
			calls++
			total := int64(5)
			if page == 0 {
				total = 3
				return Page[int]{Items: []int{1, 2}, TotalAvailableResults: &total}, nil
			}
			if page == 1 {
				return Page[int]{Items: []int{3, 4}, TotalAvailableResults: &total}, nil
			}
			return Page[int]{Items: []int{5}, TotalAvailableResults: &total}, nil
		}, func(_ context.Context, item int) error {
			visited = append(visited, item)
			return nil
		})
		if err != nil || calls != 3 || fmt.Sprint(visited) != "[1 2 3 4 5]" {
			t.Fatalf("increasing total result err=%v calls=%d visited=%v", err, calls, visited)
		}
	})

	t.Run("decrease to visited terminates before current items", func(t *testing.T) {
		t.Parallel()
		var visited []string
		err := WalkPages(context.Background(), paginationTestOptions(), func(_ context.Context, page, _ int) (Page[string], error) {
			if page == 0 {
				total := int64(4)
				return Page[string]{Items: []string{"one", "two"}, TotalAvailableResults: &total}, nil
			}
			total := int64(2)
			return Page[string]{Items: []string{"must-not-visit", "must-not-visit-2"}, TotalAvailableResults: &total}, nil
		}, func(_ context.Context, item string) error {
			visited = append(visited, item)
			return nil
		})
		if err != nil || fmt.Sprint(visited) != "[one two]" {
			t.Fatalf("decreasing total result err=%v visited=%v", err, visited)
		}
	})
}

func TestPaginationAbsentTotalUsesShortThenEmptyFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pages     [][]int
		wantCalls int
		wantItems string
	}{
		{name: "initial short", pages: [][]int{{1}}, wantCalls: 1, wantItems: "[1]"},
		{name: "full then short", pages: [][]int{{1, 2}, {3}}, wantCalls: 2, wantItems: "[1 2 3]"},
		{name: "full then empty", pages: [][]int{{1, 2}, {}}, wantCalls: 2, wantItems: "[1 2]"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls int
			var visited []int
			err := WalkPages(context.Background(), paginationTestOptions(), func(_ context.Context, page, _ int) (Page[int], error) {
				calls++
				return Page[int]{Items: test.pages[page]}, nil
			}, func(_ context.Context, item int) error {
				visited = append(visited, item)
				return nil
			})
			if err != nil || calls != test.wantCalls || fmt.Sprint(visited) != test.wantItems {
				t.Fatalf("WalkPages() err=%v calls=%d visited=%v", err, calls, visited)
			}
		})
	}
}

func TestPaginationRejectsInvalidMetadataBeforeCurrentPageVisit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		options   PaginationOptions
		page      Page[int]
		wantKind  PaginationFailureKind
		wantCause error
	}{
		{name: "negative total", options: paginationTestOptions(), page: Page[int]{Items: []int{1, 2}, TotalAvailableResults: int64Pointer(-1)}, wantKind: PaginationFailureInvalid, wantCause: ErrInvalidPagination},
		{name: "server exceeds limit", options: paginationTestOptions(), page: Page[int]{Items: []int{1, 2, 3}}, wantKind: PaginationFailureInconsistent, wantCause: ErrPaginationInconsistent},
		{name: "short before total", options: paginationTestOptions(), page: Page[int]{Items: []int{1}, TotalAvailableResults: int64Pointer(3)}, wantKind: PaginationFailureInconsistent, wantCause: ErrPaginationInconsistent},
		{name: "empty before total", options: paginationTestOptions(), page: Page[int]{TotalAvailableResults: int64Pointer(1)}, wantKind: PaginationFailureInconsistent, wantCause: ErrPaginationInconsistent},
		{name: "item ceiling", options: PaginationOptions{Limit: 2, PageCeiling: 2, ItemCeiling: 1}, page: Page[int]{Items: []int{1, 2}}, wantKind: PaginationFailureItemCeiling, wantCause: ErrPaginationItemCeiling},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var visits int
			err := WalkPages(context.Background(), test.options, func(context.Context, int, int) (Page[int], error) {
				return test.page, nil
			}, func(context.Context, int) error {
				visits++
				return nil
			})
			var paginationError *PaginationError
			if !errors.As(err, &paginationError) || paginationError.Kind() != test.wantKind || !errors.Is(err, test.wantCause) {
				t.Fatalf("WalkPages() error = %v, kind=%v cause=%v", err, test.wantKind, test.wantCause)
			}
			if visits != 0 {
				t.Fatalf("visitor calls = %d, want 0", visits)
			}
		})
	}
}

func TestPaginationRejectsInvalidOptionsWithoutCallbacks(t *testing.T) {
	t.Parallel()

	tests := []PaginationOptions{
		{},
		{StartPage: -1, Limit: 1, PageCeiling: 1, ItemCeiling: 1},
		{StartPage: 1, Limit: 1, PageCeiling: 1, ItemCeiling: 1},
		{StartPage: math.MaxInt, Limit: 1, PageCeiling: 1, ItemCeiling: 1},
		{Limit: 0, PageCeiling: 1, ItemCeiling: 1},
		{Limit: 1, PageCeiling: 0, ItemCeiling: 1},
		{Limit: 1, PageCeiling: 1, ItemCeiling: 0},
	}
	for _, options := range tests {
		var fetchCalls, visitCalls int
		err := WalkPages(context.Background(), options, func(context.Context, int, int) (Page[int], error) {
			fetchCalls++
			return Page[int]{}, nil
		}, func(context.Context, int) error {
			visitCalls++
			return nil
		})
		if !errors.Is(err, ErrInvalidPagination) || fetchCalls != 0 || visitCalls != 0 {
			t.Fatalf("WalkPages(%#v) err=%v fetch=%d visit=%d", options, err, fetchCalls, visitCalls)
		}
	}
	for _, test := range []struct {
		name  string
		fetch PageFetcher[int]
		visit ItemVisitor[int]
	}{
		{name: "nil fetch", visit: func(context.Context, int) error { return nil }},
		{name: "nil visitor", fetch: func(context.Context, int, int) (Page[int], error) { return Page[int]{}, nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := WalkPages(context.Background(), paginationTestOptions(), test.fetch, test.visit)
			if !errors.Is(err, ErrInvalidPagination) {
				t.Fatalf("WalkPages() error = %v, want ErrInvalidPagination", err)
			}
		})
	}
}

func TestPaginationEnforcesPageCeilingAndPageOverflow(t *testing.T) {
	t.Parallel()

	var calls int
	err := WalkPages(context.Background(), PaginationOptions{Limit: 1, PageCeiling: 1, ItemCeiling: 2}, func(context.Context, int, int) (Page[int], error) {
		calls++
		return Page[int]{Items: []int{1}}, nil
	}, func(context.Context, int) error { return nil })
	var paginationError *PaginationError
	if !errors.As(err, &paginationError) || paginationError.Kind() != PaginationFailurePageCeiling || !errors.Is(err, ErrPaginationPageCeiling) || calls != 1 {
		t.Fatalf("WalkPages() err=%v calls=%d", err, calls)
	}

	decision, err := paginationMetadataTransition(math.MaxInt64, 1, 0, 1, 2, 2, 1, nil)
	if decision != (paginationDecision{}) || !errors.Is(err, ErrPaginationOverflow) {
		t.Fatalf("page overflow transition = %#v, %v", decision, err)
	}
}

func TestPaginationMetadataArithmeticRejectsOverflowWithoutAllocation(t *testing.T) {
	t.Parallel()

	decision, err := paginationMetadataTransition(
		0,
		1,
		math.MaxInt64-1,
		2,
		2,
		math.MaxInt64,
		2,
		int64Pointer(math.MaxInt64),
	)
	if decision != (paginationDecision{}) || !errors.Is(err, ErrPaginationOverflow) {
		t.Fatalf("paginationMetadataTransition() = %#v, %v; want overflow", decision, err)
	}

	var visits int
	err = WalkPages(context.Background(), paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
		return Page[int]{Items: []int{1}, TotalAvailableResults: int64Pointer(math.MaxInt64)}, nil
	}, func(context.Context, int) error {
		visits++
		return nil
	})
	if !errors.Is(err, ErrPaginationInconsistent) || visits != 0 {
		t.Fatalf("WalkPages(enormous total) err=%v visits=%d", err, visits)
	}
}

func TestPaginationHonorsCancellationAtEveryBoundary(t *testing.T) {
	t.Parallel()

	t.Run("before fetch", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var calls int
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			calls++
			return Page[int]{}, nil
		}, func(context.Context, int) error { return nil })
		if !errors.Is(err, context.Canceled) || calls != 0 {
			t.Fatalf("WalkPages() err=%v calls=%d", err, calls)
		}
	})

	t.Run("between items", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		var visits int
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			return Page[int]{Items: []int{1, 2}}, nil
		}, func(context.Context, int) error {
			visits++
			cancel()
			return nil
		})
		if !errors.Is(err, context.Canceled) || visits != 1 {
			t.Fatalf("WalkPages() err=%v visits=%d", err, visits)
		}
	})

	t.Run("before next page", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		var fetches int
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			fetches++
			return Page[int]{Items: []int{1, 2}}, nil
		}, func(_ context.Context, item int) error {
			if item == 2 {
				cancel()
			}
			return nil
		})
		if !errors.Is(err, context.Canceled) || fetches != 1 {
			t.Fatalf("WalkPages() err=%v fetches=%d", err, fetches)
		}
	})

	t.Run("after final visitor callback", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		var visits int
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			return Page[int]{Items: []int{1}}, nil
		}, func(context.Context, int) error {
			visits++
			cancel()
			return nil
		})
		if !errors.Is(err, context.Canceled) || visits != 1 {
			t.Fatalf("WalkPages() err=%v visits=%d, want terminal cancellation", err, visits)
		}
	})

	t.Run("deadline after final visitor callback", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		var visits int
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			return Page[int]{Items: []int{1}}, nil
		}, func(ctx context.Context, _ int) error {
			visits++
			<-ctx.Done()
			return nil
		})
		if !errors.Is(err, context.DeadlineExceeded) || visits != 1 {
			t.Fatalf("WalkPages() err=%v visits=%d, want terminal deadline", err, visits)
		}
	})

	t.Run("fetch cancellation wins over callback error", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		var visits int
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			cancel()
			return Page[int]{}, errors.New("fetch-race-secret-canary")
		}, func(context.Context, int) error {
			visits++
			return nil
		})
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrPaginationFetch) || visits != 0 || strings.Contains(fmt.Sprintf("%#v", err), "fetch-race-secret-canary") {
			t.Fatalf("WalkPages() err=%v visits=%d", err, visits)
		}
	})

	t.Run("visitor cancellation wins over callback error", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			return Page[int]{Items: []int{1}}, nil
		}, func(context.Context, int) error {
			cancel()
			return errors.New("visitor-race-secret-canary")
		})
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrPaginationVisit) || strings.Contains(fmt.Sprintf("%#v", err), "visitor-race-secret-canary") {
			t.Fatalf("WalkPages() err=%v", err)
		}
	})

	t.Run("deadline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
		defer cancel()
		err := WalkPages(ctx, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
			return Page[int]{}, nil
		}, func(context.Context, int) error { return nil })
		var paginationError *PaginationError
		if !errors.As(err, &paginationError) || paginationError.Kind() != PaginationFailureDeadline || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("WalkPages() error = %v, want typed deadline", err)
		}
	})
}

func TestPaginationHostileContextErrorsReturnPromptlyAndFailClosed(t *testing.T) {
	cycle := &paginationCyclicContextError{}
	cycle.next = cycle

	tests := []struct {
		name   string
		ctxErr error
	}{
		{name: "cyclic self unwrap", ctxErr: cycle},
		{name: "panicking Is", ctxErr: paginationPanickingIsContextError{}},
		{name: "panicking Unwrap", ctxErr: paginationPanickingUnwrapContextError{}},
		{name: "wrapped deadline is not legal Context Err", ctxErr: paginationWrappingContextError{next: context.DeadlineExceeded}},
		{name: "dynamically uncomparable", ctxErr: paginationDynamicallyUncomparableError{payload: []string{"dynamic-context-secret-canary"}}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			var fetchCalls, visitCalls int
			type result struct {
				err   error
				panic any
			}
			results := make(chan result, 1)
			go func() {
				completed := result{}
				defer func() {
					completed.panic = recover()
					results <- completed
				}()
				completed.err = WalkPages(paginationHostileContext{Context: context.Background(), err: test.ctxErr}, paginationTestOptions(), func(context.Context, int, int) (Page[int], error) {
					fetchCalls++
					return Page[int]{}, nil
				}, func(context.Context, int) error {
					visitCalls++
					return nil
				})
			}()

			select {
			case completed := <-results:
				if completed.panic != nil {
					t.Fatalf("WalkPages() panicked: %v", completed.panic)
				}
				var paginationError *PaginationError
				if !errors.As(completed.err, &paginationError) || paginationError.Kind() != PaginationFailureCanceled || !errors.Is(completed.err, context.Canceled) {
					t.Fatalf("WalkPages() error = %v, want safe canceled PaginationError", completed.err)
				}
				if fetchCalls != 0 || visitCalls != 0 {
					t.Fatalf("callbacks fetch=%d visit=%d, want zero", fetchCalls, visitCalls)
				}
				for _, rendered := range formatEveryWay(completed.err) {
					for _, canary := range []string{"cyclic-context-secret-canary", "panicking-is-context-secret-canary", "panicking-unwrap-context-secret-canary", "wrapped-context-secret-canary", "dynamic-context-secret-canary"} {
						if strings.Contains(rendered, canary) {
							t.Fatalf("error leaked %q: %q", canary, rendered)
						}
					}
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("WalkPages() did not return within 500ms")
			}
		})
	}
}

func TestPaginationCallbackErrorsAreTypedAndRedacted(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		fetch    PageFetcher[string]
		visit    ItemVisitor[string]
		wantKind PaginationFailureKind
		want     error
	}{
		{
			name: "fetch",
			fetch: func(context.Context, int, int) (Page[string], error) {
				return Page[string]{}, errors.New("vendor-fetch-secret-canary")
			},
			visit:    func(context.Context, string) error { return nil },
			wantKind: PaginationFailureFetch,
			want:     ErrPaginationFetch,
		},
		{
			name: "visitor",
			fetch: func(context.Context, int, int) (Page[string], error) {
				return Page[string]{Items: []string{"item-secret-canary"}}, nil
			},
			visit: func(context.Context, string) error {
				return errors.New("vendor-visit-secret-canary")
			},
			wantKind: PaginationFailureVisit,
			want:     ErrPaginationVisit,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			err := WalkPages(context.Background(), paginationTestOptions(), test.fetch, test.visit)
			var paginationError *PaginationError
			if !errors.As(err, &paginationError) || paginationError.Kind() != test.wantKind || !errors.Is(err, test.want) {
				t.Fatalf("WalkPages() error = %v", err)
			}
			for _, rendered := range formatEveryWay(err) {
				for _, canary := range []string{"vendor-fetch-secret-canary", "vendor-visit-secret-canary", "item-secret-canary"} {
					if strings.Contains(rendered, canary) {
						t.Fatalf("error leaked %q: %q", canary, rendered)
					}
				}
			}
		})
	}
}

func TestPaginationPageFormattingRedactsItems(t *testing.T) {
	t.Parallel()

	page := Page[string]{Items: []string{"page-format-secret-canary"}, TotalAvailableResults: int64Pointer(1)}
	for _, rendered := range formatEveryWay(page) {
		if strings.Contains(rendered, "page-format-secret-canary") {
			t.Fatalf("Page formatting leaked an item: %q", rendered)
		}
	}
}

func paginationTestOptions() PaginationOptions {
	return PaginationOptions{Limit: 2, PageCeiling: 4, ItemCeiling: 8}
}

func int64Pointer(value int64) *int64 { return &value }

type paginationHostileContext struct {
	context.Context
	err error
}

func (ctx paginationHostileContext) Err() error { return ctx.err }

type paginationCyclicContextError struct {
	next error
}

func (*paginationCyclicContextError) Error() string { return "cyclic-context-secret-canary" }

func (e *paginationCyclicContextError) Unwrap() error { return e.next }

type paginationPanickingIsContextError struct{}

func (paginationPanickingIsContextError) Error() string {
	return "panicking-is-context-secret-canary"
}

func (paginationPanickingIsContextError) Is(error) bool {
	panic("panicking Is context secret canary")
}

type paginationPanickingUnwrapContextError struct{}

func (paginationPanickingUnwrapContextError) Error() string {
	return "panicking-unwrap-context-secret-canary"
}

func (paginationPanickingUnwrapContextError) Unwrap() error {
	panic("panicking Unwrap context secret canary")
}

type paginationDynamicallyUncomparableError struct {
	payload any
}

func (paginationDynamicallyUncomparableError) Error() string {
	return "dynamic-context-secret-canary"
}

type paginationWrappingContextError struct {
	next error
}

func (paginationWrappingContextError) Error() string { return "wrapped-context-secret-canary" }

func (e paginationWrappingContextError) Unwrap() error { return e.next }
