package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/client/auth/bearer"
)

func TestNewAuthorizerWithModifierExchangesScopedToken(t *testing.T) {
	t.Parallel()

	var tokenServiceURL string
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.Header().Set("Www-Authenticate", `Bearer realm="`+tokenServiceURL+`",service="cache-registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer registryServer.Close()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer client-token" {
			t.Fatalf("unexpected upstream authorization: %s", got)
		}
		if got := r.URL.Query().Get("service"); got != "cache-registry" {
			t.Fatalf("unexpected service: %s", got)
		}
		if got := r.URL.Query().Get("scope"); got != "repository:cache/library/nginx:pull" {
			t.Fatalf("unexpected scope: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token": "scoped-token",
		})
	}))
	defer tokenServer.Close()
	tokenServiceURL = tokenServer.URL

	authorizer := NewAuthorizerWithModifier(bearer.NewLocalAuthorizer("client-token"), true)
	req, err := http.NewRequest(http.MethodGet, registryServer.URL+"/v2/cache/library/nginx/manifests/latest", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	if err = authorizer.Modify(req); err != nil {
		t.Fatalf("Modify() error = %v", err)
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer scoped-token" {
		t.Fatalf("unexpected request authorization: %s", authHeader)
	}
	if strings.Contains(authHeader, "client-token") {
		t.Fatalf("client token should not be forwarded directly: %s", authHeader)
	}
}
