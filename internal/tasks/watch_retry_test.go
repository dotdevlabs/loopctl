package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

func TestWatchRetryClassification(t *testing.T) {
	for _, status := range []int{400, 401, 402, 403, 404, 408, 429, 499, 500, 502, 599, 600} {
		err := &apiclient.HTTPError{StatusCode: status, Err: errors.New("detail")}
		want := status == 408 || status == 429 || status >= 500 && status <= 599
		if retryableWatchError(fmt.Errorf("wrapped: %w", err)) != want {
			t.Errorf("status %d", status)
		}
	}
	for _, err := range []error{io.EOF, io.ErrUnexpectedEOF, context.DeadlineExceeded, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")}, &net.DNSError{IsTimeout: true}} {
		if !retryableWatchError(fmt.Errorf("wrapped: %w", err)) {
			t.Errorf("not retried: %v", err)
		}
	}
	for _, err := range []error{context.Canceled, errors.New("local"), &json.SyntaxError{}, &url.Error{Op: "Get", Err: errors.New("unsupported protocol")}} {
		if retryableWatchError(err) {
			t.Errorf("retried: %v", err)
		}
	}
}
func TestWatchBackoff(t *testing.T) {
	for _, tc := range []struct{ prev, cap, want time.Duration }{
		{0, time.Millisecond, time.Millisecond}, {0, 10 * time.Second, time.Second},
		{time.Second, 10 * time.Second, 2 * time.Second}, {8 * time.Second, 10 * time.Second, 10 * time.Second},
		{time.Duration(1 << 62), time.Duration(1<<63 - 1), time.Duration(1<<63 - 1)},
	} {
		if got := nextWatchDelay(tc.prev, tc.cap); got != tc.want {
			t.Errorf("%v: %v", tc, got)
		}
	}
}
func TestWatchRetriesServer(t *testing.T) {
	for _, endpoint := range []string{"task", "activities"} {
		for _, failure := range []int{0, 408, 429, 502} {
			for _, failAt := range []int32{1, 2} {
				for _, jsonMode := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%d/at=%d/json=%v", endpoint, failure, failAt, jsonMode), func(t *testing.T) {
						var calls atomic.Int32
						ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							target := strings.HasSuffix(r.URL.Path, "/activities") == (endpoint == "activities")
							if target {
								n := calls.Add(1)
								if n == failAt {
									if failure == 0 {
										w.Header().Set("Content-Length", "100")
										_, _ = io.WriteString(w, "{")
										return
									}
									w.WriteHeader(failure)
									_, _ = io.WriteString(w, "temporary")
									return
								}
							}
							if strings.HasSuffix(r.URL.Path, "/activities") {
								_, _ = io.WriteString(w, `{"data":[]}`)
								return
							}
							stage := "implementing"
							if calls.Load() >= 3 {
								stage = "completed"
							}
							_, _ = fmt.Fprintf(w, `{"data":{"id":"t1","attributes":{"stage":%q}}}`, stage)
						}))
						defer ts.Close()
						var out, diagnostics bytes.Buffer
						ctx, cancel := context.WithTimeout(makeCtx(t, ts.URL, "tok", jsonMode, &out), time.Second)
						defer cancel()
						err := watchTask(ctx, ctxutil.ActiveContextFrom(ctx), &out, &diagnostics, jsonMode, "t1", "1ms", "")
						if err != nil {
							t.Fatal(err)
						}
						if calls.Load() < 3 || strings.Count(diagnostics.String(), "watch retry") != 1 {
							t.Fatalf("calls=%d diagnostics=%s", calls.Load(), diagnostics.String())
						}
						if jsonMode && !json.Valid(out.Bytes()) {
							t.Fatalf("invalid JSON: %s", out.String())
						}
					})
				}
			}
		}
	}

}

func TestWatchPermanentAndDeadline(t *testing.T) {
	for _, mode := range []string{"404", "stall", "retry"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if mode == "stall" {
					<-r.Context().Done()
					return
				}
				status := 404
				if mode == "retry" {
					status = 502
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "specific detail")
			}))
			defer ts.Close()
			var out, diagnostics bytes.Buffer
			ctx := makeCtx(t, ts.URL, "tok", true, &out)
			start := time.Now()
			err := watchTask(ctx, ctxutil.ActiveContextFrom(ctx), &out, &diagnostics, true, "t1", "1ms", "30ms")
			if err == nil {
				t.Fatal("expected error")
			}
			if time.Since(start) > time.Second {
				t.Fatal("timeout did not bound requests")
			}
			if mode == "404" {
				if !strings.Contains(err.Error(), "specific detail") || calls.Load() != 1 || diagnostics.Len() != 0 {
					t.Fatalf("%v calls=%d stderr=%s", err, calls.Load(), diagnostics.String())
				}
			} else if !strings.Contains(err.Error(), "watch timed out") || !json.Valid(out.Bytes()) {
				t.Fatalf("%v %s", err, out.String())
			}
		})
	}
}
func TestWatchValidationAndCancellation(t *testing.T) {
	for _, tc := range []struct{ interval, timeout string }{{"0", ""}, {"-1s", ""}, {"bad", ""}, {"1s", "0"}, {"1s", "-1s"}, {"1s", "bad"}} {
		err := watchTask(context.Background(), nil, io.Discard, io.Discard, false, "t1", tc.interval, tc.timeout)
		if err == nil {
			t.Errorf("accepted %v", tc)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := watchTask(ctx, nil, io.Discard, io.Discard, false, "t1", "1s", ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCreateWatchRetriesWithoutRecreating(t *testing.T) {
	var creates, gets atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			creates.Add(1)
			_, _ = io.WriteString(w, `{"data":{"id":"t1","attributes":{"stage":"pending"}}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/activities") {
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}
		if gets.Add(1) == 1 {
			w.WriteHeader(429)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"id":"t1","attributes":{"stage":"completed"}}}`)
	}))
	defer ts.Close()
	var out, diagnostics bytes.Buffer
	cmd := createCmd()
	ctx, cancel := context.WithTimeout(makeCtx(t, ts.URL, "tok", false, &out), time.Second)
	defer cancel()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostics)
	for key, value := range map[string]string{"watch": "true", "interval": "1ms", "project-id": "p1", "kind": "feature", "title": "test", "description": "test"} {
		if err := cmd.Flags().Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if creates.Load() != 1 || gets.Load() != 2 || !strings.Contains(diagnostics.String(), "watch retry") {
		t.Fatalf("creates=%d gets=%d stderr=%s", creates.Load(), gets.Load(), diagnostics.String())
	}
}

func TestWatchActivityPaginationRetryPreservesSeenState(t *testing.T) {
	var polls, pages atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/activities") {
			if polls.Load() == 1 {
				_, _ = io.WriteString(w, `{"data":[]}`)
				return
			}
			if r.URL.RawQuery == "" {
				_, _ = io.WriteString(w, `{"data":[],"links":{"next":"/api/tasks/t1/activities?page=2"}}`)
				return
			}
			if pages.Add(1) == 1 {
				w.WriteHeader(502)
				return
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"a1","attributes":{"action":"container.provisioned"}}]}`)
			return
		}
		stage := "implementing"
		if polls.Add(1) >= 4 {
			stage = "completed"
		}
		_, _ = fmt.Fprintf(w, `{"data":{"id":"t1","attributes":{"stage":%q}}}`, stage)
	}))
	defer ts.Close()
	var out, diagnostics bytes.Buffer
	ctx, cancel := context.WithTimeout(makeCtx(t, ts.URL, "tok", false, &out), time.Second)
	defer cancel()
	if err := watchTask(ctx, ctxutil.ActiveContextFrom(ctx), &out, &diagnostics, false, "t1", "1ms", ""); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "container.provisioned") != 1 || strings.Count(diagnostics.String(), "watch retry") != 1 {
		t.Fatalf("stdout=%s stderr=%s", out.String(), diagnostics.String())
	}
}

func TestWatchParentCancellation(t *testing.T) {
	for _, stall := range []bool{false, true} {
		t.Run(fmt.Sprint(stall), func(t *testing.T) {
			started := make(chan struct{}, 1)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case started <- struct{}{}:
				default:
				}
				if stall {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(502)
			}))
			defer ts.Close()
			var out, diagnostics bytes.Buffer
			ctx, cancel := context.WithCancel(makeCtx(t, ts.URL, "tok", false, &out))
			defer cancel()
			result := make(chan error, 1)
			go func() {
				result <- watchTask(ctx, ctxutil.ActiveContextFrom(ctx), &out, &diagnostics, false, "t1", "1h", "")
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not interrupt watch")
			}
		})
	}
}
