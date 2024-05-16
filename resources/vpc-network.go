package resources

import (
	"context"
	"errors"
	"fmt"
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

const VPCNetworkResource = "VPCNetwork"

func init() {
	registry.Register(&registry.Registration{
		Name:   VPCNetworkResource,
		Scope:  nuke.Project,
		Lister: &VPCNetworkLister{},
		DependsOn: []string{
			VPCSubnetResource,
			VPCRouteResource,
		},
	})
}

type VPCNetworkLister struct {
	svc *compute.NetworksClient
}

func (l *VPCNetworkLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *VPCNetworkLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	resources := make([]resource.Resource, 0)

	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewNetworksRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListNetworksRequest{
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

		resources = append(resources, &VPCNetwork{
			svc:     l.svc,
			Name:    resp.Name,
			Project: opts.Project,
		})
	}

	return resources, nil
}

type VPCNetwork struct {
	svc      *compute.NetworksClient
	removeOp *compute.Operation
	Project  *string
	Name     *string
}

func (r *VPCNetwork) Remove(ctx context.Context) error {
	var err error
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteNetworkRequest{
		Project: *r.Project,
		Network: *r.Name,
	})
	return err
}

// HandleWait is a hook into the libnuke resource lifecycle to allow for waiting on a resource to be removed
// because certain GCP resources are async and require waiting for the operation to complete, this allows for
// polling of the operation until it is complete. Otherwise, remove is only called once and the resource is
// left in a permanent wait if the operation fails.
func (r *VPCNetwork) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("network remove op polling encountered error")
		return err
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Error("unable to delete network")
			return removeErr
		}
	}

	return nil
}

func (r *VPCNetwork) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}
