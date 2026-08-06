package transport

import (
	"errors"
	"math"
	"testing"
)

func FuzzPaginationMetadata(f *testing.F) {
	// Prism v4.3 locks zero-based $page, the documented default $limit=50,
	// the documented endpoint maximum $limit=100, and an example
	// totalAvailableResults=41. The larger seeds exercise ceilings without
	// allocating collections of attacker-controlled size.
	f.Add(int64(0), int64(1), int64(0), int64(50), int64(4), int64(200), int64(41), int64(41), true)
	f.Add(int64(0), int64(1), int64(0), int64(100), int64(1), int64(100), int64(100), int64(0), false)
	f.Add(int64(math.MaxInt64), int64(1), int64(0), int64(1), int64(2), int64(2), int64(1), int64(0), false)
	f.Add(int64(1), int64(2), int64(math.MaxInt64-1), int64(2), int64(3), int64(math.MaxInt64), int64(2), int64(math.MaxInt64), true)
	f.Add(int64(0), int64(1), int64(0), int64(50), int64(4), int64(200), int64(0), int64(-1), true)

	f.Fuzz(func(t *testing.T, page, pagesFetched, visited, limit, pageCeiling, itemCeiling, itemCount, totalValue int64, totalPresent bool) {
		var total *int64
		if totalPresent {
			total = &totalValue
		}
		decision, err := paginationMetadataTransition(
			page,
			pagesFetched,
			visited,
			limit,
			pageCeiling,
			itemCeiling,
			itemCount,
			total,
		)
		if err != nil {
			for _, allowed := range []error{
				ErrInvalidPagination,
				ErrPaginationInconsistent,
				ErrPaginationPageCeiling,
				ErrPaginationItemCeiling,
				ErrPaginationOverflow,
			} {
				if errors.Is(err, allowed) {
					return
				}
			}
			t.Fatalf("metadata transition returned unclassified error %T: %v", err, err)
		}
		if decision.visitCount < 0 || decision.nextVisited < visited || decision.nextVisited > itemCeiling {
			t.Fatalf("unsafe successful decision: %#v", decision)
		}
		if decision.terminal {
			return
		}
		if decision.nextPage != page+1 || decision.nextVisited != visited+itemCount {
			t.Fatalf("non-terminal decision did not advance exactly once: %#v", decision)
		}
	})
}
