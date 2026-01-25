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

const ComputeTargetGRPCProxyResource = "ComputeTargetGRPCProxy"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeTargetGRPCProxyResource,
		Scope:    nuke.Project,
		Resource: &ComputeTargetGRPCProxy{},
		Lister:   &ComputeTargetGRPCProxyLister{},
	})
}

type ComputeTargetGRPCProxyLister struct {
	svc *compute.TargetGrpcProxiesClient
}

func (l *ComputeTargetGRPCProxyLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ComputeTargetGRPCProxyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewTargetGrpcProxiesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListTargetGrpcProxiesRequest{
		Project: *opts.Project,
	}
	it := l.svc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate target grpc proxies")
			break
		}

		resources = append(resources, &ComputeTargetGRPCProxy{
			svc:       l.svc,
			project:   opts.Project,
			Name:      resp.Name,
			CreatedAt: resp.CreationTimestamp,
		})
	}

	return resources, nil
}

type ComputeTargetGRPCProxy struct {
	svc       *compute.TargetGrpcProxiesClient
	removeOp  *compute.Operation
	project   *string
	Name      *string
	CreatedAt *string
}

func (r *ComputeTargetGRPCProxy) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteTargetGrpcProxyRequest{
		Project:         *r.project,
		TargetGrpcProxy: *r.Name,
	})
	return err
}

func (r *ComputeTargetGRPCProxy) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeTargetGRPCProxy) String() string {
	return *r.Name
}

func (r *ComputeTargetGRPCProxy) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeTargetGRPCProxyResource)
			return removeErr
		}
	}

	return nil
}
