package resources

import (
	"context"
	"errors"
	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComputeInstanceResource = "ComputeInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeInstanceResource,
		Scope:    nuke.Project,
		Resource: &ComputeInstance{},
		Lister:   &ComputeInstanceLister{},
	})
}

type ComputeInstanceLister struct {
	svc *compute.InstancesClient
}

func (l *ComputeInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewInstancesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	for _, zone := range opts.Zones {
		req := &computepb.ListInstancesRequest{
			Project: *opts.Project,
			Zone:    zone,
		}

		it := l.svc.List(ctx, req)

		for {
			resp, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate compute instances")
				break
			}

			resources = append(resources, &ComputeInstance{
				svc:               l.svc,
				Name:              resp.Name,
				Project:           opts.Project,
				Zone:              ptr.String(zone),
				CreationTimestamp: resp.CreationTimestamp,
				Labels:            resp.Labels,
			})
		}
	}

	return resources, nil
}

type ComputeInstance struct {
	svc               *compute.InstancesClient
	Project           *string
	Region            *string
	Name              *string
	Zone              *string
	CreationTimestamp *string
	Labels            map[string]string `property:"tagPrefix=label"`
}

func (r *ComputeInstance) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteInstanceRequest{
		Project:  *r.Project,
		Zone:     *r.Zone,
		Instance: *r.Name,
	})
	return err
}

func (r *ComputeInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeInstance) String() string {
	return *r.Name
}
