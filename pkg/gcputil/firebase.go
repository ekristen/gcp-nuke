package gcputil

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"

	"golang.org/x/oauth2/google"
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

// NewFirebaseDBClient creates a new Firebase Realtime Database client to interact with the
// https://firebasedatabase.googleapis.com endpoints as there is no official golang client library
func NewFirebaseDBClient(ctx context.Context) (*FirebaseDBClient, error) {
	client, err := google.DefaultClient(ctx,
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/cloud-platform.read-only",
		"https://www.googleapis.com/auth/firebase",
		"https://www.googleapis.com/auth/firebase.readonly",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT from credentials: %v", err)
	}

	return &FirebaseDBClient{
		httpClient: client,
	}, nil
}

// ListDatabaseRegions lists Firebase Realtime Database regions
func (c *FirebaseDBClient) ListDatabaseRegions() []string {
	return []string{
		"us-central1",
		"europe-west1",
		"asia-southeast1",
	}
}

// ListDatabaseInstances lists Firebase Realtime Database instances
func (c *FirebaseDBClient) ListDatabaseInstances(ctx context.Context, parent string) ([]*DatabaseInstance, error) {
	url1 := fmt.Sprintf("https://firebasedatabase.googleapis.com/v1beta/%s/instances", parent)
	logrus.Tracef("url: %s", url1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url1, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	resp, err := c.httpClient.Do(req)
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
func (c *FirebaseDBClient) DeleteDatabaseInstance(ctx context.Context, parent, name string) error {
	url := fmt.Sprintf("https://firebasedatabase.googleapis.com/v1beta/%s/instances/%s", parent, name)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := c.httpClient.Do(req)
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
func (c *FirebaseDBClient) DisableDatabaseInstance(ctx context.Context, parent, name string) error {
	url := fmt.Sprintf("https://firebasedatabase.googleapis.com/v1beta/%s/instances/%s:disable", parent, name)
	logrus.Tracef("url: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error disabling database instance: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request error: status %d", resp.StatusCode)
	}

	return nil
}
