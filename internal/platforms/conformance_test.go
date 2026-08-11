package platforms

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestConformance_PlatformsList(t *testing.T) {
	endpoints := loadSchemaOrSkip(t)
	var violations []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		violations = schema.CheckRequest(r, endpoints)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer ts.Close()

	ctx := makeCtx(t, ts.URL, "tok", false, io.Discard)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	_ = cmd.RunE(cmd, nil)

	if len(violations) != 0 {
		t.Errorf("conformance violations for platforms list: %v", violations)
	}
}

func TestConformance_PlatformsListPaginates(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w,
				`{"data":[{"type":"platforms","id":"pf1","attributes":{"name":"rails","display_name":"Ruby on Rails"}}],"links":{"next":"%s/api/platforms?page%%5Bnumber%%5D=2"}}`,
				"http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w,
				`{"data":[{"type":"platforms","id":"pf2","attributes":{"name":"django","display_name":"Django"}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	var out strings.Builder
	ctx := makeCtx(t, ts.URL, "tok", false, &out)
	cmd := listCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("platforms list with pagination failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("expected 2 requests (2 pages); got %d", got)
	}
	result := out.String()
	if !strings.Contains(result, "rails") {
		t.Errorf("output missing rails from page 1:\n%s", result)
	}
	if !strings.Contains(result, "django") {
		t.Errorf("output missing django from page 2:\n%s", result)
	}
}
