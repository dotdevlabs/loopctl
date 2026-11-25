package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/schema"
	"github.com/spf13/cobra"
)

const jsonAPI = "application/vnd.api+json"

type responseStep struct {
	method    string
	path      string
	status    int
	overrides map[string]any
}

func response(method, path string) responseStep {
	return responseStep{method: method, path: path}
}

func responseWith(method, path string, overrides map[string]any) responseStep {
	return responseStep{method: method, path: path, overrides: overrides}
}

type responseCase struct {
	args  string
	steps []responseStep
	want  string
}

func oneResponse(args, method, path string) responseCase {
	return responseCase{args: args, steps: []responseStep{response(method, path)}}
}

func TestCommandResponseContracts(t *testing.T) {
	const task = "/api/tasks/:id"
	const pipeline = "/api/pipelines/:id"
	const projects = "/api/projects"
	billing := func(packageID string) responseStep {
		return responseStep{method: "GET", path: "/api/billing/funding_envelope", status: 402, overrides: map[string]any{
			"/errors/0/status":               "402",
			"/meta/fundable_products/0/id":   packageID,
			"/meta/fundable_products/0/kind": "topup",
			"/meta/fundable_products/0/payment_rails/human_link/params/package_id": packageID,
			"/meta/fundable_products/0/payment_rails/x402/params/package_id":       packageID,
			"/meta/fundable_products/0/payment_rails/x402/x402Version":             1,
			"/meta/fundable_products/0/payment_rails/x402/scheme":                  "exact",
			"/meta/fundable_products/0/payment_rails/x402/network":                 "arbitrum",
			"/meta/fundable_products/0/payment_rails/x402/maxAmountRequired":       "1000000",
			"/meta/fundable_products/0/payment_rails/x402/payTo":                   "0x" + strings.Repeat("1", 40),
			"/meta/fundable_products/0/payment_rails/x402/asset":                   "0x" + strings.Repeat("2", 40),
			"/meta/fundable_products/0/payment_rails/lightning/params/package_id":  packageID,
		}}
	}
	purchase := func(path, rail string) responseStep {
		return responseWith("POST", path, map[string]any{"/data/id": "purchase-1", "/data/attributes/payment_rail": rail,
			"/data/attributes/status": "pending", "/data/attributes/checkout_url": nil, "/data/attributes/failure_reason": nil})
	}
	poll := func(rail string, checkout any, status string) responseStep {
		return responseWith("GET", "/api/billing/pending_purchases/:id", map[string]any{
			"/data/id": "purchase-1", "/data/attributes/payment_rail": rail, "/data/attributes/status": status,
			"/data/attributes/checkout_url": checkout, "/data/attributes/failure_reason": nil,
		})
	}
	stage := func(name string) responseStep {
		return responseWith("GET", pipeline, map[string]any{
			"/data/relationships/stages/data/0/type":   "pipeline_stages",
			"/data/relationships/stages/data/0/id":     "stage-1",
			"/included/0/id":                           "stage-1",
			"/included/0/attributes/name":              name,
			"/included/0/attributes/custom_stage_name": name,
			"/included/0/attributes/stage_type":        "custom",
			"/included/0/attributes/agent_id":          "12",
			"/included/0/attributes/environment_id":    nil,
			"/included/0/attributes/role":              "working",
		})
	}
	cases := []responseCase{
		oneResponse("onboard --email operator@example.test", "POST", "/api/registrations"),
		{"onboard --email duplicate@example.test", []responseStep{{method: "POST", path: "/api/registrations", status: 422,
			overrides: map[string]any{"/errors/0/detail": "Email is already registered"}}}, "Email is already registered"},
		oneResponse("platforms list", "GET", "/api/platforms"),
		oneResponse("platforms get 1", "GET", "/api/platforms/:id"),
		{"agent-sizes list", []responseStep{
			responseWith("GET", "/api/agent_sizes", map[string]any{"/links/next": "http://example.test/api/agent_sizes?page%5Bnumber%5D=2", "/data/0/attributes/cpu_limit_millicores": nil}),
			responseWith("GET", "/api/agent_sizes", map[string]any{"/data/0/id": "size-page-two", "/data/0/attributes/memory_limit_mib": nil}),
		}, "size-page-two"},
		oneResponse("agents list", "GET", "/api/agents"),
		oneResponse("agents get 1", "GET", "/api/agents/:id"),
		oneResponse("agents update 1 --name agent", "PATCH", "/api/agents/:id"),
		{"credentials list", []responseStep{responseWith("GET", "/api/credential_sets", map[string]any{
			"/data/0/relationships/tasks/data/0/id": "blocked-list-task",
		})}, "blocked-list-task"},
		{"credentials get 1", []responseStep{responseWith("GET", "/api/credential_sets/:id", map[string]any{
			"/data/relationships/tasks/data/0/id": "blocked-get-task",
		})}, "blocked-get-task"},
		oneResponse("account credit --minutes 1", "POST", "/api/balance_credits"),
		oneResponse("usage", "GET", "/api/usage"),
		{"topup", []responseStep{billing("10h"), purchase("/api/billing/topups", "stripe"),
			poll("stripe", "https://checkout.example.test/session", "pending")}, "https://checkout.example.test/session"},
		{"topup --product 10h", []responseStep{billing("10h"), purchase("/api/billing/x402_payments", "usdc"),
			poll("usdc", nil, "settled")}, "Payment sent via x402"},
		{"topup --product 50h", []responseStep{billing("50h"), purchase("/api/billing/l402_payments", "lightning"),
			poll("lightning", nil, "settled")}, "Payment sent via L402"},
		oneResponse("task-kinds list", "GET", "/api/task_kinds"),
		oneResponse("task-kinds create --name custom", "POST", "/api/task_kinds"),
		oneResponse("task-kinds update 1 --name custom", "PATCH", "/api/task_kinds/:id"),
		oneResponse("task-kinds list-default-pipelines", "GET", "/api/account_pipeline_defaults"),
		oneResponse("task-kinds set-default-pipeline feature --pipeline-id 1", "PATCH", "/api/account_pipeline_defaults/:kind"),
		oneResponse("task-kinds clear-default-pipeline feature", "DELETE", "/api/account_pipeline_defaults/:kind"),
		oneResponse("pipelines list", "GET", "/api/pipelines"),
		oneResponse("pipelines create --name pipeline --kind feature", "POST", "/api/pipelines"),
		oneResponse("pipelines update 1 --name pipeline", "PATCH", pipeline),
		{"pipelines stages list 1", []responseStep{stage("first-stage")}, "first-stage"},
		{"pipelines stages add 1 --custom-stage-name ship --rework-allowed-positions=1,2", []responseStep{stage("first-stage"), response("PATCH", pipeline)}, ""},
		{"pipelines stages remove 1 old-stage", []responseStep{
			stage("old-stage"),
			response("PATCH", pipeline),
		}, ""},
		oneResponse("pipelines stages update 1 2 --rework-allowed-positions=1,2", "PATCH", "/api/pipelines/:pipeline_id/stages/:id"),
		oneResponse("pipelines stages update 1 2 --rework-allowed-positions=", "PATCH", "/api/pipelines/:pipeline_id/stages/:id"),
		oneResponse("projects list", "GET", projects),
		oneResponse("projects get 1", "GET", projects+"/:id"),
		{"projects create --name New --platform platform --pipeline pipeline", []responseStep{
			responseWith("GET", "/api/platforms", map[string]any{"/data/0/attributes/name": "platform"}),
			responseWith("GET", "/api/pipelines", map[string]any{"/data/0/attributes/name": "pipeline"}),
			response("POST", projects),
		}, ""},
		{"projects update 1 --agent-size-id 4", []responseStep{responseWith("PATCH", projects+"/:id", map[string]any{"/data/relationships/agent_size/data/id": "4"})}, `"agent_sizes"`},
		{"projects update 1 --agent-size-id=", []responseStep{responseWith("PATCH", projects+"/:id", map[string]any{"/data/relationships/agent_size/data": nil})}, `"data": null`},
		oneResponse("projects update 1 --display-name New", "PATCH", projects+"/:id"),
		oneResponse("tasks list --project-id 1", "GET", "/api/tasks"),
		oneResponse("tasks create --project-id 1 --kind feature --title New --description example", "POST", "/api/tasks"),
		{"tasks create --project-id 1 --kind feature --title New --description example --watch --timeout 1s", []responseStep{
			responseWith("POST", "/api/tasks", map[string]any{"/data/id": "1"}),
			responseWith("GET", task, map[string]any{"/data/links/self": "/api/tasks/1", "/data/attributes/status": "completed", "/data/attributes/terminal": true}),
			response("GET", "/api/tasks/:task_id/activities"),
		}, `"completed"`},
		oneResponse("tasks get 1", "GET", task),
		{"tasks update 1 --remove-depends-on 2", []responseStep{
			responseWith("GET", task, map[string]any{"/data/attributes/depends_on": []string{"2", "3"}}),
			response("PATCH", task),
		}, ""},
		{"tasks cancel 1", []responseStep{responseWith("POST", "/api/tasks/:task_id/cancellation", map[string]any{
			"/data/attributes/status": "rejected", "/data/attributes/stage": "cancel-stage",
		})}, `"stage": "cancel-stage"`},
		{"tasks cancel 2", []responseStep{{method: "POST", path: "/api/tasks/:task_id/cancellation", status: 422, overrides: map[string]any{
			"/errors/0/detail": "Cannot cancel a task that is already rejected.",
		}}}, "Cannot cancel a task that is already rejected."},
		{"tasks unblock 1", []responseStep{responseWith("POST", "/api/tasks/:task_id/unblock", map[string]any{
			"/data/attributes/status": "implementing", "/data/attributes/stage": "resume-stage",
		})}, `"stage": "resume-stage"`},
		{"tasks approve 1", []responseStep{responseWith("POST", "/api/tasks/:task_id/gate_approval", map[string]any{
			"/data/attributes/stage": "approval-stage", "/data/attributes/awaiting_human_gate": false,
			"/data/attributes/awaiting_human_gate_stage": nil,
		})}, `"stage": "approval-stage"`},
		{"tasks approve 2", []responseStep{{method: "POST", path: "/api/tasks/:task_id/gate_approval", status: 409, overrides: map[string]any{
			"/errors/0/detail": "The task is not awaiting human approval at the implementing stage.",
		}}}, "The task is not awaiting human approval at the implementing stage."},
		{"tasks watch 1 --timeout 1s", []responseStep{
			responseWith("GET", task, map[string]any{"/data/links/self": "/api/tasks/2", "/data/attributes/status": "completed", "/data/attributes/terminal": true}),
			response("GET", "/api/tasks/:task_id/activities"),
		}, `"completed"`},
		oneResponse("tasks comments list 1", "GET", "/api/tasks/:task_id/comments"),
		oneResponse("tasks comments create 1 --body hello --input-tokens 100 --output-tokens 200", "POST", "/api/tasks/:task_id/comments"),
		oneResponse("tasks todos list 1", "GET", "/api/tasks/:task_id/todos"),
		oneResponse("tasks todos create 1 --content hello", "POST", "/api/tasks/:task_id/todos"),
		oneResponse(`tasks todos create 1 --bulk-json [{"content":"first"},{"content":"second"}]`, "POST", "/api/tasks/:task_id/todos"),
		oneResponse("tasks todos update 1 2 --content hello", "PATCH", "/api/tasks/:task_id/todos/:id"),
		oneResponse("tasks recordings list 1", "GET", "/api/tasks/:task_id/recordings"),
		oneResponse("tasks recordings get 1 2", "GET", "/api/tasks/:task_id/recordings/:id"),
		oneResponse("tasks recordings content 1 2", "GET", "/api/tasks/:task_id/recordings/:recording_id/content"),
		oneResponse("tasks transitions list 1", "GET", "/api/tasks/:task_id/transitions"),
		oneResponse("tasks containers 1", "GET", "/api/tasks/:task_id/containers"),
		oneResponse("environments list", "GET", "/api/environments"),
		oneResponse("environments get 1", "GET", "/api/environments/:id"),
		oneResponse("environments create --name fixture", "POST", "/api/environments"),
		oneResponse("environments update 1 --name fixture", "PATCH", "/api/environments/:id"),
		oneResponse("environments entries list 1", "GET", "/api/environments/:environment_id/entries"),
		oneResponse("environments entries add 1 --env-var FIXTURE --type custom --value example", "POST", "/api/environments/:environment_id/entries"),
		oneResponse("environments entries change 1 2 --value example", "PATCH", "/api/environments/:environment_id/entries/:id"),
		oneResponse("environments entries remove 1 2", "DELETE", "/api/environments/:environment_id/entries/:id"),
	}
	root := buildRoot()
	coveredCommands := map[string]bool{}
	endpoints, err := schema.Load()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[schema.OperationKey]bool{}
	for _, tc := range cases {
		command, _, err := root.Find(strings.Fields(tc.args))
		if err != nil || !command.Runnable() {
			t.Fatalf("response case does not identify a runnable command: %q (%v)", tc.args, err)
		}
		coveredCommands[command.CommandPath()] = true
		for _, step := range tc.steps {
			seen[schema.OperationKey{Method: step.method, Path: step.path}] = true
		}
		t.Run(tc.args, func(t *testing.T) { runResponseCase(t, tc, endpoints) })
	}
	for key := range schema.Covered {
		if !seen[key] {
			t.Errorf("no command response case for %s %s", key.Method, key.Path)
		}
	}
	for _, command := range rootConfig().Commands {
		if command.Name() != "schema" {
			checkCommandCases(t, "loopctl", command, coveredCommands)
		}
	}
}

func checkCommandCases(t *testing.T, parent string, command *cobra.Command, covered map[string]bool) {
	t.Helper()
	path := parent + " " + command.Name()
	if command.Runnable() && !covered[path] {
		t.Errorf("no response case for command %q", path)
	}
	for _, child := range command.Commands() {
		checkCommandCases(t, path, child, covered)
	}
}

type preparedResponse struct {
	status int
	media  string
	shape  *responseShape
	body   []byte
}

func prepareResponse(t *testing.T, step responseStep) preparedResponse {
	t.Helper()
	key := schema.OperationKey{Method: step.method, Path: step.path}
	switch {
	case step.method == "DELETE":
		if err := schema.ResponseHasNoContent(key, http.StatusNoContent); err != nil {
			t.Fatal(err)
		}
		return preparedResponse{status: http.StatusNoContent}
	case strings.HasSuffix(step.path, "/content"):
		const media = "application/octet-stream"
		raw, err := schema.ResponseSchema(key, http.StatusOK, media)
		if err != nil {
			t.Fatal(err)
		}
		var shape responseShape
		if err := json.Unmarshal([]byte(raw), &shape); err != nil || shape.Type != "string" || shape.Format != "binary" {
			t.Fatalf("expected an opaque binary response, got %s (%v)", raw, err)
		}
		return preparedResponse{status: http.StatusOK, media: media, body: []byte("fixture recording")}
	}
	statuses := []int{200, 201, 202}
	if step.status != 0 {
		statuses = []int{step.status}
	}
	var schemaErr error
	for _, status := range statuses {
		for _, media := range []string{jsonAPI, "application/json"} {
			var raw string
			raw, schemaErr = schema.ResponseSchema(key, status, media)
			if schemaErr != nil {
				continue
			}
			var shape responseShape
			if err := json.Unmarshal([]byte(raw), &shape); err != nil {
				t.Fatal(err)
			}
			overrides := map[string]any{"/links/next": nil, "/links/prev": nil}
			for path, value := range step.overrides {
				overrides[path] = value
			}
			value, err := shape.example("", overrides)
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &value); err != nil {
				t.Fatal(err)
			}
			if err := shape.validate(value, ""); err != nil {
				t.Fatalf("%s %s: %v", key.Method, key.Path, err)
			}
			return preparedResponse{status: status, media: media, shape: &shape, body: body}
		}
	}
	t.Fatal(schemaErr)
	return preparedResponse{}
}

func runResponseCase(t *testing.T, tc responseCase, endpoints []schema.EndpointAttrs) {
	t.Helper()
	prepared := make([]preparedResponse, len(tc.steps))
	for i, step := range tc.steps {
		prepared[i] = prepareResponse(t, step)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/channels/transactions" && tc.args == "topup --product 50h" {
			if r.Method != "POST" || r.Header.Get("Grpc-Metadata-Macaroon") != "00" {
				t.Errorf("unexpected wallet request %s %s", r.Method, r.URL)
			}
			_, _ = fmt.Fprintf(w, `{"payment_preimage":%q}`, strings.Repeat("a", 64))
			return
		}
		i := int(requests.Add(1)) - 1
		if i >= len(tc.steps) {
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		step, fixture := tc.steps[i], prepared[i]
		if r.Method != step.method || !schema.MatchPath(step.path, r.URL.Path) {
			t.Errorf("request %d = %s %s, want %s %s", i+1, r.Method, r.URL, step.method, step.path)
		}
		if tc.args == "agent-sizes list" && i == 1 && r.URL.Query().Get("page[number]") != "2" {
			t.Errorf("next page request = %s", r.URL)
		}
		if strings.HasPrefix(tc.args, "tasks watch ") && i == 1 && r.URL.Path != "/api/tasks/2/activities" {
			t.Errorf("watch ignored task self link: %s", r.URL)
		}
		if strings.Contains(step.path, "/billing/pending_purchases/") && r.URL.Path != "/api/billing/pending_purchases/purchase-1" {
			t.Errorf("poll did not use the purchase ID: %s", r.URL)
		}
		if strings.HasSuffix(step.path, "_payments") {
			wantPackage := "10h"
			if strings.Contains(step.path, "l402") {
				wantPackage = "50h"
				if r.Header.Get("X-Api-Token") != "fixture-token" {
					t.Error("Lightning payment is missing the active account token in X-Api-Token")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if !strings.HasPrefix(r.Header.Get("Authorization"), "L402 ") {
					t.Error("Lightning payment is missing the L402 proof in Authorization")
					w.WriteHeader(http.StatusPaymentRequired)
					return
				}
			} else if r.Header.Get("X-Payment") == "" {
				t.Error("payment is missing x402 authorization")
			}
			if got := r.URL.Query().Get("package_id"); got != wantPackage {
				t.Errorf("payment package_id = %q, want %q", got, wantPackage)
			}
		}
		for _, violation := range schema.CheckRequest(r, endpoints) {
			t.Error(violation)
		}
		checkResponseCaseRequest(t, tc.args, r)
		if fixture.media != "" {
			w.Header().Set("Content-Type", fixture.media)
		}
		w.WriteHeader(fixture.status)
		if len(fixture.body) > 0 {
			_, _ = w.Write(bytes.ReplaceAll(fixture.body, []byte("http://example.test"), []byte("http://"+r.Host)))
		}
	}))
	defer server.Close()

	t.Setenv("LOOPCTL_BASE_URL", server.URL)
	t.Setenv("LOOPCTL_TOKEN", "fixture-token")
	t.Setenv("LOOPCTL_CONTEXT", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOOPCTL_ARBITRUM_PRIVATE_KEY", "")
	t.Setenv("LOOPCTL_LN_HOST", "")
	t.Setenv("LOOPCTL_LN_MACAROON_HEX", "")
	switch tc.args {
	case "topup --product 10h":
		t.Setenv("LOOPCTL_ARBITRUM_PRIVATE_KEY", strings.Repeat("0", 63)+"1")
	case "topup --product 50h":
		t.Setenv("LOOPCTL_LN_HOST", server.URL)
		t.Setenv("LOOPCTL_LN_MACAROON_HEX", "00")
	}
	var stdout, stderr bytes.Buffer
	cmd := buildRoot()
	cmd.SilenceUsage = true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	args := strings.Fields(tc.args)
	if !strings.HasPrefix(tc.args, "topup --product") {
		args = append(args, "--json")
	}
	if args[0] == "onboard" {
		args = append(args, "--url", server.URL)
	}
	cmd.SetArgs(args)
	previous := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(command *cobra.Command, args []string) error {
		if err := previous(command, args); err != nil {
			return err
		}
		ctx := ctxutil.WithActiveContext(command.Context(), &config.Context{BaseURL: server.URL, Token: "fixture-token"})
		command.SetContext(ctxutil.WithRenderer(ctx, output.New(true, "", &stdout, &stderr)))
		return nil
	}
	observed := make([]bool, len(tc.steps))
	ctx := apiclient.WithResponseObserver(t.Context(), func(resp *http.Response, path string, projection any) {
		i := int(requests.Load()) - 1
		if i < 0 || i >= len(prepared) || prepared[i].shape == nil {
			t.Errorf("unexpected JSON projection %s", path)
			return
		}
		observed[i] = true
		if err := checkProjection(prepared[i].shape, path, projection); err != nil {
			t.Errorf("%s %s: %v", resp.Request.Method, resp.Request.URL.Path, err)
		}
	})
	err := cmd.ExecuteContext(ctx)
	finalStatus := prepared[len(prepared)-1].status
	if finalStatus >= 400 && finalStatus != 402 {
		if err == nil || !strings.Contains(err.Error(), tc.want) || stdout.Len() != 0 {
			t.Fatalf("expected %q and no success output, got %v, %q", tc.want, err, &stdout)
		}
	} else if err != nil {
		t.Fatalf("command failed: %v (%s)", err, &stderr)
	}
	if got := int(requests.Load()); got != len(tc.steps) {
		t.Fatalf("got %d requests, want %d", got, len(tc.steps))
	}
	for i, fixture := range prepared {
		if fixture.shape != nil && !observed[i] {
			t.Errorf("no response decoder observed for %s %s", tc.steps[i].method, tc.steps[i].path)
		}
	}
	if err == nil && tc.want != "" && !strings.Contains(stdout.String(), tc.want) {
		t.Errorf("output does not contain %s: %s", tc.want, &stdout)
	}
	if err == nil && tc.want == "" {
		checkResponseCaseOutput(t, stdout.Bytes(), prepared[len(prepared)-1])
	}
}

func checkResponseCaseOutput(t *testing.T, output []byte, fixture preparedResponse) {
	t.Helper()
	if fixture.shape == nil {
		if len(fixture.body) > 0 && !bytes.Equal(output, fixture.body) {
			t.Errorf("recording content = %q, want original bytes %q", output, fixture.body)
		}
		return
	}
	var document struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixture.body, &document); err != nil || len(document.Data) == 0 {
		t.Fatalf("invalid fixture data: %s (%v)", fixture.body, err)
	}
	var resource struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Token string `json:"api_token"`
		} `json:"attributes"`
	}
	raw := document.Data
	if raw[0] == '[' {
		var resources []json.RawMessage
		if err := json.Unmarshal(raw, &resources); err != nil || len(resources) == 0 {
			t.Fatalf("expected a nonempty fixture collection: %s (%v)", raw, err)
		}
		raw = resources[0]
	}
	if err := json.Unmarshal(raw, &resource); err != nil {
		t.Fatal(err)
	}
	marker := resource.ID
	if resource.Type == "registrations" {
		marker = resource.Attributes.Token
	}
	if marker == "" || !bytes.Contains(output, []byte(strconv.Quote(marker))) {
		t.Errorf("output did not preserve the fixture value %q: %s", marker, output)
	}
}

func checkResponseCaseRequest(t *testing.T, args string, r *http.Request) {
	t.Helper()
	if r.ContentLength > 0 && r.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json request, got %q", r.Header.Get("Content-Type"))
	}
	if !strings.HasSuffix(r.URL.Path, "/content") && r.Header.Get("Accept") != jsonAPI {
		t.Errorf("expected JSON:API response negotiation, got %q", r.Header.Get("Accept"))
	}
	switch {
	case strings.HasPrefix(args, "pipelines stages add ") && r.Method == "PATCH":
		var request struct {
			Pipeline struct {
				Stages []struct {
					Name      string          `json:"custom_stage_name"`
					AgentID   int             `json:"agent_id"`
					Positions json.RawMessage `json:"rework_allowed_positions"`
				} `json:"stages"`
			} `json:"pipeline"`
		}
		err := json.NewDecoder(r.Body).Decode(&request)
		stages := request.Pipeline.Stages
		if err != nil || len(stages) != 2 || stages[0].Name != "first-stage" || stages[0].AgentID != 12 ||
			stages[1].Name != "ship" || string(stages[1].Positions) != "[1,2]" {
			t.Errorf("unexpected stage replacement request: %+v (%v)", stages, err)
		}
	case strings.HasPrefix(args, "pipelines stages update "):
		var request struct {
			Stage struct {
				Positions json.RawMessage `json:"rework_allowed_positions"`
			} `json:"pipeline_stage"`
		}
		want := "[1,2]"
		if strings.HasSuffix(args, "=") {
			want = "[]"
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || string(request.Stage.Positions) != want {
			t.Errorf("rework_allowed_positions = %s, want %s (%v)", request.Stage.Positions, want, err)
		}
	case strings.HasPrefix(args, "projects update "):
		var request struct {
			Project map[string]json.RawMessage `json:"project"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		want := ""
		switch {
		case strings.HasSuffix(args, "--agent-size-id 4"):
			want = `"4"`
		case strings.HasSuffix(args, "--agent-size-id="):
			want = "null"
		}
		if got := string(request.Project["agent_size_id"]); got != want {
			t.Errorf("agent_size_id on request = %q, want %q", got, want)
		}
	case strings.HasPrefix(args, "tasks cancel "), strings.HasPrefix(args, "tasks unblock "), strings.HasPrefix(args, "tasks approve "):
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 0 || r.Header.Get("Accept") != jsonAPI {
			t.Errorf("expected bodyless JSON:API action, got %q, %v, %v", body, r.Header, err)
		}
	case strings.HasPrefix(args, "tasks comments create "):
		var request struct {
			Comment      map[string]json.RawMessage `json:"comment"`
			InputTokens  int                        `json:"input_tokens"`
			OutputTokens int                        `json:"output_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Comment) != 1 ||
			request.InputTokens != 100 || request.OutputTokens != 200 {
			t.Errorf("unexpected comment token request: %+v, %v", request, err)
		}
	}
}

func TestCancellationResponseProjectionRejectsFlatStatus(t *testing.T) {
	fixture := prepareResponse(t, response("POST", "/api/tasks/:task_id/cancellation"))
	type statusProjection struct {
		Status string `json:"status"`
	}
	if err := checkProjection(fixture.shape, "/data/attributes", &statusProjection{}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonAPI)
		_, _ = w.Write(fixture.body)
	}))
	defer server.Close()
	var diagnostic error
	ctx := apiclient.WithResponseObserver(t.Context(), func(resp *http.Response, path string, projection any) {
		if err := checkProjection(fixture.shape, path, projection); err != nil {
			diagnostic = fmt.Errorf("tasks cancel: %s %s: %w", resp.Request.Method, resp.Request.URL.Path, err)
		}
	})
	active := &config.Context{BaseURL: server.URL, Token: "fixture-token"}
	if _, err := apiclient.PostJSONAPIResponse[statusProjection](ctx, active, "/api/tasks/1/cancellation", nil); err != nil {
		t.Fatal(err)
	}
	if diagnostic == nil || !strings.Contains(diagnostic.Error(), "tasks cancel: POST /api/tasks/1/cancellation: undeclared response field /status") {
		t.Fatalf("flat cancellation response projection was accepted: %v", diagnostic)
	}
}
