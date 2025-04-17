package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const GKEClusterResource = "GKECluster"

func init() {
	registry.Register(&registry.Registration{
		Name:     GKEClusterResource,
		Scope:    nuke.Project,
		Resource: &GKECluster{},
		Lister:   &GKEClusterLister{},
	})
}

type GKEClusterLister struct {
	svc *container.ClusterManagerClient
}

func (l *GKEClusterLister) ListClusters(ctx context.Context, project, location string) ([]resource.Resource, error) {
	var resources []resource.Resource

	req := &containerpb.ListClustersRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", project, location),
	}

	resp, err := l.svc.ListClusters(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list GKE clusters: %v", err)
	}

	for _, cluster := range resp.Clusters {
		region := location
		zone := ""
		if len(strings.Split(location, "-")) > 2 {
			region = strings.Join(strings.Split(location, "-")[:2], "-")
			zone = location
		}

		resources = append(resources, &GKECluster{
			svc:     l.svc,
			Project: ptr.String(project),
			Region:  ptr.String(region),
			Name:    ptr.String(cluster.Name),
			Zone:    ptr.String(zone),
			Status:  ptr.String(cluster.Status.String()),
		})
	}

	return resources, nil
}

func (l *GKEClusterLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)

	logrus.WithFields(logrus.Fields{
		"region":        *opts.Region,
		"resource_type": "GKECluster",
	}).Debug("GKE Lister - Region value")

	if err := opts.BeforeList(nuke.Regional, "container.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = container.NewClusterManagerClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	locations := []string{*opts.Region}
	locations = append(locations, opts.Zones...)

	for _, loc := range locations {
		clusters, err := l.ListClusters(ctx, *opts.Project, loc)
		if err != nil {
			return nil, err
		}
		resources = append(resources, clusters...)
	}

	return resources, nil
}

type GKECluster struct {
	svc      *container.ClusterManagerClient
	removeOp *containerpb.Operation
	Project  *string
	Region   *string
	Name     *string
	Zone     *string
	Status   *string
}

func (r *GKECluster) Remove(ctx context.Context) error {
	var err error
	location := r.Region
	if *r.Zone != "" {
		location = r.Zone
	}

	r.removeOp, err = r.svc.DeleteCluster(ctx, &containerpb.DeleteClusterRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/clusters/%s", *r.Project, *location, *r.Name),
	})
	return err
}

func (r *GKECluster) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *GKECluster) String() string {
	return *r.Name
}

func (r *GKECluster) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	// TODO: this does not properly handle the wait it somehow ends in an error.
	var err error
	r.removeOp, err = r.svc.GetOperation(ctx, &containerpb.GetOperationRequest{
		Name: r.removeOp.Name,
	})
	if err != nil {
		return err
	}

	if r.removeOp.Status != containerpb.Operation_DONE {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.GetError() != nil {
		return fmt.Errorf("operation failed: %v", r.removeOp.GetError())
	}

	return nil
}
