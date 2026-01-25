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

const ComputeExternalVpnGatewayResource = "ComputeExternalVpnGateway"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeExternalVpnGatewayResource,
		Scope:    nuke.Project,
		Resource: &ComputeExternalVpnGateway{},
		Lister:   &ComputeExternalVpnGatewayLister{},
		DependsOn: []string{
			ComputeVpnTunnelResource,
		},
	})
}

type ComputeExternalVpnGatewayLister struct {
	svc *compute.ExternalVpnGatewaysClient
}

func (l *ComputeExternalVpnGatewayLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ComputeExternalVpnGatewayLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewExternalVpnGatewaysRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListExternalVpnGatewaysRequest{
		Project: *opts.Project,
	}
	it := l.svc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate external vpn gateways")
			break
		}

		resources = append(resources, &ComputeExternalVpnGateway{
			svc:            l.svc,
			project:        opts.Project,
			Name:           resp.Name,
			RedundancyType: resp.RedundancyType,
			Labels:         resp.Labels,
		})
	}

	return resources, nil
}

type ComputeExternalVpnGateway struct {
	svc            *compute.ExternalVpnGatewaysClient
	removeOp       *compute.Operation
	project        *string
	Name           *string
	RedundancyType *string
	Labels         map[string]string `property:"tagPrefix=label"`
}

func (r *ComputeExternalVpnGateway) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteExternalVpnGatewayRequest{
		Project:            *r.project,
		ExternalVpnGateway: *r.Name,
	})
	return err
}

func (r *ComputeExternalVpnGateway) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeExternalVpnGateway) String() string {
	return *r.Name
}

func (r *ComputeExternalVpnGateway) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeExternalVpnGatewayResource)
			return removeErr
		}
	}

	return nil
}
