package gcputil

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"

	"google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/api/serviceusage/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

type Organization struct {
	Name        string
	DisplayName string
}

func (o *Organization) ID() string {
	return strings.Split(o.Name, "organizations/")[1]
}

type Project struct {
	Name      string
	ProjectID string
}

func (p *Project) ID() string {
	return strings.Split(p.Name, "projects/")[1]
}

type GCP struct {
	Organizations []*Organization
	Projects      []*Project
	Regions       []string
	APIS          []string

	ProjectID string

	zones map[string][]string

	tokenSource   oauth2.TokenSource
	clientOptions []option.ClientOption
}

func (g *GCP) HasOrganizations() bool {
	if g.Organizations == nil {
		return false
	}
	return len(g.Organizations) > 0
}

func (g *GCP) HasProjects() bool {
	if g.Projects == nil {
		return false
	}
	return len(g.Projects) > 0
}

func (g *GCP) GetZones(region string) []string {
	return g.zones[region]
}

func (g *GCP) ImpersonateServiceAccount(ctx context.Context, targetServiceAccount string) error {
	credsClient, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return err
	}
	defer credsClient.Close()

	req := &credentialspb.GenerateAccessTokenRequest{
		Name:  fmt.Sprintf("projects/-/serviceAccounts/%s", targetServiceAccount),
		Scope: []string{"https://www.googleapis.com/auth/cloud-platform"},
		Lifetime: &durationpb.Duration{
			Seconds: int64(time.Hour.Seconds()), // 1 hour
		},
	}
	resp, err := credsClient.GenerateAccessToken(ctx, req)
	if err != nil {
		return err
	}

	// Create a new authenticated client using the impersonated access token
	g.tokenSource = oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: resp.AccessToken,
	})

	g.clientOptions = append(g.clientOptions, option.WithTokenSource(g.tokenSource))

	return nil
}

func (g *GCP) GetClientOptions() []option.ClientOption {
	return g.clientOptions
}

func (g *GCP) ID() string {
	return g.ProjectID
}

func (g *GCP) GetEnabledAPIs() []string {
	return g.APIS
}

func (g *GCP) GetCredentials(ctx context.Context) (*google.Credentials, error) {
	return google.FindDefaultCredentials(ctx)
}

func New(ctx context.Context, projectID, impersonateServiceAccount string) (*GCP, error) {
	gcp := &GCP{
		Organizations: make([]*Organization, 0),
		Projects:      make([]*Project, 0),
		Regions:       []string{"global"},
		ProjectID:     projectID,
		zones:         make(map[string][]string),
		clientOptions: make([]option.ClientOption, 0),
	}

	if impersonateServiceAccount != "" {
		if err := gcp.ImpersonateServiceAccount(ctx, impersonateServiceAccount); err != nil {
			return nil, err
		}
	}

	service, err := cloudresourcemanager.NewService(ctx, gcp.GetClientOptions()...)
	if err != nil {
		return nil, err
	}

	req := service.Organizations.Search()
	if resp, err := req.Do(); err != nil {
		return nil, err
	} else {
		for _, org := range resp.Organizations {
			newOrg := &Organization{
				Name:        org.Name,
				DisplayName: org.DisplayName,
			}

			gcp.Organizations = append(gcp.Organizations, newOrg)

			logrus.WithFields(logrus.Fields{
				"name":        newOrg.Name,
				"displayName": newOrg.DisplayName,
				"id":          newOrg.ID(),
			}).Trace("organization found")
		}
	}

	// Request to list projects
	preq := service.Projects.Search()
	if err := preq.Pages(ctx, func(page *cloudresourcemanager.SearchProjectsResponse) error {
		for _, project := range page.Projects {
			newProject := &Project{
				Name:      project.Name,
				ProjectID: project.ProjectId,
			}
			gcp.Projects = append(gcp.Projects, newProject)

			logrus.WithFields(logrus.Fields{
				"name":       newProject.Name,
				"project.id": newProject.ProjectID,
				"id":         newProject.ID(),
			}).Trace("project found")
		}
		return nil
	}); err != nil {
		return nil, err
	}

	c, err := compute.NewRegionsRESTClient(ctx, gcp.GetClientOptions()...)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Build the request to list regions
	regionReq := &computepb.ListRegionsRequest{
		Project: projectID,
	}

	it := c.List(ctx, regionReq)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		gcp.Regions = append(gcp.Regions, resp.GetName())

		if gcp.zones[resp.GetName()] == nil {
			gcp.zones[resp.GetName()] = make([]string, 0)
		}

		for _, z := range resp.GetZones() {
			zoneShort := strings.Split(z, "/")[len(strings.Split(z, "/"))-1]
			gcp.zones[resp.GetName()] = append(gcp.zones[resp.GetName()], zoneShort)
		}
	}

	serviceUsage, err := serviceusage.NewService(ctx, gcp.GetClientOptions()...)
	if err != nil {
		return nil, err
	}

	suReq := serviceUsage.Services.
		List(fmt.Sprintf("projects/%s", projectID)).
		Filter("state:ENABLED")

	if suErr := suReq.Pages(ctx, func(page *serviceusage.ListServicesResponse) error {
		for _, svc := range page.Services {
			gcp.APIS = append(gcp.APIS, svc.Config.Name)
		}
		return nil
	}); suErr != nil {
		return nil, suErr
	}

	return gcp, nil
}
