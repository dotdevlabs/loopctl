package tasks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/schema"
)

func TestConformance_RecordingsContent_ErrorDetails(t *testing.T) {
	details, err := schema.SpecEnum("#/components/schemas/RecordingContentError/properties/errors/items/properties/detail")
	if err != nil {
		t.Fatal(err)
	}
	for i, detail := range details {
		for _, file := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/file=%t", i, file), func(t *testing.T) {
				status := http.StatusNotFound
				if i == 2 {
					status = http.StatusGone
				}
				if i == 3 {
					status = http.StatusUnprocessableEntity
				}
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/vnd.api+json")
					w.WriteHeader(status)
					_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"status": fmt.Sprint(status), "title": http.StatusText(status), "detail": detail}}})
				}))
				defer ts.Close()
				var out bytes.Buffer
				cmd := recordingsContentCmd()
				cmd.SetContext(makeCtx(t, ts.URL, "tok", false, &out))
				cmd.SetOut(&out)
				dir := t.TempDir()
				if file {
					if err := cmd.Flags().Set("output", filepath.Join(dir, "recording")); err != nil {
						t.Fatal(err)
					}
				}
				err := cmd.RunE(cmd, []string{"t1", "r1"})
				wantCode := clierror.CodeNotFound
				if status == http.StatusUnprocessableEntity {
					wantCode = clierror.CodeBadRequest
				}
				cliErr, ok := err.(*clierror.CLIError)
				if !ok || cliErr.Message != detail || cliErr.Code != wantCode {
					t.Fatalf("want detail %q, got %v", detail, err)
				}
				if out.Len() != 0 {
					t.Fatalf("unexpected stdout: %s", out.String())
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("failed download left files: %v, %v", entries, err)
				}
			})
		}
	}
}

func TestConformance_RecordingsList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := recordingsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks recordings list: %v", violations)
	}
}

func TestConformance_RecordingsGet(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"task_recordings","id":"r1","attributes":{"recording_type":"asciinema","filename":"s.cast","stage_name":"","file_available":true,"created_at":"2026-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := recordingsGetCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1", "r1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks recordings get: %v", violations)
	}
}

func TestConformance_RecordingsContent(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, "raw bytes")
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"t1", "r1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for tasks recordings content: %v", violations)
	}
}

func TestConformance_RecordingsList_Paginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if n == 1 {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"task_recordings","id":"r1","attributes":{"recording_type":"asciinema","filename":"p1.cast","stage_name":"","file_available":true}}],"links":{"next":"%s/api/tasks/t1/recordings?page%%5Bnumber%%5D=2"},"meta":{}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_recordings","id":"r2","attributes":{"recording_type":"jsonl","filename":"p2.jsonl","stage_name":"","file_available":true}}],"links":{},"meta":{}}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.RunE(cmd, []string{"t1"})

	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests for pagination; got %d", got)
	}
	result := out.String()
	if !strings.Contains(result, "r1") || !strings.Contains(result, "r2") {
		t.Errorf("both pages should appear in output; got:\n%s", result)
	}
}

func TestConformance_RecordingsContent_NullMetadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tasks/t1/recordings":
			w.Header().Set("Content-Type", "application/vnd.api+json")
			// Recording with file_available=false and no content_url
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_recordings","id":"r1","attributes":{"recording_type":"asciinema","filename":"s.cast","stage_name":"","file_available":false,"content_url":""}}],"links":{},"meta":{}}`)
		case "/api/tasks/t1/recordings/r1/content":
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := recordingsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Errorf("list with null metadata should not panic; got: %v", err)
	}
}

func TestConformance_RecordingsContent_DeletedError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"410","detail":"recording file has been deleted"}]}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)

	err := cmd.RunE(cmd, []string{"t1", "r1"})
	if err == nil {
		t.Fatal("expected nonzero error for 410 (deleted recording)")
	}
}

func TestConformance_RecordingsContent_SecretSafeDiag(t *testing.T) {
	const token = "super-secret-bearer-token-12345"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, "some raw content bytes")
	}))
	defer ts.Close()

	var verboseBuf bytes.Buffer
	ctx := makeCtx(t, ts.URL, token, false, io.Discard)
	ctx = apiclient.WithVerbose(ctx, &verboseBuf)

	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)

	if err := cmd.RunE(cmd, []string{"t1", "r1"}); err != nil {
		t.Fatalf("content retrieval failed: %v", err)
	}

	verboseOutput := verboseBuf.String()
	if strings.Contains(verboseOutput, token) {
		t.Errorf("verbose log must not contain the bearer token; got:\n%s", verboseOutput)
	}
	if strings.Contains(verboseOutput, "raw content bytes") {
		t.Errorf("verbose log must not echo raw content bytes; got:\n%s", verboseOutput)
	}
}
