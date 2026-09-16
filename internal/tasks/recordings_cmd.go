package tasks

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"

	"github.com/dotdevlabs/loopctl/internal/apiclient"
)

// RecordingAttrs holds the attributes for a task recording.
type RecordingAttrs struct {
	RecordingType string `json:"recording_type"`
	Filename      string `json:"filename"`
	StartedAt     string `json:"started_at"`
	EndedAt       string `json:"ended_at"`
	Duration      int    `json:"duration"`
	SizeBytes     int    `json:"size_bytes"`
	FileAvailable bool   `json:"file_available"`
	DeletedAt     string `json:"deleted_at"`
	ContentURL    string `json:"content_url"`
	ContainerID   string `json:"container_id"`
	StageName     string `json:"stage_name"`
	CreatedAt     string `json:"created_at"`
}

func recordingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recordings",
		Short: "Manage task recordings",
	}
	cmd.AddCommand(recordingsListCmd())
	cmd.AddCommand(recordingsGetCmd())
	cmd.AddCommand(recordingsContentCmd())
	return cmd
}

func recordingsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-id>",
		Short: "List recordings for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/recordings"
			col, err := apiclient.GetJSONAPICollectionAllPages[RecordingAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			cols := []output.Column{
				{Header: "ID"},
				{Header: "TYPE"},
				{Header: "FILENAME"},
				{Header: "STAGE_NAME"},
				{Header: "FILE_AVAILABLE"},
				{Header: "STARTED_AT"},
			}
			rows := make([][]string, len(col.Data))
			for i, rec := range col.Data {
				a := rec.Attributes
				rows[i] = []string{
					rec.ID,
					a.RecordingType,
					a.Filename,
					a.StageName,
					fmt.Sprintf("%v", a.FileAvailable),
					a.StartedAt,
				}
			}
			return r.Render(cols, rows, col)
		},
	}
}

func recordingsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <task-id> <recording-id>",
		Short: "Get a single recording by ID",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			activeCtx := ctxutil.ActiveContextFrom(ctx)
			r := ctxutil.RendererFrom(ctx)

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/recordings/" + url.PathEscape(args[1])
			res, err := apiclient.GetJSONAPISingle[RecordingAttrs](ctx, activeCtx, path)
			if err != nil {
				return err
			}

			a := res.Attributes
			cols := []output.Column{
				{Header: "ID"},
				{Header: "TYPE"},
				{Header: "FILENAME"},
				{Header: "STAGE_NAME"},
				{Header: "FILE_AVAILABLE"},
				{Header: "STARTED_AT"},
				{Header: "ENDED_AT"},
				{Header: "DURATION"},
				{Header: "SIZE_BYTES"},
				{Header: "CREATED_AT"},
			}
			rows := [][]string{{
				res.ID,
				a.RecordingType,
				a.Filename,
				a.StageName,
				fmt.Sprintf("%v", a.FileAvailable),
				a.StartedAt,
				a.EndedAt,
				fmt.Sprintf("%d", a.Duration),
				fmt.Sprintf("%d", a.SizeBytes),
				a.CreatedAt,
			}}
			return r.Render(cols, rows, res)
		},
	}
}

func recordingsContentCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "content <task-id> <recording-id>",
		Short: "Retrieve the raw content of a recording",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			path := "/api/tasks/" + url.PathEscape(args[0]) + "/recordings/" + url.PathEscape(args[1]) + "/content"

			if ctxutil.GlobalFlagsFrom(ctx).DryRun {
				activeCtx := ctxutil.ActiveContextFrom(ctx)
				baseURL := activeCtx.BaseURL
				if len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
					baseURL = baseURL[:len(baseURL)-1]
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would GET %s%s\n", baseURL, path)
				return nil
			}

			activeCtx := ctxutil.ActiveContextFrom(ctx)

			if outputPath == "" {
				return apiclient.GetRawContent(ctx, activeCtx, path, cmd.OutOrStdout())
			}

			if _, err := os.Stat(outputPath); err == nil {
				return clierror.New(clierror.CodeConflict, "file already exists: "+outputPath, "use a different output path or remove the existing file")
			}

			dir := filepath.Dir(outputPath)
			tmp, err := os.CreateTemp(dir, ".loopctl-rec-*")
			if err != nil {
				return fmt.Errorf("creating temp file: %w", err)
			}
			tmpName := tmp.Name()

			if err := apiclient.GetRawContent(ctx, activeCtx, path, tmp); err != nil {
				_ = tmp.Close()
				_ = os.Remove(tmpName)
				return err
			}

			size, _ := tmp.Seek(0, 2) // get written size
			if err := tmp.Close(); err != nil {
				_ = os.Remove(tmpName)
				return fmt.Errorf("closing temp file: %w", err)
			}
			if err := os.Rename(tmpName, outputPath); err != nil {
				_ = os.Remove(tmpName)
				return fmt.Errorf("writing output file: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bytes to %s\n", size, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Write content to this file path instead of stdout")
	return cmd
}
