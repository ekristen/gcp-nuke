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

const ComputeBackendBucketResource = "ComputeBackendBucket"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeBackendBucketResource,
		Scope:    nuke.Project,
		Resource: &ComputeBackendBucket{},
		Lister:   &ComputeBackendBucketLister{},
	})
}

type ComputeBackendBucketLister struct {
	svc *compute.BackendBucketsClient
}

func (l *ComputeBackendBucketLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ComputeBackendBucketLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com", ComputeBackendBucketResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewBackendBucketsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListBackendBucketsRequest{
		Project: *opts.Project,
	}
	it := l.svc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate backend buckets")
			break
		}

		resources = append(resources, &ComputeBackendBucket{
			svc:        l.svc,
			project:    opts.Project,
			Name:       resp.Name,
			BucketName: resp.BucketName,
			CreatedAt:  resp.CreationTimestamp,
		})
	}

	return resources, nil
}

type ComputeBackendBucket struct {
	svc        *compute.BackendBucketsClient
	removeOp   *compute.Operation
	project    *string
	Name       *string
	BucketName *string
	CreatedAt  *string
}

func (r *ComputeBackendBucket) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteBackendBucketRequest{
		Project:       *r.project,
		BackendBucket: *r.Name,
	})
	return err
}

func (r *ComputeBackendBucket) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeBackendBucket) String() string {
	return *r.Name
}

func (r *ComputeBackendBucket) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeBackendBucketResource)
			return removeErr
		}
	}

	return nil
}
