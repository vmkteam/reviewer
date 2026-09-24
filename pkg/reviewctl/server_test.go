package reviewctl

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	zm "github.com/vmkteam/zenrpc-middleware"
	"github.com/vmkteam/zenrpc/v2"
)

func TestMaskKeysHidesProjectKeyInLoggedParams(t *testing.T) {
	const key = "11111111-2222-3333-4444-555555555555"
	for _, params := range []string{
		`{"projectKey":"` + key + `","tokenEnv":true}`,
		`["` + key + `",true]`, // positional
	} {
		var buf bytes.Buffer
		pf := maskKeys(slog.New(slog.NewJSONHandler(&buf, nil)).InfoContext)
		var handled json.RawMessage
		h := zm.WithSLog(pf, zm.DefaultServerName, nil)(func(_ context.Context, _ string, p json.RawMessage) zenrpc.Response {
			handled = p
			return zenrpc.Response{}
		})

		h(t.Context(), "ReviewConfig", json.RawMessage(params))
		logged := buf.String()
		require.NotEmpty(t, logged)
		assert.NotContains(t, logged, key)
		assert.Contains(t, logged, "11111111…")
		assert.JSONEq(t, params, string(handled), "the method gets the real key")
	}
}
