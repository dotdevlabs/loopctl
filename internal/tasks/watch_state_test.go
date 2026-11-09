package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/loopctl/internal/schema"
)

func TestWatchServerState(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		t.Run(fmt.Sprint(jsonMode), func(t *testing.T) {
			var gets int32
			steps := []watchStep{}
			for _, stage := range []string{"reviewed", "completed", "rejected", "blocked", "custom"} {
				steps = append(steps, watchStep{fmt.Sprintf(`{"stage":%q,"status":"implementing","terminal":false,"awaiting_human_gate":false,"awaiting_human_gate_stage":null}`, stage), `[]`})
			}
			gate := `{"stage":"approval","status":"implementing","terminal":false,"awaiting_human_gate":true,"awaiting_human_gate_stage":"approval","pr_url":"https://example.com/pr/1"}`
			steps = append(steps, watchStep{gate, `[]`}, watchStep{gate, `[]`}, watchStep{`{"stage":"merging","status":"implementing","terminal":false,"awaiting_human_gate":false,"awaiting_human_gate_stage":null}`, `[]`}, watchStep{gate, `[]`}, watchStep{`{"stage":"custom-end","status":"completed","terminal":true,"awaiting_human_gate":false,"awaiting_human_gate_stage":null}`, `[]`})
			ts := watchStub(t, "t1", steps, &gets)
			defer ts.Close()
			var out, diagnostics bytes.Buffer
			ctx := makeCtx(t, ts.URL, "tok", jsonMode, &out)
			err := watchTask(ctx, ctxutil.ActiveContextFrom(ctx), &out, &diagnostics, jsonMode, "t1", "1ms", "1s")
			if err != nil || atomic.LoadInt32(&gets) != int32(len(steps)) {
				t.Fatalf("polls=%d error=%v", gets, err)
			}
			progress := out.String()
			if jsonMode {
				progress = diagnostics.String()
				var result struct {
					Attributes TaskAttrs `json:"attributes"`
				}
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if !result.Attributes.Terminal || result.Attributes.Status != "completed" {
					t.Fatalf("result=%s", out.String())
				}
			}
			if strings.Count(progress, "waiting for approval at stage approval (https://example.com/pr/1)") != 2 {
				t.Fatalf("progress=%s", progress)
			}
		})
	}
}

func TestWatchInitialGateAndTimeout(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		var gets int32
		ts := watchStub(t, "t1", []watchStep{{`{"stage":"custom","status":"implementing","terminal":false,"awaiting_human_gate":true,"awaiting_human_gate_stage":"custom"}`, `[]`}}, &gets)
		var out, diagnostics bytes.Buffer
		ctx := makeCtx(t, ts.URL, "tok", jsonMode, &out)
		err := watchTask(ctx, ctxutil.ActiveContextFrom(ctx), &out, &diagnostics, jsonMode, "t1", "1ms", "30ms")
		ts.Close()
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("error=%v", err)
		}
		progress := out.String()
		if jsonMode {
			progress = diagnostics.String()
			var result map[string]string
			if json.Unmarshal(out.Bytes(), &result) != nil || result["error"] != "timeout" {
				t.Fatal(out.String())
			}
		}
		if strings.Count(progress, "waiting for approval at stage custom") != 1 {
			t.Fatal(progress)
		}
	}
}

func TestWatchUnexpectedTerminalStatus(t *testing.T) {
	var gets int32
	ts := watchStub(t, "t1", []watchStep{{`{"stage":"custom","status":"unknown","terminal":true}`, `[]`}}, &gets)
	defer ts.Close()
	_, err := runWatch(t, ts.URL, true, "1s")
	if err == nil || !strings.Contains(err.Error(), "unexpected terminal task status") {
		t.Fatalf("error=%v", err)
	}
}

func TestWatchCustomFailureStates(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		for _, status := range []string{"blocked", "rejected"} {
			var gets int32
			attrs := fmt.Sprintf(`{"stage":"custom-stop","status":%q,"terminal":%t,"blocked_reason":"needs intervention","awaiting_human_gate":false,"awaiting_human_gate_stage":null}`, status, status == "rejected")
			ts := watchStub(t, "t1", []watchStep{{attrs, `[]`}}, &gets)
			got, err := runWatch(t, ts.URL, jsonMode, "1s")
			ts.Close()
			if err == nil || !strings.Contains(err.Error(), "task "+status) || gets != 1 {
				t.Fatalf("status=%s error=%v polls=%d", status, err, gets)
			}
			if status == "blocked" && !strings.Contains(err.Error(), "needs intervention") {
				t.Fatal(err)
			}
			if jsonMode {
				var result struct {
					Attributes TaskAttrs `json:"attributes"`
				}
				if json.Unmarshal([]byte(got), &result) != nil || result.Attributes.Status != status {
					t.Fatal(got)
				}
			}
		}
	}
}

func TestCreateWatchHumanGateContract(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	for _, jsonMode := range []bool{false, true} {
		var gets, posts int32
		steps := []watchStep{
			{`{"stage":"first-gate","status":"implementing","terminal":false,"awaiting_human_gate":true,"awaiting_human_gate_stage":"first-gate"}`, `[]`},
			{`{"stage":"second-gate","status":"implementing","terminal":false,"awaiting_human_gate":true,"awaiting_human_gate_stage":"second-gate"}`, `[]`},
			{`{"stage":"second-gate","status":"implementing","terminal":false,"awaiting_human_gate":true,"awaiting_human_gate_stage":"second-gate"}`, `[]`},
			{`{"stage":"merging","status":"implementing","terminal":false,"awaiting_human_gate":false,"awaiting_human_gate_stage":null}`, `[]`},
			{`{"stage":"finished","status":"completed","terminal":true,"awaiting_human_gate":false,"awaiting_human_gate_stage":null}`, `[]`},
		}
		stub := watchStub(t, "t1", steps, &gets)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if violations := schema.CheckRequest(r, endpoints); len(violations) != 0 {
				t.Errorf("request violations: %v", violations)
			}
			if r.Method == http.MethodPost {
				atomic.AddInt32(&posts, 1)
				w.Header().Set("Content-Type", "application/vnd.api+json")
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"data":{"type":"tasks","id":"t1","attributes":{"stage":"first-gate","status":"implementing","terminal":false,"awaiting_human_gate":true,"awaiting_human_gate_stage":"first-gate"}}}`)
				return
			}
			stub.Config.Handler.ServeHTTP(w, r)
		}))
		var out, diagnostics bytes.Buffer
		cmd := createCmd()
		cmd.SetContext(makeCtx(t, ts.URL, "tok", jsonMode, &out))
		cmd.SetOut(&out)
		cmd.SetErr(&diagnostics)
		for key, value := range map[string]string{"project-id": "proj1", "kind": "feature", "title": "Watch gate", "watch": "true", "interval": "1ms", "timeout": "1s"} {
			if err := cmd.Flags().Set(key, value); err != nil {
				t.Fatal(err)
			}
		}
		err := cmd.RunE(cmd, nil)
		ts.Close()
		stub.Close()
		if err != nil || posts != 1 || gets != 5 {
			t.Fatalf("error=%v posts=%d gets=%d", err, posts, gets)
		}
		progress := out.String()
		if jsonMode {
			progress = diagnostics.String()
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
		}
		if strings.Count(progress, "waiting for approval at stage") != 2 {
			t.Fatal(progress)
		}
	}
}
