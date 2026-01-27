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

const ComputeTargetPoolResource = "ComputeTargetPool"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeTargetPoolResource,
		Scope:    nuke.Project,
		Resource: &ComputeTargetPool{},
		Lister:   &ComputeTargetPoolLister{},
	})
}

type ComputeTargetPoolLister struct {
	svc *compute.TargetPoolsClient
}

func (l *ComputeTargetPoolLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ComputeTargetPoolLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com", ComputeTargetPoolResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewTargetPoolsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListTargetPoolsRequest{
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
			logrus.WithError(err).Error("unable to iterate target pools")
			break
		}

		resources = append(resources, &ComputeTargetPool{
			svc:       l.svc,
			project:   opts.Project,
			region:    opts.Region,
			Name:      resp.Name,
			CreatedAt: resp.CreationTimestamp,
		})
	}

	return resources, nil
}

type ComputeTargetPool struct {
	svc       *compute.TargetPoolsClient
	removeOp  *compute.Operation
	project   *string
	region    *string
	Name      *string
	CreatedAt *string
}

func (r *ComputeTargetPool) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteTargetPoolRequest{
		Project:    *r.project,
		Region:     *r.region,
		TargetPool: *r.Name,
	})
	return err
}

func (r *ComputeTargetPool) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeTargetPool) String() string {
	return *r.Name
}

func (r *ComputeTargetPool) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeTargetPoolResource)
			return removeErr
		}
	}

	return nil
}
