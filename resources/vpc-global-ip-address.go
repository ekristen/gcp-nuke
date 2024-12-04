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

const VPCGlobalIPAddressResource = "VPCGlobalIPAddress"

func init() {
	registry.Register(&registry.Registration{
		Name:     VPCGlobalIPAddressResource,
		Scope:    nuke.Project,
		Resource: &VPCGlobalIPAddress{},
		Lister:   &VPCGlobalIPAddressLister{},
	})
}

type VPCGlobalIPAddressLister struct {
	svc *compute.GlobalAddressesClient
}

func (l *VPCGlobalIPAddressLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewGlobalAddressesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListGlobalAddressesRequest{
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

		resources = append(resources, &VPCGlobalIPAddress{
			svc:         l.svc,
			project:     opts.Project,
			region:      opts.Region,
			Name:        resp.Name,
			Address:     resp.Address,
			AddressType: resp.AddressType,
		})
	}

	return resources, nil
}

type VPCGlobalIPAddress struct {
	svc         *compute.GlobalAddressesClient
	removeOp    *compute.Operation
	project     *string
	region      *string
	Name        *string
	Address     *string
	AddressType *string
}

func (r *VPCGlobalIPAddress) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteGlobalAddressRequest{
		Project: *r.project,
		Address: *r.Name, // misleading - address is actually the name not the IPv4/IPv6 value
	})
	return err
}

func (r *VPCGlobalIPAddress) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VPCGlobalIPAddress) String() string {
	return *r.Name
}

func (r *VPCGlobalIPAddress) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("network remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Error("unable to delete vpc global ip address")
			return removeErr
		}
	}

	return nil
}
