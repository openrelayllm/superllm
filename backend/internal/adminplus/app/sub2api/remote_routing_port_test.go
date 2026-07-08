package sub2api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoteAdminAPIRoutingPortEnsureAccountInGroupUsesAdminAPI(t *testing.T) {
	var gotAPIKey string
	var gotPayload map[string]any
	var putCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		require.Equal(t, "/api/v1/admin/accounts/42", r.URL.Path)
		switch r.Method {
		case http.MethodGet:
			writeRemoteSub2APISuccess(t, w, map[string]any{
				"id":          42,
				"name":        "local-upstream",
				"platform":    "openai",
				"type":        "apikey",
				"status":      "active",
				"schedulable": true,
				"group_ids":   []int64{10},
			})
		case http.MethodPut:
			putCalled = true
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotPayload))
			writeRemoteSub2APISuccess(t, w, map[string]any{
				"id":          42,
				"name":        "local-upstream",
				"platform":    "openai",
				"type":        "apikey",
				"status":      "active",
				"schedulable": true,
				"group_ids":   []int64{10, 20},
			})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	port, err := NewRemoteAdminAPIRoutingPort(server.URL, "admin-secret", server.Client(), nil)
	require.NoError(t, err)

	account, err := port.EnsureAccountInGroup(context.Background(), 42, 20)

	require.NoError(t, err)
	require.Equal(t, "admin-secret", gotAPIKey)
	require.True(t, putCalled)
	require.Equal(t, []any{float64(10), float64(20)}, gotPayload["group_ids"])
	require.Equal(t, true, gotPayload["confirm_mixed_channel_risk"])
	require.Equal(t, []int64{10, 20}, account.GroupIDs)
}

func TestRemoteAdminAPIRoutingPortEnsureAccountInGroupSkipsExistingBinding(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		require.Equal(t, http.MethodGet, r.Method)
		writeRemoteSub2APISuccess(t, w, map[string]any{
			"id":          42,
			"name":        "local-upstream",
			"platform":    "openai",
			"type":        "apikey",
			"status":      "active",
			"schedulable": true,
			"group_ids":   []int64{10, 20},
		})
	}))
	defer server.Close()

	port, err := NewRemoteAdminAPIRoutingPort(server.URL, "admin-secret", server.Client(), nil)
	require.NoError(t, err)

	account, err := port.EnsureAccountInGroup(context.Background(), 42, 20)

	require.NoError(t, err)
	require.Equal(t, 1, requestCount)
	require.Equal(t, []int64{10, 20}, account.GroupIDs)
}

func TestRemoteAdminAPIRoutingPortSetAccountSchedulableUsesDedicatedAdminAPI(t *testing.T) {
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/accounts/42/schedulable", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "admin-secret", r.Header.Get("x-api-key"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotPayload))
		writeRemoteSub2APISuccess(t, w, map[string]any{
			"id":          42,
			"name":        "local-upstream",
			"platform":    "openai",
			"type":        "apikey",
			"status":      "active",
			"schedulable": false,
			"group_ids":   []int64{10},
		})
	}))
	defer server.Close()

	port, err := NewRemoteAdminAPIRoutingPort(server.URL, "admin-secret", server.Client(), nil)
	require.NoError(t, err)

	account, err := port.SetAccountSchedulable(context.Background(), 42, false, "capacity_watch")

	require.NoError(t, err)
	require.Equal(t, false, gotPayload["schedulable"])
	require.Equal(t, "capacity_watch", gotPayload["reason"])
	require.False(t, account.Schedulable)
}

func writeRemoteSub2APISuccess(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"code":    0,
		"message": "success",
		"data":    data,
	}))
}
