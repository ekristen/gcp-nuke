package gcputil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/api/option/internaloption"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	htransport "google.golang.org/api/transport/http"
)

// FirebaseDBClient is a client to interact with the Firebase Realtime Database API
type FirebaseDBClient struct {
	httpClient *http.Client
}

// DatabaseInstance represents a Firebase Realtime Database instance
type DatabaseInstance struct {
	Name        string `json:"name"`
	Project     string `json:"project"`
	DatabaseURL string `json:"databaseUrl"`
	Type        string `json:"type"`
	State       string `json:"state"`
}

type loggingTransport struct {
	transport http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Log the request
	logRequest(req)

	// Perform the request
	res, err := t.transport.RoundTrip(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, err
	}

	// Log the response
	logResponse(res, time.Since(start))

	return res, nil
}

func logRequest(req *http.Request) {
	fmt.Printf("Request: %s %s\n", req.Method, req.URL)
	fmt.Printf("Headers: %v\n", req.Header)

	if req.Body != nil {
		bodyBytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			fmt.Printf("Error reading request body: %v\n", err)
		} else {
			req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes)) // Reassign the body
			fmt.Printf("Body: %s\n", string(bodyBytes))
		}
	}
}

func logResponse(res *http.Response, duration time.Duration) {
	fmt.Printf("Response: %s in %v\n", res.Status, duration)
	fmt.Printf("Headers: %v\n", res.Header)

	if res.Body != nil {
		bodyBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Printf("Error reading response body: %v\n", err)
		} else {
			res.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes)) // Reassign the body
			fmt.Printf("Body: %s\n", string(bodyBytes))
		}
	}
}

const basePath = "https://firebasedatabase.googleapis.com/"
const basePathTemplate = "https://firebasedatabase.UNIVERSE_DOMAIN/"
const mtlsBasePath = "https://firebasedatabase.mtls.googleapis.com/"

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
	opts = append(opts, internaloption.WithDefaultEndpoint(basePath))
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
