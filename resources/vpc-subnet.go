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

const VPCSubnetResource = "VPCSubnet"

func init() {
	registry.Register(&registry.Registration{
		Name:   VPCSubnetResource,
		Scope:  nuke.Project,
		Lister: &VPCSubnetLister{},
	})
}

type VPCSubnetLister struct {
	svc *compute.SubnetworksClient
}

func (l *VPCSubnetLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *VPCSubnetLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)

	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	resources := make([]resource.Resource, 0)

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewSubnetworksRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListSubnetworksRequest{
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

		resources = append(resources, &VPCSubnet{
			svc:     l.svc,
			Name:    resp.Name,
			Project: opts.Project,
			Region:  opts.Region,
		})
	}

	return resources, nil
}

type VPCSubnet struct {
	svc     *compute.SubnetworksClient
	Project *string
	Name    *string
	Region  *string
}

func (r *VPCSubnet) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteSubnetworkRequest{
		Project:    *r.Project,
		Region:     *r.Region,
		Subnetwork: *r.Name,
	})
	return err
}

func (r *VPCSubnet) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}
