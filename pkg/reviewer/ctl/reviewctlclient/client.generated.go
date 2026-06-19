// Code generated from jsonrpc schema by rpcgen v2.5.x with golang v1.1.1; DO NOT EDIT.

package reviewctlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/vmkteam/appkit"
	"github.com/vmkteam/zenrpc/v2"
)

const name = "reviewctlclient"

var (
	// Always import time package. Generated models can contain time.Time fields.
	_ time.Time
)

type Client struct {
	rpcClient *rpcClient

	Reviewctl *svcReviewctl
}

func NewClient(endpoint string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Second * 30}
	}
	c := &Client{
		rpcClient: newRPCClient(endpoint, httpClient),
	}

	c.Reviewctl = newClientReviewctl(c.rpcClient)

	return c
}

type Config struct {
	ApiBaseURL      string              `json:"apiBaseURL"`
	ApiProvider     string              `json:"apiProvider"`
	Effort          string              `json:"effort"`
	Model           string              `json:"model"`
	Params          RunnerProfileParams `json:"params"`
	Runner          string              `json:"runner"`
	RunnerProfileID int                 `json:"runnerProfileId"`
	Title           string              `json:"title"`
	Token           string              `json:"token"`
}

type ReviewSetup struct {
	Judge   *Config  `json:"judge,omitempty"`
	Panel   []Config `json:"panel"`
	Primary *Config  `json:"primary,omitempty"`
}

type RunnerProfileParams struct {
	AllowDangerousPermissions bool `json:"allowDangerousPermissions"`
}

type svcReviewctl struct {
	client *rpcClient
}

func newClientReviewctl(client *rpcClient) *svcReviewctl {
	return &svcReviewctl{
		client: client,
	}
}

var (
	ErrReviewctlFusionPrompt400 = zenrpc.NewError(400, fmt.Errorf("invalid project key"))
)

// FusionPrompt returns the built-in judge/synthesizer prompt for multi-review.
// The algorithm is project-agnostic, so the same built-in is served for every
// project; the projectKey is validated for a consistent contract with Prompt and
// to leave room for a per-project override later.
func (c *svcReviewctl) FusionPrompt(ctx context.Context, projectKey string) (res string, err error) {
	_req := struct {
		ProjectKey string
	}{
		ProjectKey: projectKey,
	}

	err = c.client.call(ctx, "reviewctl.FusionPrompt", _req, &res)

	switch v := err.(type) {
	case *zenrpc.Error:
		if v.Code == 400 {
			err = ErrReviewctlFusionPrompt400
		}
	}

	return
}

var (
	ErrReviewctlPrompt400 = zenrpc.NewError(400, fmt.Errorf("invalid project key"))
	ErrReviewctlPrompt500 = zenrpc.NewError(500, fmt.Errorf("internal error"))
)

// Prompt assembles and returns the review prompt for a project key.
func (c *svcReviewctl) Prompt(ctx context.Context, projectKey string) (res string, err error) {
	_req := struct {
		ProjectKey string
	}{
		ProjectKey: projectKey,
	}

	err = c.client.call(ctx, "reviewctl.Prompt", _req, &res)

	switch v := err.(type) {
	case *zenrpc.Error:
		if v.Code == 400 {
			err = ErrReviewctlPrompt400
		}
		if v.Code == 500 {
			err = ErrReviewctlPrompt500
		}
	}

	return
}

var (
	ErrReviewctlReviewConfig400 = zenrpc.NewError(400, fmt.Errorf("invalid project key"))
	ErrReviewctlReviewConfig404 = zenrpc.NewError(404, fmt.Errorf("no runner profile for project (and no default)"))
	ErrReviewctlReviewConfig500 = zenrpc.NewError(500, fmt.Errorf("internal error"))
)

// ReviewConfig resolves the multi-review panel for a project key: the primary
// runner (pinned profile or default), the additional panel members
// (runnerProfileIds) and the optional judge (judgeRunnerProfileId). Each config
// includes the real token. A nil judge means single review via the primary.
func (c *svcReviewctl) ReviewConfig(ctx context.Context, projectKey string) (res *ReviewSetup, err error) {
	_req := struct {
		ProjectKey string
	}{
		ProjectKey: projectKey,
	}

	err = c.client.call(ctx, "reviewctl.ReviewConfig", _req, &res)

	switch v := err.(type) {
	case *zenrpc.Error:
		if v.Code == 400 {
			err = ErrReviewctlReviewConfig400
		}
		if v.Code == 404 {
			err = ErrReviewctlReviewConfig404
		}
		if v.Code == 500 {
			err = ErrReviewctlReviewConfig500
		}
	}

	return
}

type rpcClient struct {
	endpoint string
	cl       *http.Client

	requestID uint64
}

func newRPCClient(endpoint string, httpClient *http.Client) *rpcClient {
	return &rpcClient{
		endpoint: endpoint,
		cl:       httpClient,
	}
}

func (rc *rpcClient) call(ctx context.Context, methodName string, request, result interface{}) error {
	// encode params
	bts, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode params: %w", err)
	}

	requestID := atomic.AddUint64(&rc.requestID, 1)
	requestIDBts := json.RawMessage(strconv.Itoa(int(requestID)))

	req := zenrpc.Request{
		Version: zenrpc.Version,
		ID:      &requestIDBts,
		Method:  methodName,
		Params:  bts,
	}

	ctx = appkit.NewCallerNameContext(ctx, name)

	res, err := rc.Exec(ctx, req)
	if err != nil {
		return err
	}

	if res == nil {
		return nil
	}

	if res.Error != nil {
		return res.Error
	}

	if res.Result == nil {
		return nil
	}

	if result == nil {
		return nil
	}

	return json.Unmarshal(*res.Result, result)
}

// Exec makes http request to jsonrpc endpoint and returns json rpc response.
func (rc *rpcClient) Exec(ctx context.Context, rpcReq zenrpc.Request) (*zenrpc.Response, error) {
	if appkit.NotificationFromContext(ctx) {
		rpcReq.ID = nil
	}

	c, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("json marshal call failed: %w", err)
	}

	buf := bytes.NewReader(c)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rc.endpoint, buf)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	appkit.SetXRequestIDFromCtx(ctx, req)

	// Do request
	resp, err := rc.cl.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("make request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response (%d)", resp.StatusCode)
	}

	var zresp zenrpc.Response
	if rpcReq.ID == nil {
		return &zresp, nil
	}

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response body (%s) read failed: %w", bb, err)
	}

	if err = json.Unmarshal(bb, &zresp); err != nil {
		return nil, fmt.Errorf("json decode failed (%s): %w", bb, err)
	}

	return &zresp, nil
}
