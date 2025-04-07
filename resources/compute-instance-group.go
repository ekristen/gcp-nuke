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

const ComputeInstanceGroupResource = "ComputeInstanceGroup"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeInstanceGroupResource,
		Scope:    nuke.Project,
		Resource: &ComputeInstanceGroup{},
		Lister:   &ComputeInstanceGroupLister{},
	})
}

type ComputeInstanceGroupLister struct {
	svc *compute.InstanceGroupsClient
}

func (l *ComputeInstanceGroupLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewInstanceGroupsRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	for _, zone := range opts.Zones {
		req := &computepb.ListInstanceGroupsRequest{
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
				logrus.WithError(err).Error("unable to iterate compute instance groups")
				break
			}

			resources = append(resources, &ComputeInstanceGroup{
				svc:               l.svc,
				Name:              resp.Name,
				Project:           opts.Project,
				Zone:              ptr.String(zone),
				CreationTimestamp: resp.CreationTimestamp,
			})
		}
	}

	return resources, nil
}

type ComputeInstanceGroup struct {
	svc               *compute.InstanceGroupsClient
	Project           *string
	Region            *string
	Name              *string
	Zone              *string
	CreationTimestamp *string
}

func (r *ComputeInstanceGroup) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteInstanceGroupRequest{
		Project:       *r.Project,
		Zone:          *r.Zone,
		InstanceGroup: *r.Name,
	})
	return err
}

func (r *ComputeInstanceGroup) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeInstanceGroup) String() string {
	return *r.Name
}
