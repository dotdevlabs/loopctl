package environments

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/loopctl/internal/schema"
)

func loadSchemaOrSkip(t *testing.T) []schema.EndpointAttrs {
	t.Helper()
	endpoints, err := schema.Load()
	if err != nil {
		t.Skipf("cannot load schema fixture: %v", err)
	}
	return endpoints
}

func TestConformance_EnvironmentsList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{"current_page":1,"page_count":1,"per_page":20,"total_count":0}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments list: %v", violations)
	}
}

func TestConformance_EnvironmentsCreate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"environments","id":"1","attributes":{"name":"test","description":null,"shared":false,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := createCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "test")
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments create: %v", violations)
	}
}

func TestConformance_EnvironmentsGet(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"environments","id":"1","attributes":{"name":"test","description":null,"shared":false,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := getCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments get: %v", violations)
	}
}

func TestConformance_EnvironmentsUpdate(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"environments","id":"1","attributes":{"name":"updated","description":null,"shared":false,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := updateCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("name", "updated")
	_ = cmd.RunE(cmd, []string{"1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments update: %v", violations)
	}
}

func TestConformance_EnvironmentsEntriesList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{},"meta":{"current_page":1,"page_count":1,"per_page":20,"total_count":0}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := entriesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments entries list: %v", violations)
	}
}

func TestConformance_EnvironmentsEntriesAdd(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"environment_entries","id":"1","attributes":{"env_var":"MY_VAR","entry_type":"custom","service":null,"custom_value":"val","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := entriesAddCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("env-var", "MY_VAR")
	_ = cmd.Flags().Set("type", "custom")
	_ = cmd.Flags().Set("value", "val")
	_ = cmd.RunE(cmd, []string{"1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments entries add: %v", violations)
	}
}

func TestConformance_EnvironmentsEntriesChange(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"environment_entries","id":"1","attributes":{"env_var":"MY_VAR","entry_type":"custom","service":"svc","custom_value":"val","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := entriesChangeCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.Flags().Set("service", "svc")
	_ = cmd.RunE(cmd, []string{"1", "1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments entries change: %v", violations)
	}
}

func TestConformance_EnvironmentsEntriesRemove(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := entriesRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, []string{"1", "1"})

	if len(violations) != 0 {
		t.Errorf("conformance violations for environments entries remove: %v", violations)
	}
}

func TestConformance_EnvironmentsListPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"environments","id":"1","attributes":{"name":"env1","description":null,"shared":false,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"links":{"self":null}}],"links":{"next":"%s/api/environments?page%%5Bnumber%%5D=2"},"meta":{"current_page":1,"page_count":2,"per_page":20,"total_count":2}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"environments","id":"2","attributes":{"name":"env2","description":null,"shared":false,"default":false,"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"links":{"self":null}}],"links":{},"meta":{"current_page":2,"page_count":2,"per_page":20,"total_count":2}}`)
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("environments list pagination failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (2 pages); got %d", got)
	}
}

func TestConformance_EnvironmentsEntriesListPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"environment_entries","id":"1","attributes":{"env_var":"VAR1","entry_type":"custom","service":null,"custom_value":"v1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}],"links":{"next":"%s/api/environments/1/entries?page%%5Bnumber%%5D=2"},"meta":{"current_page":1,"page_count":2,"per_page":20,"total_count":2}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"environment_entries","id":"2","attributes":{"env_var":"VAR2","entry_type":"custom","service":null,"custom_value":"v2","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"},"relationships":{"credential_set":{"data":null}},"links":{"self":null}}],"links":{},"meta":{"current_page":2,"page_count":2,"per_page":20,"total_count":2}}`)
		}
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := entriesListCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)

	if err := cmd.RunE(cmd, []string{"1"}); err != nil {
		t.Fatalf("environments entries list pagination failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (2 pages); got %d", got)
	}
}
