package gcputil

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/api/option/internaloption"
	htransport "google.golang.org/api/transport/http"
	"io"
	"net/http"
)

const basePath = "https://firebasedatabase.googleapis.com/"
const basePathTemplate = "https://firebasedatabase.UNIVERSE_DOMAIN/"
const mtlsBasePath = "https://firebasedatabase.mtls.googleapis.com/"

// DatabaseInstance represents a Firebase Realtime Database instance
type DatabaseInstance struct {
	Name        string `json:"name"`
	Project     string `json:"project"`
	DatabaseURL string `json:"databaseUrl"`
	Type        string `json:"type"`
	State       string `json:"state"`
}

// NewFirebaseDatabaseService creates a new Firebase Realtime Database client to interact with the
// https://firebasedatabase.googleapis.com endpoints as there is no official golang client library
func NewFirebaseDatabaseService(ctx context.Context, opts ...option.ClientOption) (*FirebaseDatabaseService, error) {
	scopesOption := internaloption.WithDefaultScopes(
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/cloud-platform.read-only",
		"https://www.googleapis.com/auth/firebase",
		"https://www.googleapis.com/auth/firebase.readonly",
	)
	// NOTE: prepend, so we don't override user-specified scopes.
	opts = append([]option.ClientOption{scopesOption}, opts...)
	opts = append(opts, internaloption.WithDefaultEndpointTemplate(basePathTemplate))
	opts = append(opts, internaloption.WithDefaultMTLSEndpoint(mtlsBasePath))
	opts = append(opts, internaloption.EnableNewAuthLibrary())
	client, endpoint, err := htransport.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	s := &FirebaseDatabaseService{client: client, BasePath: basePath}
	if endpoint != "" {
		s.BasePath = endpoint
	}

	return s, nil
}

type FirebaseDatabaseService struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment
}

// ListDatabaseRegions lists Firebase Realtime Database regions
func (s *FirebaseDatabaseService) ListDatabaseRegions() []string {
	return []string{
		"us-central1",
		"europe-west1",
		"asia-southeast1",
	}
}

// ListDatabaseInstances lists Firebase Realtime Database instances
func (s *FirebaseDatabaseService) ListDatabaseInstances(ctx context.Context, parent string) ([]*DatabaseInstance, error) {
	url1 := fmt.Sprintf("%sv1beta/%s/instances", s.BasePath, parent)
	logrus.Tracef("url: %s", url1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url1, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting database instances: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d, _ := io.ReadAll(resp.Body)
		fmt.Println(string(d))
		return nil, fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	var instances struct {
		Instances []*DatabaseInstance `json:"instances"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}
	return instances.Instances, nil
}

// DeleteDatabaseInstance deletes a Firebase Realtime Database instance
func (s *FirebaseDatabaseService) DeleteDatabaseInstance(ctx context.Context, parent, name string) error {
	url := fmt.Sprintf("%sv1beta/%s/instances/%s", s.BasePath, parent, name)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error deleting database instance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	return nil
}

// DisableDatabaseInstance disables a Firebase Realtime Database instance
func (s *FirebaseDatabaseService) DisableDatabaseInstance(ctx context.Context, parent, name string) error {
	url := fmt.Sprintf("%sv1beta/%s/instances/%s:disable", s.BasePath, parent, name)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error disabling database instance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	return nil
}
