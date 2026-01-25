package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

func (l *GKEClusterLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
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
			svc:               l.svc,
			Project:           ptr.String(project),
			Region:            ptr.String(region),
			Name:              ptr.String(cluster.Name),
			Zone:              ptr.String(zone),
			Status:            ptr.String(cluster.Status.String()),
			CreationTimestamp: ptr.String(cluster.CreateTime),
			Labels:            cluster.ResourceLabels,
		})
	}

	return resources, nil
}

func (l *GKEClusterLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
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
	svc               *container.ClusterManagerClient
	removeOp          *containerpb.Operation
	Project           *string
	Region            *string
	Name              *string
	Zone              *string
	Status            *string
	CreationTimestamp *string
	Labels            map[string]string `property:"tagPrefix=label"`
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
	if err != nil {
		logrus.WithError(err).WithField("cluster", *r.Name).Trace("gke cluster delete error")
		return liberror.ErrWaitResource(fmt.Sprintf("delete failed: %v", err))
	}
	return nil
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

	var err error
	r.removeOp, err = r.svc.GetOperation(ctx, &containerpb.GetOperationRequest{
		Name: r.removeOp.Name,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			logrus.WithField("cluster", *r.Name).Trace("operation not found, assuming completed")
			return nil
		}
		logrus.WithError(err).WithField("cluster", *r.Name).Trace("failed to get operation status")
		return liberror.ErrWaitResource(fmt.Sprintf("poll failed: %v", err))
	}

	if r.removeOp.Status != containerpb.Operation_DONE {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.GetError() != nil {
		return fmt.Errorf("operation failed: %v", r.removeOp.GetError())
	}

	return nil
}
