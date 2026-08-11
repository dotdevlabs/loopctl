package apiclient_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

type testData struct {
	Value string `json:"value"`
}

func activeCtx(t *testing.T, serverURL string) *config.Context {
	t.Helper()
	return &config.Context{BaseURL: serverURL, Token: "tok"}
}

func TestPostEnvelopeSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"value":"hello"}}`)
	}))
	defer ts.Close()

	env, err := apiclient.PostEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if env.Data.Value != "hello" {
		t.Errorf("expected value=hello; got: %q", env.Data.Value)
	}
}

func TestPostEnvelopeErrorsArray(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":["slug is invalid","name can't be blank"]}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error for 422")
	}
	if !strings.Contains(err.Error(), "slug is invalid") {
		t.Errorf("expected 'slug is invalid' in error; got: %v", err)
	}
	if !strings.Contains(err.Error(), "name can't be blank") {
		t.Errorf("expected 'name can't be blank' in error; got: %v", err)
	}
}

func TestPostEnvelopeMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"message":"validation failed"}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected 'validation failed'; got: %v", err)
	}
}

func TestPostEnvelopeErrorField(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"bad request"}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected 'bad request'; got: %v", err)
	}
}

func TestPostEnvelopeBareStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer ts.Close()

	_, err := apiclient.PostEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Unprocessable Entity") {
		t.Errorf("expected 'Unprocessable Entity'; got: %v", err)
	}
}

func TestPostEnvelopeSetsAuthHeader(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"value":"ok"}}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("expected 'Bearer tok'; got: %q", gotAuth)
	}
}

func TestPostEnvelopeContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"value":"ok"}}`)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := apiclient.PostEnvelope[testData](ctx, activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled; got: %v", err)
	}
}

func TestPatchJSONAPISingleSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH; got: %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected JSON:API Accept; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"42","attributes":{"value":"patched"}}}`)
	}))
	defer ts.Close()

	res, err := apiclient.PatchJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test/42", map[string]string{"value": "patched"})
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if res.ID != "42" {
		t.Errorf("expected ID=42; got: %q", res.ID)
	}
	if res.Attributes.Value != "patched" {
		t.Errorf("expected value=patched; got: %q", res.Attributes.Value)
	}
}

func TestPatchJSONAPISingleJSONAPIErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"value is invalid"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.PatchJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test/1", nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "value is invalid") {
		t.Errorf("expected 'value is invalid' in error; got: %v", err)
	}
}

func TestPatchJSONAPISingle404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"not found"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.PatchJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test/999", nil)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error; got: %v", err)
	}
}

func TestPatchJSONAPISingleContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"1","attributes":{"value":"ok"}}}`)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := apiclient.PatchJSONAPISingle[testData](ctx, activeCtx(t, ts.URL), "/api/test/1", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled; got: %v", err)
	}
}

func TestGetJSONAPISingleSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET; got: %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected JSON:API Accept; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"7","attributes":{"value":"found"}}}`)
	}))
	defer ts.Close()

	res, err := apiclient.GetJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test/7")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if res.ID != "7" {
		t.Errorf("expected ID=7; got: %q", res.ID)
	}
	if res.Attributes.Value != "found" {
		t.Errorf("expected value=found; got: %q", res.Attributes.Value)
	}
}

func TestGetJSONAPISingleNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"record not found"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.GetJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test/99")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "record not found") {
		t.Errorf("expected 'record not found' in error; got: %v", err)
	}
}

func TestGetJSONAPISingleBareStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := apiclient.GetJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test/1")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "Internal Server Error") {
		t.Errorf("expected 'Internal Server Error' in error; got: %v", err)
	}
}

func TestGetJSONAPICollectionSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected JSON:API Accept; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"1","attributes":{"value":"first"}},{"type":"items","id":"2","attributes":{"value":"second"}}]}`)
	}))
	defer ts.Close()

	col, err := apiclient.GetJSONAPICollection[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if len(col.Data) != 2 {
		t.Fatalf("expected 2 items; got: %d", len(col.Data))
	}
	if col.Data[0].ID != "1" || col.Data[0].Attributes.Value != "first" {
		t.Errorf("first item wrong; got id=%q value=%q", col.Data[0].ID, col.Data[0].Attributes.Value)
	}
	if col.Data[1].ID != "2" || col.Data[1].Attributes.Value != "second" {
		t.Errorf("second item wrong; got id=%q value=%q", col.Data[1].ID, col.Data[1].Attributes.Value)
	}
}

func TestGetJSONAPICollectionError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"403","detail":"access denied"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.GetJSONAPICollection[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected 'access denied' in error; got: %v", err)
	}
}

func TestGetEnvelopeSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected application/json Accept; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"value":"envelope-val"}}`)
	}))
	defer ts.Close()

	env, err := apiclient.GetEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if env.Data.Value != "envelope-val" {
		t.Errorf("expected value=envelope-val; got: %q", env.Data.Value)
	}
}

func TestGetEnvelopeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"message":"token expired"}`)
	}))
	defer ts.Close()

	_, err := apiclient.GetEnvelope[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "token expired") {
		t.Errorf("expected 'token expired' in error; got: %v", err)
	}
}

func TestGetJSONSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected application/json Accept; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"value":"direct"}`)
	}))
	defer ts.Close()

	result, err := apiclient.GetJSON[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if result.Value != "direct" {
		t.Errorf("expected value=direct; got: %q", result.Value)
	}
}

func TestGetJSONError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"invalid param"}`)
	}))
	defer ts.Close()

	_, err := apiclient.GetJSON[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err == nil {
		t.Fatal("expected error on 400")
	}
	if !strings.Contains(err.Error(), "invalid param") {
		t.Errorf("expected 'invalid param' in error; got: %v", err)
	}
}

func TestPostJSONAPISingleSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST; got: %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected JSON:API Accept; got: %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("Content-Type") != "application/vnd.api+json" {
			t.Errorf("expected JSON:API Content-Type; got: %s", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"new1","attributes":{"value":"created"}}}`)
	}))
	defer ts.Close()

	res, err := apiclient.PostJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", map[string]string{"value": "x"})
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if res.ID != "new1" {
		t.Errorf("expected ID=new1; got: %q", res.ID)
	}
	if res.Attributes.Value != "created" {
		t.Errorf("expected value=created; got: %q", res.Attributes.Value)
	}
}

func TestPostJSONAPISingleError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"title can't be blank"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "title can't be blank") {
		t.Errorf("expected 'title can't be blank' in error; got: %v", err)
	}
}

func TestPostJSONAPISingleErrorFlatJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"unsupported format"}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostJSONAPISingle[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error on 400")
	}
	// flat error body with no JSON:API errors array: falls back to http.StatusText
	if !strings.Contains(err.Error(), "Bad Request") {
		t.Errorf("expected 'Bad Request' fallback in error for flat body; got: %v", err)
	}
}

func TestVerboseLogging(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"title is required"}]}`)
	}))
	defer ts.Close()

	var verboseBuf strings.Builder
	ctx := apiclient.WithVerbose(context.Background(), &verboseBuf)

	_, err := apiclient.PostEnvelope[testData](ctx, activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}

	verbose := verboseBuf.String()
	if !strings.Contains(verbose, "POST") {
		t.Errorf("expected POST in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/test") {
		t.Errorf("expected /api/test in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "422") {
		t.Errorf("expected 422 in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "title is required") {
		t.Errorf("expected error body in verbose output; got: %q", verbose)
	}
}

func TestPostJSONSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST; got: %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept application/json; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"value":"flat"}`)
	}))
	defer ts.Close()

	result, err := apiclient.PostJSON[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if result.Value != "flat" {
		t.Errorf("expected value=flat; got: %q", result.Value)
	}
}

func TestPostJSONNoBody(t *testing.T) {
	var gotContentLength string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentLength = r.Header.Get("Content-Length")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	var zero testData
	result, err := apiclient.PostJSON[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err != nil {
		t.Fatalf("expected no error for no-content; got: %v", err)
	}
	if result != zero {
		t.Errorf("expected zero value for empty response; got: %+v", result)
	}
	if gotContentLength != "" && gotContentLength != "0" {
		t.Errorf("expected no Content-Length or 0 for nil body; got: %s", gotContentLength)
	}
}

func TestPostJSONError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"task already cancelled"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostJSON[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "task already cancelled") {
		t.Errorf("expected 'task already cancelled' in error; got: %v", err)
	}
}

func TestPostJSONBodyJSONAPIResponseSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST; got: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json; got: %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected Accept application/vnd.api+json; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"x1","attributes":{"value":"created"}}}`)
	}))
	defer ts.Close()

	res, err := apiclient.PostJSONBodyJSONAPIResponse[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if res.ID != "x1" {
		t.Errorf("expected ID=x1; got: %q", res.ID)
	}
	if res.Attributes.Value != "created" {
		t.Errorf("expected value=created; got: %q", res.Attributes.Value)
	}
}

func TestPostJSONBodyJSONAPIResponseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"name is taken"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.PostJSONBodyJSONAPIResponse[testData](context.Background(), activeCtx(t, ts.URL), "/api/test", nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "name is taken") {
		t.Errorf("expected 'name is taken' in error; got: %v", err)
	}
}

func TestPatchJSONBodyJSONAPIResponseSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH; got: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json; got: %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "application/vnd.api+json" {
			t.Errorf("expected Accept application/vnd.api+json; got: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"value":"updated"}}}`)
	}))
	defer ts.Close()

	res, err := apiclient.PatchJSONBodyJSONAPIResponse[testData](context.Background(), activeCtx(t, ts.URL), "/api/pipelines/p1", map[string]string{"name": "updated"})
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if res.ID != "p1" {
		t.Errorf("expected ID=p1; got: %q", res.ID)
	}
	if res.Attributes.Value != "updated" {
		t.Errorf("expected value=updated; got: %q", res.Attributes.Value)
	}
}

func TestPatchJSONBodyJSONAPIResponseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"422","detail":"name is taken"}]}`)
	}))
	defer ts.Close()

	_, err := apiclient.PatchJSONBodyJSONAPIResponse[testData](context.Background(), activeCtx(t, ts.URL), "/api/pipelines/p1", nil)
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(err.Error(), "name is taken") {
		t.Errorf("expected 'name is taken' in error; got: %v", err)
	}
}

func TestPatchJSONBodyJSONAPIResponseContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"pipelines","id":"p1","attributes":{"value":"ok"}}}`)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := apiclient.PatchJSONBodyJSONAPIResponse[testData](ctx, activeCtx(t, ts.URL), "/api/pipelines/p1", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled; got: %v", err)
	}
}

func TestVerboseLoggingGetJSONAPISingle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"5","attributes":{"value":"ok"}}}`)
	}))
	defer ts.Close()

	var verboseBuf strings.Builder
	ctx := apiclient.WithVerbose(context.Background(), &verboseBuf)

	_, err := apiclient.GetJSONAPISingle[testData](ctx, activeCtx(t, ts.URL), "/api/test/5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verbose := verboseBuf.String()
	if !strings.Contains(verbose, "GET") {
		t.Errorf("expected GET in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "200") {
		t.Errorf("expected 200 in verbose output; got: %q", verbose)
	}
	if !strings.Contains(verbose, "/api/test/5") {
		t.Errorf("expected path in verbose output; got: %q", verbose)
	}
}

func TestDeleteJSONAPISuccess(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	err := apiclient.DeleteJSONAPI(context.Background(), activeCtx(t, ts.URL), "/api/account_pipeline_defaults/feature")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE; got: %s", gotMethod)
	}
	if gotPath != "/api/account_pipeline_defaults/feature" {
		t.Errorf("expected /api/account_pipeline_defaults/feature; got: %s", gotPath)
	}
}

func TestDeleteJSONAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"errors":[{"status":"404","detail":"task kind not found"}]}`)
	}))
	defer ts.Close()

	err := apiclient.DeleteJSONAPI(context.Background(), activeCtx(t, ts.URL), "/api/account_pipeline_defaults/unknown")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "task kind not found") {
		t.Errorf("expected detail in error; got: %v", err)
	}
}

func TestDeleteJSONAPIBareStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	err := apiclient.DeleteJSONAPI(context.Background(), activeCtx(t, ts.URL), "/api/test/1")
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("expected 'Forbidden' in error; got: %v", err)
	}
}

func TestDeleteJSONAPIContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := apiclient.DeleteJSONAPI(ctx, activeCtx(t, ts.URL), "/api/test/1")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled; got: %v", err)
	}
}

func TestGetJSONAPICollectionPopulatesLinks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[],"links":{"first":"/api/test","next":"/api/test?page[number]=2","last":"/api/test?page[number]=3"}}`)
	}))
	defer ts.Close()

	col, err := apiclient.GetJSONAPICollection[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if col.Links.Next != "/api/test?page[number]=2" {
		t.Errorf("expected Next link; got: %q", col.Links.Next)
	}
	if col.Links.First != "/api/test" {
		t.Errorf("expected First link; got: %q", col.Links.First)
	}
}

func TestGetJSONAPICollectionAllPagesSinglePage(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"1","attributes":{"value":"only"}}],"links":{}}`)
	}))
	defer ts.Close()

	col, err := apiclient.GetJSONAPICollectionAllPages[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 request; got: %d", requestCount)
	}
	if len(col.Data) != 1 || col.Data[0].ID != "1" {
		t.Errorf("expected 1 item with id=1; got: %+v", col.Data)
	}
}

func TestGetJSONAPICollectionAllPagesFollowsNext(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprintf(w, `{"data":[{"type":"items","id":"1","attributes":{"value":"page1"}}],"links":{"next":"%s/api/test?page%%5Bnumber%%5D=2"}}`, "http://"+r.Host)
		} else {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"2","attributes":{"value":"page2"}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	col, err := apiclient.GetJSONAPICollectionAllPages[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests; got: %d", requestCount)
	}
	if len(col.Data) != 2 {
		t.Fatalf("expected 2 items; got: %d", len(col.Data))
	}
	if col.Data[0].ID != "1" || col.Data[1].ID != "2" {
		t.Errorf("unexpected items: %+v", col.Data)
	}
}

func TestGetJSONAPICollectionAllPagesRelativeNext(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		if r.URL.RawQuery == "" {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"a","attributes":{"value":"p1"}}],"links":{"next":"/api/test?page[number]=2"}}`)
		} else {
			_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"b","attributes":{"value":"p2"}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	col, err := apiclient.GetJSONAPICollectionAllPages[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests; got: %d", requestCount)
	}
	if len(col.Data) != 2 {
		t.Fatalf("expected 2 items; got: %d", len(col.Data))
	}
	if col.Data[0].ID != "a" || col.Data[1].ID != "b" {
		t.Errorf("unexpected items: %+v", col.Data)
	}
}

func TestGetJSONAPICollectionAllPagesThreePages(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		page := r.URL.Query().Get("page[number]")
		switch page {
		case "":
			_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"1","attributes":{"value":"p1"}}],"links":{"next":"/api/test?page[number]=2"}}`)
		case "2":
			_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"2","attributes":{"value":"p2"}}],"links":{"next":"/api/test?page[number]=3"}}`)
		default:
			_, _ = fmt.Fprint(w, `{"data":[{"type":"items","id":"3","attributes":{"value":"p3"}}],"links":{}}`)
		}
	}))
	defer ts.Close()

	col, err := apiclient.GetJSONAPICollectionAllPages[testData](context.Background(), activeCtx(t, ts.URL), "/api/test")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if requestCount != 3 {
		t.Errorf("expected 3 requests; got: %d", requestCount)
	}
	if len(col.Data) != 3 {
		t.Fatalf("expected 3 items; got: %d", len(col.Data))
	}
}

func TestGetJSONAPISingleFullDecodesLinksself(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"7","links":{"self":"/api/items/7"},"attributes":{"value":"v"}}}`)
	}))
	defer ts.Close()

	res, err := apiclient.GetJSONAPISingleFull[testData](context.Background(), activeCtx(t, ts.URL), "/api/items/7")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if res.ID != "7" {
		t.Errorf("expected ID=7; got: %q", res.ID)
	}
	if res.Attributes.Value != "v" {
		t.Errorf("expected value=v; got: %q", res.Attributes.Value)
	}
	if res.SelfLink != "/api/items/7" {
		t.Errorf("expected SelfLink=/api/items/7; got: %q", res.SelfLink)
	}
}

func TestGetJSONAPISingleFullNoLinksself(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprint(w, `{"data":{"type":"items","id":"5","attributes":{"value":"nolnk"}}}`)
	}))
	defer ts.Close()

	res, err := apiclient.GetJSONAPISingleFull[testData](context.Background(), activeCtx(t, ts.URL), "/api/items/5")
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if res.SelfLink != "" {
		t.Errorf("expected empty SelfLink; got: %q", res.SelfLink)
	}
	if res.ID != "5" {
		t.Errorf("expected ID=5; got: %q", res.ID)
	}
}
