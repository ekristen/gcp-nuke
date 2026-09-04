package gcputil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateMaskForScopesToTheProviderBeingChanged(t *testing.T) {
	cases := []struct {
		name   string
		config *ProjectConfig
		want   string
	}{
		{
			name:   "anonymous",
			config: &ProjectConfig{SignIn: &SignInConfig{Anonymous: &ProviderConfig{Enabled: false}}},
			want:   "signIn.anonymous.enabled",
		},
		{
			name:   "email",
			config: &ProjectConfig{SignIn: &SignInConfig{Email: &ProviderConfig{Enabled: false}}},
			want:   "signIn.email.enabled",
		},
		{
			name:   "phone uses the phoneNumber api field",
			config: &ProjectConfig{SignIn: &SignInConfig{Phone: &ProviderConfig{Enabled: false}}},
			want:   "signIn.phoneNumber.enabled",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := updateMaskFor(tc.config)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected mask %q, got %q", tc.want, got)
			}
		})
	}
}

func TestUpdateMaskForRejectsAnUnscopedWrite(t *testing.T) {
	// An empty mask would let the backend treat the PATCH as a whole-config write.
	if _, err := updateMaskFor(&ProjectConfig{}); err == nil {
		t.Fatal("expected an error for a config with no updatable fields set")
	}
	if _, err := updateMaskFor(&ProjectConfig{SignIn: &SignInConfig{}}); err == nil {
		t.Fatal("expected an error for a config with an empty signIn block")
	}
}

func TestUpdateProjectConfigSendsScopedMaskAndMinimalBody(t *testing.T) {
	var gotQuery, gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("updateMask")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"projects/test/config"}`))
	}))
	defer srv.Close()

	svc := &IdentityPlatformService{client: srv.Client(), BasePath: srv.URL + "/admin/v2/"}

	_, err := svc.UpdateProjectConfig(context.Background(), "test", &ProjectConfig{
		SignIn: &SignInConfig{Anonymous: &ProviderConfig{Enabled: false}},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if gotQuery != "signIn.anonymous.enabled" {
		t.Fatalf("expected updateMask signIn.anonymous.enabled, got %q", gotQuery)
	}

	// enabled:false must survive marshaling, and the output-only name plus the
	// unrelated autodeleteAnonymousUsers toggle must not be sent.
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body was not valid json: %v", err)
	}
	if _, ok := body["name"]; ok {
		t.Fatalf("output-only name was sent in the update body: %s", gotBody)
	}
	if _, ok := body["autodeleteAnonymousUsers"]; ok {
		t.Fatalf("autodeleteAnonymousUsers was sent in the update body: %s", gotBody)
	}

	signIn, ok := body["signIn"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a signIn block, got: %s", gotBody)
	}
	anonymous, ok := signIn["anonymous"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a signIn.anonymous block, got: %s", gotBody)
	}
	if enabled, ok := anonymous["enabled"].(bool); !ok || enabled {
		t.Fatalf("expected enabled:false to be sent, got: %s", gotBody)
	}
}

func TestUpdateProjectConfigSurfacesTheErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"message":"EMAIL_TEMPLATE_UPDATE_NOT_ALLOWED","status":"INVALID_ARGUMENT"}}`))
	}))
	defer srv.Close()

	svc := &IdentityPlatformService{client: srv.Client(), BasePath: srv.URL + "/admin/v2/"}

	_, err := svc.UpdateProjectConfig(context.Background(), "test", &ProjectConfig{
		SignIn: &SignInConfig{Anonymous: &ProviderConfig{Enabled: false}},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "EMAIL_TEMPLATE_UPDATE_NOT_ALLOWED") {
		t.Fatalf("expected the api message in the error, got: %v", err)
	}
}
