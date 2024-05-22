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

const ComputeFirewallResource = "ComputeFirewall"

func init() {
	registry.Register(&registry.Registration{
		Name:   ComputeFirewallResource,
		Scope:  nuke.Project,
		Lister: &ComputeFirewallLister{},
	})
}

type ComputeFirewallLister struct {
	svc *compute.FirewallsClient
}

func (l *ComputeFirewallLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewFirewallsRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListFirewallsRequest{
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

		resources = append(resources, &ComputeFirewall{
			svc:     l.svc,
			Name:    resp.Name,
			Project: opts.Project,
		})
	}

	return resources, nil
}

type ComputeFirewall struct {
	svc     *compute.FirewallsClient
	Project *string
	Region  *string
	Name    *string
}

func (r *ComputeFirewall) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteFirewallRequest{
		Project:  *r.Project,
		Firewall: *r.Name,
	})
	return err
}

func (r *ComputeFirewall) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeFirewall) String() string {
	return *r.Name
}
