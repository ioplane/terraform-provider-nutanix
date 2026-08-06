package transport

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrInvalidPagination identifies invalid pagination options or metadata.
	ErrInvalidPagination = errors.New("pagination input is invalid")
	// ErrPaginationInconsistent identifies page metadata that would silently truncate results.
	ErrPaginationInconsistent = errors.New("pagination response is inconsistent")
	// ErrPaginationPageCeiling identifies exhaustion of the approved page ceiling.
	ErrPaginationPageCeiling = errors.New("pagination page ceiling was reached")
	// ErrPaginationItemCeiling identifies exhaustion of the approved item ceiling.
	ErrPaginationItemCeiling = errors.New("pagination item ceiling was reached")
	// ErrPaginationOverflow identifies unsafe page or item arithmetic.
	ErrPaginationOverflow = errors.New("pagination arithmetic overflowed")
	// ErrPaginationFetch identifies a page fetch failure whose vendor details are not exposed.
	ErrPaginationFetch = errors.New("pagination page fetch failed")
	// ErrPaginationVisit identifies a visitor failure whose item details are not exposed.
	ErrPaginationVisit = errors.New("pagination item visit failed")
)

// PaginationFailureKind classifies a page walk failure without exposing page
// contents, callback errors, or vendor metadata.
type PaginationFailureKind string

const (
	// PaginationFailureInvalid identifies invalid walker options or metadata.
	PaginationFailureInvalid PaginationFailureKind = "invalid"
	// PaginationFailureInconsistent identifies a page that cannot satisfy its metadata.
	PaginationFailureInconsistent PaginationFailureKind = "inconsistent"
	// PaginationFailurePageCeiling identifies a walk requiring another disallowed page.
	PaginationFailurePageCeiling PaginationFailureKind = "page_ceiling"
	// PaginationFailureItemCeiling identifies a page that would cross the item ceiling.
	PaginationFailureItemCeiling PaginationFailureKind = "item_ceiling"
	// PaginationFailureOverflow identifies unsafe page or item arithmetic.
	PaginationFailureOverflow PaginationFailureKind = "overflow"
	// PaginationFailureFetch identifies a redacted fetch callback failure.
	PaginationFailureFetch PaginationFailureKind = "fetch"
	// PaginationFailureVisit identifies a redacted visitor callback failure.
	PaginationFailureVisit PaginationFailureKind = "visit"
	// PaginationFailureCanceled identifies caller cancellation.
	PaginationFailureCanceled PaginationFailureKind = "canceled"
	// PaginationFailureDeadline identifies caller deadline expiry.
	PaginationFailureDeadline PaginationFailureKind = "deadline"
)

// PaginationError is a vendor-neutral, secret-safe page walk failure.
type PaginationError struct {
	kind  PaginationFailureKind
	cause error
}

// Error returns a stable message containing no page or callback data.
func (e *PaginationError) Error() string {
	if e == nil {
		return "pagination walk failed"
	}
	return fmt.Sprintf("pagination walk failed (%s)", e.kind)
}

// Format prevents private error fields from entering verbose diagnostics.
func (e *PaginationError) Format(state fmt.State, verb rune) {
	formatSafeText(state, verb, e.Error())
}

// Unwrap exposes only a stable pagination or caller-context cause.
func (e *PaginationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Kind returns the stable pagination failure class.
func (e *PaginationError) Kind() PaginationFailureKind {
	if e == nil {
		return ""
	}
	return e.kind
}

// PaginationOptions defines a zero-based page walk and its operation-specific
// endpoint limit plus explicit safety ceilings.
type PaginationOptions struct {
	StartPage   int
	Limit       int
	PageCeiling int
	ItemCeiling int
}

// Page is one decoded namespace page. The total is optional; Items are visited
// synchronously and are never collected by the transport package.
type Page[T any] struct {
	Items                 []T
	TotalAvailableResults *int64
}

// Format prevents decoded page items from entering diagnostics or logs.
func (Page[T]) Format(state fmt.State, verb rune) {
	formatSafeText(state, verb, "transport.Page(redacted)")
}

// PageFetcher decodes exactly one locally selected zero-based page.
type PageFetcher[T any] func(ctx context.Context, page, limit int) (Page[T], error)

// ItemVisitor consumes one decoded item without requiring collect-all storage.
type ItemVisitor[T any] func(ctx context.Context, item T) error

// WalkPages fetches and visits pages sequentially without following response
// links. It treats each present total as authoritative for that page.
func WalkPages[T any](
	ctx context.Context,
	options PaginationOptions,
	fetch PageFetcher[T],
	visit ItemVisitor[T],
) error {
	if ctx == nil || fetch == nil || visit == nil || !validPaginationOptions(options) {
		return newPaginationError(PaginationFailureInvalid, ErrInvalidPagination)
	}
	if err := ctx.Err(); err != nil {
		return paginationContextError(err)
	}

	pageNumber := options.StartPage
	var pagesFetched int64
	var visited int64
	for {
		if err := ctx.Err(); err != nil {
			return paginationContextError(err)
		}
		page, err := fetch(ctx, pageNumber, options.Limit)
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return paginationContextError(contextErr)
			}
			return newPaginationError(PaginationFailureFetch, ErrPaginationFetch)
		}
		if err := ctx.Err(); err != nil {
			return paginationContextError(err)
		}

		pagesFetched++
		var total *int64
		if page.TotalAvailableResults != nil {
			value := *page.TotalAvailableResults
			total = &value
		}
		decision, err := paginationMetadataTransition(
			int64(pageNumber),
			pagesFetched,
			visited,
			int64(options.Limit),
			int64(options.PageCeiling),
			int64(options.ItemCeiling),
			int64(len(page.Items)),
			total,
		)
		if err != nil {
			return err
		}

		for index := int64(0); index < decision.visitCount; index++ {
			if err := ctx.Err(); err != nil {
				return paginationContextError(err)
			}
			visitErr := visit(ctx, page.Items[int(index)])
			if contextErr := ctx.Err(); contextErr != nil {
				return paginationContextError(contextErr)
			}
			if visitErr != nil {
				return newPaginationError(PaginationFailureVisit, ErrPaginationVisit)
			}
		}
		visited = decision.nextVisited
		if decision.terminal {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return paginationContextError(err)
		}
		if decision.nextPage > maximumInt64ForInt() {
			return newPaginationError(PaginationFailureOverflow, ErrPaginationOverflow)
		}
		pageNumber = int(decision.nextPage)
	}
}

type paginationDecision struct {
	visitCount  int64
	nextVisited int64
	nextPage    int64
	terminal    bool
}

func paginationMetadataTransition(
	page int64,
	pagesFetched int64,
	visited int64,
	limit int64,
	pageCeiling int64,
	itemCeiling int64,
	itemCount int64,
	total *int64,
) (paginationDecision, error) {
	if page < 0 || pagesFetched < 1 || visited < 0 || limit < 1 ||
		pageCeiling < 1 || itemCeiling < 1 || itemCount < 0 {
		return paginationDecision{}, newPaginationError(PaginationFailureInvalid, ErrInvalidPagination)
	}
	if pagesFetched > pageCeiling {
		return paginationDecision{}, newPaginationError(PaginationFailurePageCeiling, ErrPaginationPageCeiling)
	}
	if visited > itemCeiling {
		return paginationDecision{}, newPaginationError(PaginationFailureItemCeiling, ErrPaginationItemCeiling)
	}
	if itemCount > limit {
		return paginationDecision{}, newPaginationError(PaginationFailureInconsistent, ErrPaginationInconsistent)
	}
	if total != nil && *total < 0 {
		return paginationDecision{}, newPaginationError(PaginationFailureInvalid, ErrInvalidPagination)
	}
	if total != nil && *total <= visited {
		return paginationDecision{nextVisited: visited, terminal: true}, nil
	}
	if itemCount > int64(^uint64(0)>>1)-visited {
		return paginationDecision{}, newPaginationError(PaginationFailureOverflow, ErrPaginationOverflow)
	}
	nextVisited := visited + itemCount
	if nextVisited > itemCeiling {
		return paginationDecision{}, newPaginationError(PaginationFailureItemCeiling, ErrPaginationItemCeiling)
	}

	decision := paginationDecision{
		visitCount:  itemCount,
		nextVisited: nextVisited,
	}
	if total != nil {
		if itemCount < limit && nextVisited < *total {
			return paginationDecision{}, newPaginationError(PaginationFailureInconsistent, ErrPaginationInconsistent)
		}
		if nextVisited >= *total {
			decision.terminal = true
			return decision, nil
		}
	} else if itemCount < limit {
		decision.terminal = true
		return decision, nil
	}

	if pagesFetched >= pageCeiling {
		return paginationDecision{}, newPaginationError(PaginationFailurePageCeiling, ErrPaginationPageCeiling)
	}
	if page == int64(^uint64(0)>>1) {
		return paginationDecision{}, newPaginationError(PaginationFailureOverflow, ErrPaginationOverflow)
	}
	decision.nextPage = page + 1
	return decision, nil
}

func validPaginationOptions(options PaginationOptions) bool {
	return options.StartPage == 0 && options.Limit > 0 &&
		options.PageCeiling > 0 && options.ItemCeiling > 0
}

func maximumInt64ForInt() int64 {
	return int64(^uint(0) >> 1)
}

func newPaginationError(kind PaginationFailureKind, cause error) *PaginationError {
	return &PaginationError{kind: kind, cause: approvedPaginationCause(cause)}
}

func paginationContextError(err error) *PaginationError {
	if sameKnownError(err, context.DeadlineExceeded) {
		return newPaginationError(PaginationFailureDeadline, context.DeadlineExceeded)
	}
	return newPaginationError(PaginationFailureCanceled, context.Canceled)
}

func approvedPaginationCause(cause error) error {
	switch cause {
	case context.Canceled,
		context.DeadlineExceeded,
		ErrInvalidPagination,
		ErrPaginationInconsistent,
		ErrPaginationPageCeiling,
		ErrPaginationItemCeiling,
		ErrPaginationOverflow,
		ErrPaginationFetch,
		ErrPaginationVisit:
		return cause
	default:
		return nil
	}
}
