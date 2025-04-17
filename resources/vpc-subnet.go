package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
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
		Name:     VPCSubnetResource,
		Scope:    nuke.Project,
		Resource: &VPCSubnet{},
		Lister:   &VPCSubnetLister{},
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
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewSubnetworksRESTClient(ctx, opts.ClientOptions...)
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
			logrus.WithError(err).Error("unable to iterate subnetworks")
			break
		}

		networkParts := strings.Split(resp.GetNetwork(), "/")
		networkName := networkParts[len(networkParts)-1]

		// TODO: query network to determine if auto-subnet
		resources = append(resources, &VPCSubnet{
			svc:       l.svc,
			Name:      resp.Name,
			project:   opts.Project,
			region:    opts.Region,
			Network:   ptr.String(networkName),
			IPV4Range: resp.IpCidrRange,
			IPV6Range: resp.Ipv6CidrRange,
		})
	}

	return resources, nil
}

type VPCSubnet struct {
	svc       *compute.SubnetworksClient
	removeOp  *compute.Operation
	project   *string
	region    *string
	Name      *string
	Network   *string
	IPV4Range *string
	IPV6Range *string
}

func (r *VPCSubnet) Filter() error {
	if *r.Name == "default" && strings.HasSuffix(*r.IPV4Range, "/20") {
		return fmt.Errorf("cannot remove default auto-subnet")
	}
	return nil
}

func (r *VPCSubnet) Remove(ctx context.Context) error {
	var err error
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteSubnetworkRequest{
		Project:    *r.project,
		Region:     *r.region,
		Subnetwork: *r.Name,
	})
	return err
}

func (r *VPCSubnet) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VPCSubnet) String() string {
	return *r.Name
}

// HandleWait is a hook into the libnuke resource lifecycle to allow for waiting on a resource to be removed
// because certain GCP resources are async and require waiting for the operation to complete, this allows for
// polling of the operation until it is complete. Otherwise, remove is only called once and the resource is
// left in a permanent wait if the operation fails.
func (r *VPCSubnet) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("subnet remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", VPCSubnetResource)
			return removeErr
		}
	}

	return nil
}
