package gcputil

import (
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/iterator"
	"log"
	"strings"
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

	zones map[string][]string
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

func New(ctx context.Context, projectID string) (*GCP, error) {
	gcp := &GCP{
		Organizations: make([]*Organization, 0),
		Projects:      make([]*Project, 0),
		Regions:       []string{"global"},
		zones:         make(map[string][]string),
	}

	service, err := cloudresourcemanager.NewService(ctx)
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

	c, err := compute.NewRegionsRESTClient(ctx)
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

	return gcp, nil
}
