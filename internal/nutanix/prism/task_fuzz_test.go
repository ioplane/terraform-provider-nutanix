package prism

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	ntnxtask "github.com/ioplane/terraform-provider-nutanix/internal/task"
)

func FuzzTaskResponseDecode(f *testing.F) {
	// Seeds come from the locked Prism v4.3 getTaskById Task, AppMessage,
	// KVPair, and MapOfStringWrapper schemas. Completion values are parsed only
	// to validate the seven approved union variants, then discarded.
	for _, body := range []string{
		taskEnvelope("task-id", "RUNNING", nil, nil, nil),
		`{"data":{"extId":"task-id","status":"SUCCEEDED","completionDetails":[{"name":"string","value":"safe"},{"name":"integer","value":42},{"name":"boolean","value":true},{"name":"strings","value":["one","two"]},{"name":"integers","value":[1,2]},{"name":"map","value":{"key":"safe"}},{"name":"maps","value":[{}, {"map":{"key":"safe"}}]}]}}`,
		`{"data":{"extId":"task-id","status":"FAILED","errorMessages":[{"code":"TASK.SAFE","errorGroup":"TASK","severity":"ERROR","message":"fuzz-secret-canary"}]}}`,
		`{"data":{"extId":"task-id","status":"RUNNING","completionDetails":[{"name":"fraction","value":1.5}]}}`,
		`{"data":{"extId":"task-id","status":"RUNNING","completionDetails":[{"name":"raw-map-list","value":[{"key":"invalid"}]}]}}`,
		`{"data":{"extId":"task-id","status":"RUNNING"}} {}`,
		`{"data":`,
		strings.Repeat("[", 128) + strings.Repeat("]", 128),
	} {
		f.Add([]byte(body))
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		if int64(len(body)) > getTaskBodyLimit {
			return
		}
		snapshot, err := decodeTask(body, "task-id")
		if err != nil {
			if !errors.Is(err, ErrInvalidTaskResponse) {
				t.Fatalf("decodeTask() returned unclassified error %T: %v", err, err)
			}
			if strings.Contains(fmt.Sprintf("%+v", err), "fuzz-secret-canary") {
				t.Fatalf("decode error leaked remote canary: %+v", err)
			}
			return
		}
		if snapshot.ExtID != "task-id" || snapshot.ProgressPercentage < 0 ||
			snapshot.ProgressPercentage > 100 || len(snapshot.Errors) > maximumTaskErrors ||
			len(snapshot.Warnings) > maximumTaskWarnings || snapshot.ServerDelay != 0 {
			t.Fatalf("unsafe successful snapshot: %#v", snapshot)
		}
		switch snapshot.Status.Outcome() {
		case ntnxtask.OutcomePending,
			ntnxtask.OutcomeSucceeded,
			ntnxtask.OutcomeFailed,
			ntnxtask.OutcomeCanceled,
			ntnxtask.OutcomeUnsupported:
		default:
			t.Fatalf("unknown successful outcome: %q", snapshot.Status.Outcome())
		}
		for _, rendered := range []string{
			fmt.Sprintf("%v", snapshot),
			fmt.Sprintf("%+v", snapshot),
			fmt.Sprintf("%#v", snapshot),
		} {
			if strings.Contains(rendered, "fuzz-secret-canary") {
				t.Fatalf("snapshot formatting leaked remote canary: %q", rendered)
			}
		}
	})
}
