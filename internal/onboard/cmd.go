// Package onboard implements the "onboard" command for unauthenticated machine registration.
package onboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

const defaultBaseURL = "https://app.loopcontrol.ai"

// RegistrationAttrs holds the attributes returned by POST /api/registrations.
type RegistrationAttrs struct {
	AccountName  string `json:"account_name"`
	AccountSlug  string `json:"account_slug"`
	OwnerEmail   string `json:"owner_email"`
	APIToken     string `json:"api_token"`
	APITokenName string `json:"api_token_name"`
}

// NewCmd returns the "onboard" command.
func NewCmd() *cobra.Command {
	var (
		baseURL     string
		contextName string
		email       string
		tokenName   string
		accountName string
	)

	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Register this machine and persist API credentials",
		// No-op PersistentPreRunE prevents Cobra from walking up to root's
		// PersistentPreRunE, which would fail when no auth context exists yet.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			attrs, err := postRegistration(ctx, baseURL, email, tokenName, accountName)
			if err != nil {
				return err
			}

			cfg, err := config.Load("loopctl")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			cfg.Contexts[contextName] = config.Context{
				BaseURL: baseURL,
				Token:   attrs.APIToken,
			}
			if cfg.CurrentContext == "" {
				cfg.CurrentContext = contextName
			}

			if err := config.Save("loopctl", cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			jsonMode, _ := cmd.Root().PersistentFlags().GetBool("json")
			if jsonMode {
				return output.JSONTo(cmd.OutOrStdout(), struct {
					Context    string `json:"context"`
					Token      string `json:"token"`
					TokenLabel string `json:"token_label"`
				}{
					Context:    contextName,
					Token:      attrs.APIToken,
					TokenLabel: attrs.APITokenName,
				})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Registered. Context %q saved.\n", contextName)
			return err
		},
	}

	cmd.Flags().StringVar(&baseURL, "url", defaultBaseURL, "LoopControl API base URL")
	cmd.Flags().StringVar(&contextName, "name", "default", "Context name to store credentials under")
	cmd.Flags().StringVar(&email, "email", "", "Email address for the new account (required)")
	cmd.Flags().StringVar(&tokenName, "token-name", "", "Label for the generated API token")
	cmd.Flags().StringVar(&accountName, "account-name", "", "Account name")

	_ = cmd.MarkFlagRequired("email")

	return cmd
}

// postRegistration calls POST {baseURL}/api/registrations (unauthenticated) and returns
// the registration attributes from the response.
func postRegistration(ctx context.Context, baseURL, email, tokenName, accountName string) (*RegistrationAttrs, error) {
	body := map[string]any{
		"registration": buildRegistration(email, tokenName, accountName),
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}

	fullURL := strings.TrimRight(baseURL, "/") + "/api/registrations"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(encoded)) //#nosec G107 -- URL comes from --url flag, same pattern as apiclient.go
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		return nil, fmt.Errorf("closing response body: %w", closeErr)
	}
	if readErr != nil {
		return nil, fmt.Errorf("reading response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("registration failed (%d): %s", resp.StatusCode, extractError(respBody, resp.StatusCode))
	}

	var doc struct {
		Data struct {
			Attributes RegistrationAttrs `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &doc); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if doc.Data.Attributes.APIToken == "" {
		return nil, fmt.Errorf("missing token in registration response")
	}

	return &doc.Data.Attributes, nil
}

func buildRegistration(email, tokenName, accountName string) map[string]any {
	reg := map[string]any{
		"email_address": email,
	}
	if tokenName != "" {
		reg["api_token_name"] = tokenName
	}
	if accountName != "" {
		reg["account_name"] = accountName
	}
	return reg
}

func extractError(body []byte, status int) string {
	if len(body) > 0 {
		var errDoc struct {
			Errors []struct {
				Detail string `json:"detail"`
				Title  string `json:"title"`
			} `json:"errors"`
		}
		if json.Unmarshal(body, &errDoc) == nil && len(errDoc.Errors) > 0 {
			parts := make([]string, 0, len(errDoc.Errors))
			for _, e := range errDoc.Errors {
				if e.Detail != "" {
					parts = append(parts, e.Detail)
				} else if e.Title != "" {
					parts = append(parts, e.Title)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		}
	}
	return http.StatusText(status)
}
