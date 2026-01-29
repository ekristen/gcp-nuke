package resources

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComputeHealthCheckResource = "ComputeHealthCheck"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeHealthCheckResource,
		Scope:    nuke.Project,
		Resource: &ComputeHealthCheck{},
		Lister:   &ComputeHealthCheckLister{},
	})
}

type ComputeHealthCheckLister struct {
	svc *compute.HealthChecksClient
}

func (l *ComputeHealthCheckLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ComputeHealthCheckLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com", ComputeHealthCheckResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewHealthChecksRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListHealthChecksRequest{
		Project: *opts.Project,
	}

	it := l.svc.List(ctx, req)

	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate compute health checks")
			break
		}

		resources = append(resources, &ComputeHealthCheck{
			svc:               l.svc,
			Name:              resp.Name,
			Project:           opts.Project,
			CreationTimestamp: resp.CreationTimestamp,
			Type:              resp.Type,
		})
	}

	return resources, nil
}

type ComputeHealthCheck struct {
	svc               *compute.HealthChecksClient
	Project           *string
	Name              *string
	CreationTimestamp *string
	Type              *string
}

func (r *ComputeHealthCheck) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteHealthCheckRequest{
		Project:     *r.Project,
		HealthCheck: *r.Name,
	})
	return err
}

func (r *ComputeHealthCheck) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeHealthCheck) String() string {
	return *r.Name
}
