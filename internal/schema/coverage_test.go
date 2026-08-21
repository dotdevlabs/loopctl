package schema_test

import (
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

// TestBidirectionalCoverage is the primary enforcement mechanism for the API contract gate.
// It runs on every `go test ./...` invocation using the embedded spec (no GITHUB_TOKEN needed).
//
// The test fails when:
//   - A spec operation lacks both a Covered entry and an Excluded entry (forward gate)
//   - A Covered entry references a path not in the spec (reverse gate A)
//   - An Excluded entry references a path not in the spec (reverse gate B)
//   - A Covered entry claims a query param the spec doesn't document (query param gate)
//   - A Covered entry for a paginated spec operation has Paginated: false (pagination gate)
func TestBidirectionalCoverage(t *testing.T) {
	endpoints, err := schema.Load()
	if err != nil {
		t.Fatalf("schema.Load() failed: %v", err)
	}

	// Index spec operations by OperationKey.
	specOps := make(map[schema.OperationKey]schema.EndpointAttrs, len(endpoints))
	for _, ep := range endpoints {
		specOps[schema.OperationKey{Method: ep.Method, Path: ep.Path}] = ep
	}

	// Forward gate: every spec operation is covered or explicitly excluded.
	for key, ep := range specOps {
		_, covered := schema.Covered[key]
		_, excluded := schema.Excluded[key]
		if !covered && !excluded {
			t.Errorf("spec operation %s %s (operationId=%q) has no loopctl coverage and is not in Excluded — add a command or add to Excluded with a reason",
				key.Method, key.Path, ep.OperationID)
		}
	}

	// Reverse gate A: every Covered entry must exist in the spec.
	for key, cov := range schema.Covered {
		if _, ok := specOps[key]; !ok {
			t.Errorf("Covered entry %q references %s %s which is not in the spec — spec may have renamed or removed it",
				cov.Command, key.Method, key.Path)
		}
	}

	// Reverse gate B: every Excluded entry must exist in the spec.
	for key, reason := range schema.Excluded {
		if _, ok := specOps[key]; !ok {
			t.Errorf("Excluded entry for %s %s is no longer in the spec (remove stale exclusion); exclusion reason: %s",
				key.Method, key.Path, reason)
		}
	}

	// Query param gate: claimed params must be documented in the spec.
	for key, cov := range schema.Covered {
		ep := specOps[key]
		specQPs := make(map[string]bool, len(ep.QueryParams))
		for _, qp := range ep.QueryParams {
			specQPs[qp.Name] = true
		}
		for _, claimed := range cov.QueryParams {
			if !specQPs[claimed] {
				t.Errorf("Covered[%s %s] (command=%q) claims query param %q but spec does not document it",
					key.Method, key.Path, cov.Command, claimed)
			}
		}
	}

	// Pagination gate: covered + paginated spec ops must declare Paginated: true.
	for key, ep := range specOps {
		if !ep.IsPaginated {
			continue
		}
		cov, ok := schema.Covered[key]
		if !ok {
			continue // excluded, not our problem
		}
		if !cov.Paginated {
			t.Errorf("Covered[%s %s] (command=%q) covers a paginated spec operation but Paginated is false — update the manifest",
				key.Method, key.Path, cov.Command)
		}
	}
}
