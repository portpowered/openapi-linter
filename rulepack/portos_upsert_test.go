package rulepack

import (
	"strings"
	"testing"
)

func TestEtagConflict_RequiresPreconditionStatuses(t *testing.T) {
	source := responsePolicySource("/groups/{id}", "post", "GroupUpsert", conditionalHeaders+"responses: {'409': {}}")
	findings := checkAPI(t, "portos.etag-conflict", source, "")
	if len(findings) != 2 {
		t.Fatalf("got %#v, want two missing precondition statuses", findings)
	}
	for i, status := range []string{"412", "428"} {
		if findings[i].Pointer != "#/paths/~1groups~1{id}/post" || !strings.Contains(findings[i].Message, status) {
			t.Fatalf("finding %d: %#v", i, findings[i])
		}
	}
}

func TestEtagConflict_AllowsInFlightConflictWithIdempotencyKey(t *testing.T) {
	headers := "parameters: [{name: If-Match, in: header}, {name: Idempotency-Key, in: header}], "
	source := responsePolicySource("/groups", "post", "GroupUpsert", headers+"responses: {'409': {}, '412': {}, '428': {}}")
	if findings := checkAPI(t, "portos.etag-conflict", source, ""); len(findings) != 0 {
		t.Fatalf("unexpected findings %#v", findings)
	}
}

func TestDeleteIdempotent_RequiresIdempotencyKeyAllows404(t *testing.T) {
	source := responsePolicySource("/groups/{id}", "delete", "GroupDelete", "responses: {'404': {}}")
	findings := checkAPI(t, "portos.delete-idempotent", source, "")
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1groups~1{id}/delete" || !strings.Contains(findings[0].Message, "Idempotency-Key") {
		t.Fatalf("unexpected findings %#v", findings)
	}
	source = strings.Replace(source, "operationId: GroupDelete,", "operationId: GroupDelete, parameters: [{name: Idempotency-Key, in: header}],", 1)
	if findings := checkAPI(t, "portos.delete-idempotent", source, ""); len(findings) != 0 {
		t.Fatalf("unexpected findings %#v", findings)
	}
}
