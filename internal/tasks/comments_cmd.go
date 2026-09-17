package tasks

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// CommentAttrs holds the attributes for a task comment.
type CommentAttrs struct {
	Body         string `json:"body"`
	CommentType  string `json:"comment_type"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	CreatedAt    string `json:"created_at"`
}

// commentsCmd returns the "comments" parent command.
func commentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "Manage task comments",
	}
	cmd.AddCommand(commentsListCmd())
	cmd.AddCommand(commentsCreateCmd())
	return cmd
}

func commentsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-id>",
		Short: "List comments on a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/comments"
			col, err := apiclient.GetJSONAPICollectionAllPages[CommentAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "BODY"},
				{Header: "COMMENT_TYPE"},
				{Header: "CREATED_AT"},
			}
			rows := make([][]string, len(col.Data))
			for i, c := range col.Data {
				a := c.Attributes
				truncated := a.Body
				if len(truncated) > 60 {
					truncated = truncated[:57] + "..."
				}
				rows[i] = []string{c.ID, truncated, a.CommentType, a.CreatedAt}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func commentsCreateCmd() *cobra.Command {
	var (
		body         string
		inputTokens  int
		outputTokens int
	)

	cmd := &cobra.Command{
		Use:   "create <task-id>",
		Short: "Post a comment on a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"dry-run: would POST /api/tasks/%s/comments {body=%q}\n", args[0], body)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			commentBody := map[string]any{"body": body}
			if cmd.Flags().Changed("input-tokens") {
				commentBody["input_tokens"] = inputTokens
			}
			if cmd.Flags().Changed("output-tokens") {
				commentBody["output_tokens"] = outputTokens
			}
			reqBody := map[string]any{"comment": commentBody}

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/comments"
			res, err := apiclient.PostJSONBodyJSONAPIResponse[CommentAttrs](ctx, activeCtx, path, reqBody)
			if err != nil {
				return err
			}

			c := res.Attributes
			truncated := c.Body
			if len(truncated) > 60 {
				truncated = truncated[:57] + "..."
			}
			cols := []output.Column{
				{Header: "ID"},
				{Header: "BODY"},
				{Header: "CREATED_AT"},
			}
			rows := [][]string{{res.ID, truncated, c.CreatedAt}}
			return r.Render(cols, rows, res)
		},
	}

	cmd.Flags().StringVar(&body, "body", "", "Comment body text")
	cmd.Flags().IntVar(&inputTokens, "input-tokens", 0, "Input token count for billing")
	cmd.Flags().IntVar(&outputTokens, "output-tokens", 0, "Output token count for billing")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}
