package direct

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"reviewsrv/pkg/rest"

	"github.com/stretchr/testify/require"
)

func fullGroups() map[string]groupData {
	g := make(map[string]groupData)
	for _, rt := range reviewTypes {
		g[rt] = groupData{summary: "summary " + rt, isAccepted: true, markdown: "body for " + rt}
	}
	return g
}

func TestCheckGroupsComplete(t *testing.T) {
	// Complete groups, no findings -> ok.
	require.NoError(t, checkGroupsComplete(fullGroups(), nil))

	// Missing a markdown body.
	g := fullGroups()
	g["code"] = groupData{summary: "s", isAccepted: true, markdown: ""}
	require.ErrorContains(t, checkGroupsComplete(g, nil), "missing group")

	// Missing a summary.
	g = fullGroups()
	g["tests"] = groupData{summary: "", isAccepted: true, markdown: "b"}
	require.ErrorContains(t, checkGroupsComplete(g, nil), "missing summary")

	// More markdown findings than issues.
	g = fullGroups()
	g["code"] = groupData{summary: "s", isAccepted: false, markdown: "### C1. one\n### C2. two\n"}
	require.ErrorContains(t, checkGroupsComplete(g, nil), "findings")
	require.NoError(t, checkGroupsComplete(g, []rest.ReviewDraftIssue{{LocalID: "C1"}, {LocalID: "C2"}}))

	// A group never set at all.
	require.ErrorContains(t, checkGroupsComplete(map[string]groupData{}, nil), "missing group")
}

func TestIncrementalSubmitFlow(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry()
	b := newReviewBuilder()
	_, setGroup := setGroupTool(b)
	_, addIssues := addIssuesTool(b)
	_, submit := submitReviewTool(dir, b, reg)

	// set_group for all five groups (each markdown carries one finding).
	for _, rt := range reviewTypes {
		args := fmt.Sprintf(`{"reviewType":%q,"summary":"sum %s","isAccepted":false,"markdown":"### C1. issue in %s\nbody"}`, rt, rt, rt)
		out, err := call(t, setGroup, args)
		require.NoError(t, err)
		require.Contains(t, out, rt)
	}

	// Submit before issues -> rejected (5 findings, 0 issues).
	_, err := call(t, submit, `{"review":{"description":"ok"}}`)
	require.ErrorContains(t, err, "findings")
	require.False(t, reg.Submitted())

	// Add the five matching issues.
	parts := make([]string, len(reviewTypes))
	for i, rt := range reviewTypes {
		parts[i] = fmt.Sprintf(`{"localId":"C1","severity":"medium","title":"t","description":"d","file":"x.go","lines":"1","fileType":%q}`, rt)
	}
	out, err := call(t, addIssues, `{"issues":[`+strings.Join(parts, ",")+`]}`)
	require.NoError(t, err)
	require.Contains(t, out, "total 5")

	// Now finalize.
	_, err = call(t, submit, `{"review":{"description":"done","effortMinutes":30,"aiSlopScore":0.1}}`)
	require.NoError(t, err)
	require.True(t, reg.Submitted())

	// review.json + all R*.md written.
	data, err := os.ReadFile(filepath.Join(dir, "review.json"))
	require.NoError(t, err)
	require.Contains(t, string(data), "\"reviewType\": \"architecture\"")
	require.Contains(t, string(data), "\"localId\": \"C1\"")
	for _, prefix := range []string{"R1", "R2", "R3", "R4", "R5"} {
		_, err := os.Stat(filepath.Join(dir, prefix+mdSuffix))
		require.NoError(t, err, "missing %s", prefix)
	}
}

func TestSubmitTolerantOfStringifiedReview(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry()
	b := newReviewBuilder()
	_, setGroup := setGroupTool(b)
	_, submit := submitReviewTool(dir, b, reg)
	for _, rt := range reviewTypes {
		_, err := call(t, setGroup, fmt.Sprintf(`{"reviewType":%q,"summary":"s","isAccepted":true,"markdown":"body"}`, rt))
		require.NoError(t, err)
	}
	// "review" sent as a JSON-encoded string (double-encoding) — must still finalize.
	_, err := call(t, submit, `{"review":"{\"description\":\"done\",\"effortMinutes\":30,\"aiSlopScore\":0.05}"}`)
	require.NoError(t, err)
	require.True(t, reg.Submitted())
	data, _ := os.ReadFile(filepath.Join(dir, "review.json"))
	require.Contains(t, string(data), "\"description\": \"done\"")
}

func TestSetGroupRejectsInvalid(t *testing.T) {
	b := newReviewBuilder()
	_, setGroup := setGroupTool(b)
	_, err := call(t, setGroup, `{"reviewType":"bogus","summary":"s","isAccepted":true,"markdown":"b"}`)
	require.ErrorContains(t, err, "invalid reviewType")
	_, err = call(t, setGroup, `{"reviewType":"code","summary":"","isAccepted":true,"markdown":"b"}`)
	require.ErrorContains(t, err, "required")
}
