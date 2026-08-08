package schema

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

func TestShowCmd_Table(t *testing.T) {
	var buf bytes.Buffer
	renderer := output.New(false, "", &buf, io.Discard)
	ctx := ctxutil.WithRenderer(context.Background(), renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{})

	cmd := showCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("showCmd failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "METHOD") {
		t.Errorf("table missing METHOD header; got:\n%s", got)
	}
	if !strings.Contains(got, "PATH") {
		t.Errorf("table missing PATH header; got:\n%s", got)
	}
	if !strings.Contains(got, "/api/tasks") {
		t.Errorf("table missing /api/tasks path; got:\n%s", got)
	}
}

func TestShowCmd_JSON(t *testing.T) {
	var buf bytes.Buffer
	renderer := output.New(true, "", &buf, io.Discard)
	ctx := ctxutil.WithRenderer(context.Background(), renderer)
	ctx = ctxutil.WithGlobalFlags(ctx, ctxutil.GlobalFlags{JSON: true})

	cmd := showCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("showCmd failed: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, `"data"`) {
		t.Errorf("JSON missing 'data' key; got:\n%s", got)
	}
	if !strings.Contains(got, `"http_method"`) {
		t.Errorf("JSON missing 'http_method'; got:\n%s", got)
	}
	if !strings.Contains(got, "/api/tasks") {
		t.Errorf("JSON missing /api/tasks; got:\n%s", got)
	}
}
