package tasks

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const transitionListResp = `{"data":[` +
	`{"type":"task_transitions","id":"tr1","attributes":{"from_stage":"planning","to_stage":"implementing","custom_stage_name":"","metadata":{},"created_at":"2026-01-01T00:00:00Z"}},` +
	`{"type":"task_transitions","id":"tr2","attributes":{"from_stage":"implementing","to_stage":"reviewing","custom_stage_name":"code review","metadata":{},"created_at":"2026-01-02T00:00:00Z"}}` +
	`],"links":{},"meta":{}}`

func TestTransitionsList(t *testing.T) {
	var gotPath, gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, transitionListResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := transitionsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("transitions list failed: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("expected GET; got %s", gotMethod)
	}
	if gotPath != "/api/tasks/t1/transitions" {
		t.Errorf("expected /api/tasks/t1/transitions; got %s", gotPath)
	}
	got := out.String()
	if !strings.Contains(got, "tr1") {
		t.Errorf("output missing tr1:\n%s", got)
	}
	if !strings.Contains(got, "planning") {
		t.Errorf("output missing from_stage 'planning':\n%s", got)
	}
	if !strings.Contains(got, "implementing") {
		t.Errorf("output missing to_stage 'implementing':\n%s", got)
	}
	if !strings.Contains(got, "tr2") {
		t.Errorf("output missing tr2:\n%s", got)
	}
	if !strings.Contains(got, "code review") {
		t.Errorf("output missing custom_stage_name 'code review':\n%s", got)
	}
}

func TestTransitionsList_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, transitionListResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)
	cmd := transitionsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("transitions list JSON failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("expected JSON envelope with data; got:\n%s", got)
	}
	if !strings.Contains(got, `"from_stage"`) {
		t.Errorf("expected from_stage in JSON output; got:\n%s", got)
	}
}

func TestTransitionsList_Paginated(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if n == 1 {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"task_transitions","id":"tr1","attributes":{"from_stage":"planning","to_stage":"implementing","custom_stage_name":"","metadata":{},"created_at":"2026-01-01T00:00:00Z"}}],"links":{"next":"%s/api/tasks/t1/transitions?page%%5Bnumber%%5D=2"},"meta":{}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_transitions","id":"tr2","attributes":{"from_stage":"implementing","to_stage":"reviewing","custom_stage_name":"","metadata":{},"created_at":"2026-01-02T00:00:00Z"}}],"links":{},"meta":{}}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := transitionsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("transitions list paginated failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (pagination); got %d", got)
	}
	got := out.String()
	if !strings.Contains(got, "tr1") {
		t.Errorf("output missing tr1:\n%s", got)
	}
	if !strings.Contains(got, "tr2") {
		t.Errorf("output missing tr2:\n%s", got)
	}
}

func TestTransitionsList_TaskIDEscaped(t *testing.T) {
	var gotRawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawPath = r.URL.RawPath
		if gotRawPath == "" {
			gotRawPath = r.URL.Path
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := transitionsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"task/with/slash"})

	if gotRawPath != "/api/tasks/task%2Fwith%2Fslash/transitions" {
		t.Errorf("expected path-escaped task id; got %s", gotRawPath)
	}
}
