package tasks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

const recordingListResp = `{"data":[{"type":"task_recordings","id":"r1","attributes":{"recording_type":"asciinema","filename":"session.cast","stage_name":"implementing","file_available":true,"started_at":"2026-01-01T00:00:00Z","ended_at":"2026-01-01T01:00:00Z","duration":3600,"size_bytes":1024,"created_at":"2026-01-01T00:00:00Z"}}],"links":{},"meta":{}}`

const recordingGetResp = `{"data":{"type":"task_recordings","id":"r1","attributes":{"recording_type":"asciinema","filename":"session.cast","stage_name":"implementing","file_available":true,"started_at":"2026-01-01T00:00:00Z","ended_at":"2026-01-01T01:00:00Z","duration":3600,"size_bytes":1024,"created_at":"2026-01-01T00:00:00Z"}}}`

func makeCtxDryRun(t *testing.T, serverURL string) context.Context {
	t.Helper()
	r := output.New(false, "", io.Discard, io.Discard)
	ctx := context.Background()
	ctx = ctxutil.WithRenderer(ctx, r)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{DryRun: true})
	ctx = ctxutil.WithActiveContext(ctx, &config.Context{BaseURL: serverURL, Token: "tok"})
	return ctx
}

func TestRecordingsList(t *testing.T) {
	var gotPath, gotMethod, gotAuth, gotAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, recordingListResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("recordings list failed: %v", err)
	}
	if gotPath != "/api/tasks/t1/recordings" {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("unexpected method: %s", gotMethod)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("unexpected auth: %s", gotAuth)
	}
	if gotAccept != "application/vnd.api+json" {
		t.Errorf("unexpected Accept: %s", gotAccept)
	}
	got := out.String()
	if !strings.Contains(got, "r1") {
		t.Errorf("output missing recording id:\n%s", got)
	}
	if !strings.Contains(got, "asciinema") {
		t.Errorf("output missing type:\n%s", got)
	}
	if !strings.Contains(got, "session.cast") {
		t.Errorf("output missing filename:\n%s", got)
	}
}

func TestRecordingsList_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, recordingListResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)
	cmd := recordingsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("recordings list JSON failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("expected JSON envelope with data; got:\n%s", got)
	}
	if !strings.Contains(got, `"recording_type"`) {
		t.Errorf("expected recording_type in JSON output; got:\n%s", got)
	}
	if !strings.Contains(got, `"filename"`) {
		t.Errorf("expected filename in JSON output; got:\n%s", got)
	}
}

func TestRecordingsList_Paginated(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if n == 1 {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"task_recordings","id":"r1","attributes":{"recording_type":"asciinema","filename":"s1.cast","stage_name":"","file_available":true}}],"links":{"next":"%s/api/tasks/t1/recordings?page%%5Bnumber%%5D=2"},"meta":{}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"task_recordings","id":"r2","attributes":{"recording_type":"jsonl","filename":"s2.jsonl","stage_name":"","file_available":true}}],"links":{},"meta":{}}`)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1"}); err != nil {
		t.Fatalf("recordings list paginated failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (pagination); got %d", got)
	}
	got := out.String()
	if !strings.Contains(got, "r1") {
		t.Errorf("output missing r1:\n%s", got)
	}
	if !strings.Contains(got, "r2") {
		t.Errorf("output missing r2:\n%s", got)
	}
}

func TestRecordingsGet(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, recordingGetResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsGetCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1", "r1"}); err != nil {
		t.Fatalf("recordings get failed: %v", err)
	}
	if gotPath != "/api/tasks/t1/recordings/r1" {
		t.Errorf("unexpected path: %s", gotPath)
	}
	got := out.String()
	if !strings.Contains(got, "r1") {
		t.Errorf("output missing id:\n%s", got)
	}
	if !strings.Contains(got, "asciinema") {
		t.Errorf("output missing type:\n%s", got)
	}
}

func TestRecordingsGet_JSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, recordingGetResp)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", true, &out)
	cmd := recordingsGetCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1", "r1"}); err != nil {
		t.Fatalf("recordings get JSON failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"recording_type"`) {
		t.Errorf("expected recording_type in JSON output; got:\n%s", got)
	}
	if !strings.Contains(got, `"filename"`) {
		t.Errorf("expected filename in JSON output; got:\n%s", got)
	}
}

func TestRecordingsContent_Stdout(t *testing.T) {
	content := []byte("raw content bytes\x00\x01\x02")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1", "r1"}); err != nil {
		t.Fatalf("recordings content stdout failed: %v", err)
	}
	if !bytes.Equal(out.Bytes(), content) {
		t.Errorf("stdout content mismatch: got %v, want %v", out.Bytes(), content)
	}
}

func TestRecordingsContent_File(t *testing.T) {
	content := []byte("file content bytes")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.cast")

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("output", outFile)

	if err := cmd.RunE(cmd, []string{"t1", "r1"}); err != nil {
		t.Fatalf("recordings content file failed: %v", err)
	}

	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("file content mismatch: got %q, want %q", got, content)
	}
	if out.Len() != 0 {
		t.Errorf("expected no stdout output when writing to file; got: %q", out.String())
	}
}

func TestRecordingsContent_FileExists(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.cast")
	if err := os.WriteFile(existing, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("output", existing)

	err := cmd.RunE(cmd, []string{"t1", "r1"})
	if err == nil {
		t.Fatal("expected error when output file already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error; got: %v", err)
	}
	if called {
		t.Error("HTTP request should not be made when output file already exists")
	}
}

func TestRecordingsContent_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"recording not found"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"t1", "r1"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("expected not-found message; got: %v", err)
	}
}

func TestRecordingsContent_410(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"410","detail":"recording file has been deleted"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"t1", "r1"})
	if err == nil {
		t.Fatal("expected error for 410")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "deleted") && !strings.Contains(errMsg, "gone") && !strings.Contains(errMsg, "Gone") {
		t.Errorf("expected 'deleted' or 'gone' in 410 error; got: %v", err)
	}
}

func TestRecordingsContent_422(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"recording type does not support content streaming"}]}`)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, []string{"t1", "r1"})
	if err == nil {
		t.Fatal("expected error for 422")
	}
}

func TestRecordingsContent_LargeJSONL(t *testing.T) {
	var lineSlice []string
	for i := 0; i < 30; i++ {
		lineSlice = append(lineSlice, fmt.Sprintf(`{"seq":%d,"ts":1700000000.%d,"type":"o","data":"output line %d with some padding to make it longer than trivial padpadpadpad"}`, i, i, i))
	}
	content := strings.Join(lineSlice, "\n") + "\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, content)
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1", "r1"}); err != nil {
		t.Fatalf("large JSONL content failed: %v", err)
	}

	got := out.String()
	if got != content {
		t.Errorf("large JSONL content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}

	gotLines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(gotLines) < 5 {
		t.Errorf("expected at least 5 JSONL lines; got %d", len(gotLines))
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("expected final newline in JSONL output")
	}
	if len(got) < 2048 {
		t.Errorf("expected >=2KB of content; got %d bytes", len(got))
	}
}

func TestGetRawContent_RedirectStripsAuth(t *testing.T) {
	const token = "secret-bearer-token-xyz"

	var redirectTargetAuth string
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, "content")
	}))
	defer redirectTarget.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/content", http.StatusFound)
	}))
	defer origin.Close()

	activeCtx := &config.Context{BaseURL: origin.URL, Token: token}
	ctx := context.Background()

	var out bytes.Buffer
	if err := apiclient.GetRawContent(ctx, activeCtx, "/content", &out); err != nil {
		t.Fatalf("GetRawContent failed: %v", err)
	}

	if strings.Contains(redirectTargetAuth, token) {
		t.Errorf("Authorization header with token was forwarded to redirect target; got: %q", redirectTargetAuth)
	}
}

func TestRecordingsContentDryRun(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	var out bytes.Buffer
	ctx := makeCtxDryRun(t, ts.URL)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"t1", "r1"}); err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if called {
		t.Error("dry-run should not make HTTP request")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("expected dry-run message; got: %s", out.String())
	}
}

func TestRecordingsContent_NoPartialFileOnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.cast")

	var out bytes.Buffer
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := recordingsContentCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	_ = cmd.Flags().Set("output", outFile)

	err := cmd.RunE(cmd, []string{"t1", "r1"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if _, statErr := os.Stat(outFile); !os.IsNotExist(statErr) {
		t.Error("expected no partial output file after content error")
	}
}
