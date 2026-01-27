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

const ComputeVpnTunnelResource = "ComputeVpnTunnel"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeVpnTunnelResource,
		Scope:    nuke.Project,
		Resource: &ComputeVpnTunnel{},
		Lister:   &ComputeVpnTunnelLister{},
	})
}

type ComputeVpnTunnelLister struct {
	svc *compute.VpnTunnelsClient
}

func (l *ComputeVpnTunnelLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ComputeVpnTunnelLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com", ComputeVpnTunnelResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewVpnTunnelsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListVpnTunnelsRequest{
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
			logrus.WithError(err).Error("unable to iterate vpn tunnels")
			break
		}

		resources = append(resources, &ComputeVpnTunnel{
			svc:        l.svc,
			project:    opts.Project,
			region:     opts.Region,
			Name:       resp.Name,
			VpnGateway: resp.VpnGateway,
			PeerIp:     resp.PeerIp,
			Status:     resp.Status,
			Labels:     resp.Labels,
		})
	}

	return resources, nil
}

type ComputeVpnTunnel struct {
	svc        *compute.VpnTunnelsClient
	removeOp   *compute.Operation
	project    *string
	region     *string
	Name       *string
	VpnGateway *string
	PeerIp     *string
	Status     *string
	Labels     map[string]string `property:"tagPrefix=label"`
}

func (r *ComputeVpnTunnel) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteVpnTunnelRequest{
		Project:   *r.project,
		Region:    *r.region,
		VpnTunnel: *r.Name,
	})
	return err
}

func (r *ComputeVpnTunnel) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeVpnTunnel) String() string {
	return *r.Name
}

func (r *ComputeVpnTunnel) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeVpnTunnelResource)
			return removeErr
		}
	}

	return nil
}
