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
	if !strings.Contains(err.Error(), "HTTP 422") {
		t.Errorf("expected 'HTTP 422'; got: %v", err)
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
