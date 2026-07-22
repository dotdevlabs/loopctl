package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/ctlkit/pkg/version"
)

func newAICmd(ver version.Info, workflows []airef.Workflow) *cobra.Command {
	return &cobra.Command{
		Use:   "ai",
		Short: "Print AI-ingestible command reference",
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")
			ref := airef.Build(cmd.Root(), ver.Product, ver.Version, workflows)
			if jsonMode {
				return output.JSONTo(cmd.OutOrStdout(), ref)
			}
			return renderAIMarkdown(cmd.OutOrStdout(), ref)
		},
	}
}

// renderAIMarkdown renders the reference as human-readable markdown.
// renderMarkdown in ctlkit/pkg/airef is unexported, so we inline it here.
func renderAIMarkdown(w io.Writer, ref airef.Reference) error {
	m := &markdownWriter{w: w}
	m.printf("# %s Command Reference\n\n", ref.Product)
	m.printf("Generated: %s  Version: %s\n\n", time.Now().Format("2006-01-02"), ref.Version)

	for _, cmd := range ref.Commands {
		renderCmdMarkdown(m, cmd, 2)
	}

	if len(ref.Workflows) > 0 {
		m.printf("## Common Workflows\n\n")
		for _, wf := range ref.Workflows {
			m.printf("### %s\n\n%s\n\n", wf.Name, wf.Description)
			for i, step := range wf.Steps {
				m.printf("%d. %s\n", i+1, step)
			}
			m.println()
		}
	}
	return m.err
}

func renderCmdMarkdown(m *markdownWriter, cmd airef.CommandRef, depth int) {
	heading := strings.Repeat("#", depth)
	m.printf("%s %s\n\n", heading, cmd.Use)
	if cmd.Short != "" {
		m.printf("%s\n\n", cmd.Short)
	}
	if len(cmd.Flags) > 0 {
		m.printf("**Flags:**\n\n")
		m.printf("| Flag | Type | Default | Required | Description |\n")
		m.printf("|------|------|---------|----------|-------------|\n")
		for _, f := range cmd.Flags {
			req := ""
			if f.Required {
				req = "yes"
			}
			m.printf("| --%s | %s | %s | %s | %s |\n", f.Name, f.Type, f.Default, req, f.Usage)
		}
		m.println()
	}
	for _, sub := range cmd.Subcommands {
		renderCmdMarkdown(m, sub, depth+1)
	}
}

type markdownWriter struct {
	w   io.Writer
	err error
}

func (m *markdownWriter) printf(format string, args ...any) {
	if m.err != nil {
		return
	}
	_, m.err = fmt.Fprintf(m.w, format, args...)
}

func (m *markdownWriter) println() {
	if m.err != nil {
		return
	}
	_, m.err = fmt.Fprintln(m.w)
}
