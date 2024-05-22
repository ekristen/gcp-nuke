package resources

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const VPCRouterResource = "VPCRouter"

func init() {
	registry.Register(&registry.Registration{
		Name:   VPCRouterResource,
		Scope:  nuke.Project,
		Lister: &VPCRouterLister{},
	})
}

type VPCRouterLister struct {
	svc *compute.RoutersClient
}

func (l *VPCRouterLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRoutersRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRoutersRequest{
		Project: *opts.Project,
		Region:  *opts.Region,
	}
	it := l.svc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		resources = append(resources, &VPCRouter{
			svc:     l.svc,
			Project: opts.Project,
			Region:  opts.Region,
			Name:    resp.Name,
		})
	}

	return resources, nil
}

type VPCRouter struct {
	svc     *compute.RoutersClient
	Project *string
	Region  *string
	Name    *string
}

func (r *VPCRouter) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteRouterRequest{
		Project: *r.Project,
		Region:  *r.Region,
		Router:  *r.Name,
	})
	return err
}

func (r *VPCRouter) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VPCRouter) String() string {
	return *r.Name
}
