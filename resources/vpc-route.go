package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"
	"strings"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const VPCRouteResource = "VPCRoute"

func init() {
	registry.Register(&registry.Registration{
		Name:     VPCRouteResource,
		Scope:    nuke.Project,
		Resource: &VPCRoute{},
		Lister:   &VPCRouteLister{},
	})
}

type VPCRouteLister struct {
	svc *compute.RoutesClient
}

func (l *VPCRouteLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRoutesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRoutesRequest{
		Project: *opts.Project,
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

		resources = append(resources, &VPCRoute{
			svc:         l.svc,
			Project:     opts.Project,
			Network:     ptr.String(strings.Split(*resp.Network, "/")[len(strings.Split(*resp.Network, "/"))-1]),
			Name:        resp.Name,
			RouteType:   resp.RouteType,
			Description: resp.Description,
		})
	}

	return resources, nil
}

type VPCRoute struct {
	svc         *compute.RoutesClient
	Project     *string
	Region      *string
	Name        *string
	Network     *string
	RouteType   *string
	Description *string `property:"-"`
}

func (r *VPCRoute) Filter() error {
	if strings.HasPrefix(*r.Description, "Default local route to the subnetwork") {
		return fmt.Errorf("unable to remove default local route")
	}
	return nil
}

func (r *VPCRoute) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteRouteRequest{
		Project: *r.Project,
		Route:   *r.Name,
	})
	return err
}

func (r *VPCRoute) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VPCRoute) String() string {
	return *r.Name
}
