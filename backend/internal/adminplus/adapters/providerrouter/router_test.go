package providerrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	newapiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/newapi/provider"
	sub2apiprovider "github.com/Wei-Shaw/sub2api/internal/adminplus/adapters/sub2api/provider"
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestRouterProbeProfileUsesNewAPIWhenBundleHasCompatibleUserHeader(t *testing.T) {
	var sawSelf bool
	var sawSub2APIProfile bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self":
			sawSelf = true
			require.Equal(t, "9", r.Header.Get("New-Api-User"))
			require.Equal(t, "9", r.Header.Get("Veloera-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":9,"username":"alice","role":1,"status":1,"quota":500000,"used_quota":0}}`))
		case "/api/v1/user/profile":
			sawSub2APIProfile = true
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	router := New(sub2apiprovider.NewSessionProfileClient(server.Client()), newapiprovider.NewClient(server.Client()))
	result, err := router.ProbeSub2APIUserProfile(context.Background(), ports.SessionProbeInput{
		SupplierID: 1,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"required_headers": map[string]any{"Veloera-User": "9"},
		},
	})

	require.NoError(t, err)
	require.True(t, sawSelf)
	require.False(t, sawSub2APIProfile)
	require.Equal(t, "new_api", result.SystemType)
	require.NotNil(t, result.BalanceCents)
	require.Equal(t, int64(100), *result.BalanceCents)
}

func TestProviderTypeFromBundleDoesNotTreatContextUserIDAsNewAPIEvidence(t *testing.T) {
	require.Empty(t, providerTypeFromBundle(map[string]any{
		"context": map[string]any{"user_id": "9"},
	}))
}

func TestRouterProbeProfileFallsBackToNewAPIWhenSub2APIEndpointIsIncompatible(t *testing.T) {
	var sawSub2APIProfile bool
	var sawSelf bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/user/profile":
			sawSub2APIProfile = true
			http.NotFound(w, r)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case "/api/user/self":
			sawSelf = true
			require.Equal(t, "42", r.Header.Get("New-Api-User"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"username":"ops","role":1,"status":1,"quota":2500000,"used_quota":1000000,"request_count":3}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	router := New(sub2apiprovider.NewSessionProfileClient(server.Client()), newapiprovider.NewClient(server.Client()))
	result, err := router.ProbeSub2APIUserProfile(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "sub2api",
			"required_headers": map[string]any{"New-Api-User": "42"},
		},
	})

	require.NoError(t, err)
	require.True(t, sawSub2APIProfile)
	require.True(t, sawSelf)
	require.Equal(t, "new_api", result.SystemType)
	require.NotNil(t, result.BalanceCents)
	require.Equal(t, int64(500), *result.BalanceCents)
	require.Equal(t, "sub2api", result.Diagnostics["fallback_from"])
	require.Equal(t, "SUPPLIER_SESSION_PROBE_BAD_STATUS", result.Diagnostics["fallback_reason"])
	require.Equal(t, server.URL+"/api/v1/user/profile", result.Diagnostics["fallback_endpoint"])
}

func TestRouterProbeProfileReturnsNewAPIErrorWhenNewAPIEvidenceFallbackFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/user/profile":
			http.NotFound(w, r)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case "/api/user/self":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"not logged in"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	router := New(sub2apiprovider.NewSessionProfileClient(server.Client()), newapiprovider.NewClient(server.Client()))
	_, err := router.ProbeSub2APIUserProfile(context.Background(), ports.SessionProbeInput{
		SupplierID: 7,
		Origin:     server.URL,
		APIBaseURL: server.URL,
		Bundle: map[string]any{
			"provider_type":    "sub2api",
			"required_headers": map[string]any{"New-Api-User": "42"},
		},
	})

	require.Error(t, err)
	appErr := infraerrors.FromError(err)
	require.Equal(t, "SUPPLIER_SESSION_PERMISSION_DENIED", appErr.Reason)
	require.Equal(t, server.URL+"/api/user/self", appErr.Metadata["endpoint"])
	require.Equal(t, "sub2api", appErr.Metadata["fallback_from"])
	require.Equal(t, "SUPPLIER_SESSION_PROBE_BAD_STATUS", appErr.Metadata["fallback_reason"])
}
