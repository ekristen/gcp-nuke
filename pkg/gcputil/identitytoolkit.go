package gcputil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"
)

const identityPlatformBasePath = "https://identitytoolkit.googleapis.com/admin/v2/"

// ProjectConfig represents the configuration of a project in Identity Platform
type ProjectConfig struct {
	Name                     string        `json:"name"`
	AutoDeleteAnonymousUsers bool          `json:"autodeleteAnonymousUsers"`
	SignIn                   *SignInConfig `json:"signIn,omitempty"`
	MFA                      *MFAConfig    `json:"mfa,omitempty"`
}

type SignInConfig struct {
	Anonymous *ProviderConfig `json:"anonymous,omitempty"`
	Email     *ProviderConfig `json:"email,omitempty"`
	Phone     *ProviderConfig `json:"phoneNumber,omitempty"`
}

type MFAConfig struct {
	State string `json:"state"`
}

type ProviderConfig struct {
	Enabled bool `json:"enabled"`
}

// IdentityPlatformService provides methods to interact with the Identity Platform API
type IdentityPlatformService struct {
	client    *http.Client
	BasePath  string
	UserAgent string
}

// NewIdentityPlatformService creates a new Identity Platform client
func NewIdentityPlatformService(ctx context.Context, opts ...option.ClientOption) (*IdentityPlatformService, error) {
	client, endpoint, err := htransport.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	s := &IdentityPlatformService{client: client, BasePath: identityPlatformBasePath}
	if endpoint != "" {
		s.BasePath = endpoint
	}

	return s, nil
}

// GetProjectConfig retrieves the configuration of a project
func (s *IdentityPlatformService) GetProjectConfig(ctx context.Context, projectID string) (*ProjectConfig, error) {
	url := fmt.Sprintf("%sprojects/%s/config", s.BasePath, projectID)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting project config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	var config ProjectConfig
	if err = json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	return &config, nil
}

// UpdateProjectConfig updates the configuration of a project
func (s *IdentityPlatformService) UpdateProjectConfig(ctx context.Context, projectID string, config *ProjectConfig) (*ProjectConfig, error) {
	url := fmt.Sprintf("%sprojects/%s/config", s.BasePath, projectID)
	logrus.Tracef("url: %s", url)

	body, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error updating project config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	var updatedConfig ProjectConfig
	if err = json.NewDecoder(resp.Body).Decode(&updatedConfig); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	return &updatedConfig, nil
}

type ListDefaultSupportedOAuthIdpConfigsResponse struct {
	DefaultSupportedIdpConfigs []*DefaultSupportedOAuthIdpConfig `json:"defaultSupportedIdpConfigs"`
}

type DefaultSupportedOAuthIdpConfig struct {
	Name         *string `json:"name,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
	ClientID     *string `json:"clientId,omitempty"`
	ClientSecret *string `json:"clientSecret,omitempty"`
}

type ListOAuthIdpConfigsResponse struct {
	OAuthIdpConfigs []*OAuthIdpConfig `json:"oauthIdpConfigs"`
}

type OAuthIdpConfig struct {
	Name        *string `json:"name"`
	ClientId    *string `json:"clientId"`
	Issuer      *string `json:"issuer"`
	DisplayName *string `json:"displayName"`
	Enabled     *bool   `json:"enabled"`
}

// ListDefaultSupportedOAuthIdpConfigs lists the OAuth IDP configurations of a project
func (s *IdentityPlatformService) ListDefaultSupportedOAuthIdpConfigs(ctx context.Context, projectID string) (*ListDefaultSupportedOAuthIdpConfigsResponse, error) {
	url := fmt.Sprintf("%sprojects/%s/defaultSupportedIdpConfigs", s.BasePath, projectID)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting OAuth IDP configs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	var configs *ListDefaultSupportedOAuthIdpConfigsResponse
	if err = json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	return configs, nil
}

// DeleteDefaultSupportedOAuthIdpConfig deletes an OAuth IDP configuration of a project
func (s *IdentityPlatformService) DeleteDefaultSupportedOAuthIdpConfig(ctx context.Context, projectID, configID string) error {
	url := fmt.Sprintf("%sprojects/%s/defaultSupportedIdpConfigs/%s", s.BasePath, projectID, configID)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error deleting OAuth IDP config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	return nil
}

// ListOAuthIdpConfigs lists the OAuth IDP configurations of a project
func (s *IdentityPlatformService) ListOAuthIdpConfigs(ctx context.Context, projectID string) (*ListOAuthIdpConfigsResponse, error) {
	url := fmt.Sprintf("%sprojects/%s/oauthIdpConfigs", s.BasePath, projectID)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting OAuth IDP configs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	var configs *ListOAuthIdpConfigsResponse
	if err = json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	return configs, nil
}

// DeleteOAuthIdpConfig deletes an OAuth IDP configuration of a project
func (s *IdentityPlatformService) DeleteOAuthIdpConfig(ctx context.Context, projectID, configID string) error {
	url := fmt.Sprintf("%sprojects/%s/oauthIdpConfigs/%s", s.BasePath, projectID, configID)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error deleting OAuth IDP config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	return nil
}
