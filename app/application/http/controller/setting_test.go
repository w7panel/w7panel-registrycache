package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
)

func TestBuildCommonRegistryCacheListFiltersSensitiveFields(t *testing.T) {
	list := map[string]*logic.RegistryCacheSetting{
		"global": {
			CacheRegistry: logic.CacheStorageRegistry{
				ServerUrl: "https://cache-user:cache-pass@cache.example.com/private",
				Username:  "cache-user",
				Password:  "cache-secret",
			},
			Extra: map[string]interface{}{
				"page_setting": map[string]interface{}{
					"markdown":       "# Public home",
					"copyright":      "Public copyright",
					"internal_token": "page-secret",
				},
			},
		},
		"mirror.example.com": {
			CacheRegistry: logic.CacheStorageRegistry{
				ServerUrl: "https://cache.example.com/private",
				Username:  "repository-user",
				Password:  "repository-secret",
			},
			RegistrySources: []logic.RegistrySource{
				{
					ServerUrl: "https://origin-user:origin-pass@origin.example.com/v2?token=source-secret#fragment",
					Username:  "source-user",
					Password:  "source-password",
				},
			},
			OriginRegistry: logic.RegistrySource{
				ServerUrl: "https://origin-fallback-user:origin-fallback-pass@fallback.example.com/v2?token=fallback-secret",
				Username:  "origin-fallback-user",
				Password:  "origin-fallback-password",
			},
			Extra: map[string]interface{}{
				"internal_token": "site-secret",
			},
		},
	}

	result := buildCommonRegistryCacheList(list)
	if result["global"].Extra == nil || result["global"].Extra.PageSetting == nil {
		t.Fatal("global page settings must be included in the public list")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal common list: %v", err)
	}
	response := string(encoded)
	if !strings.Contains(response, "https://origin.example.com/v2") {
		t.Fatalf("sanitized source registry is missing from response: %s", response)
	}
	if !strings.Contains(response, "# Public home") {
		t.Fatalf("global page settings are missing from response: %s", response)
	}
	for _, sensitive := range []string{
		"page-secret",
		"cache-user",
		"cache-pass",
		"cache-secret",
		"origin-user",
		"origin-pass",
		"source-secret",
		"source-user",
		"source-password",
		"origin-fallback-user",
		"origin-fallback-pass",
		"origin-fallback-password",
		"fallback-secret",
		"repository-user",
		"repository-secret",
		"site-secret",
	} {
		if strings.Contains(response, sensitive) {
			t.Fatalf("public response contains sensitive value %q: %s", sensitive, response)
		}
	}
}

func TestSanitizeCommonURL(t *testing.T) {
	want := "https://origin.example.com/v2"
	if got := sanitizeCommonURL("https://user:pass@origin.example.com/v2?token=secret#internal"); got != want {
		t.Fatalf("sanitizeCommonURL() = %q, want %q", got, want)
	}
	if got := sanitizeCommonURL("invalid"); got != "" {
		t.Fatalf("sanitizeCommonURL(invalid) = %q, want empty", got)
	}
}
