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

const ComputeBackendServiceResource = "ComputeBackendService"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeBackendServiceResource,
		Scope:    nuke.Project,
		Resource: &ComputeBackendService{},
		Lister:   &ComputeBackendServiceLister{},
	})
}

type ComputeBackendServiceLister struct {
	svc       *compute.RegionBackendServicesClient
	globalSvc *compute.BackendServicesClient
}

func (l *ComputeBackendServiceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
	if l.globalSvc != nil {
		_ = l.globalSvc.Close()
	}
}

func (l *ComputeBackendServiceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err == nil {
		globalResources, err := l.listGlobal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list global ssl certificates")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err == nil {
		regionalResources, err := l.listRegional(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional ssl certificates")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *ComputeBackendServiceLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.globalSvc == nil {
		var err error
		l.globalSvc, err = compute.NewBackendServicesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListBackendServicesRequest{
		Project: *opts.Project,
	}
	it := l.globalSvc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		resources = append(resources, &ComputeBackendService{
			globalSvc: l.globalSvc,
			project:   opts.Project,
			Name:      resp.Name,
		})
	}

	return resources, nil
}

func (l *ComputeBackendServiceLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRegionBackendServicesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRegionBackendServicesRequest{
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

		resources = append(resources, &ComputeBackendService{
			svc:     l.svc,
			project: opts.Project,
			region:  opts.Region,
			Name:    resp.Name,
		})
	}

	return resources, nil
}

type ComputeBackendService struct {
	svc       *compute.RegionBackendServicesClient
	globalSvc *compute.BackendServicesClient
	removeOp  *compute.Operation
	project   *string
	region    *string
	Name      *string
}

func (r *ComputeBackendService) Remove(ctx context.Context) error {
	if r.svc != nil {
		return r.removeRegion(ctx)
	} else if r.globalSvc != nil {
		return r.removeGlobal(ctx)
	}

	return errors.New("unable to determine service")
}

func (r *ComputeBackendService) removeGlobal(ctx context.Context) (err error) {
	r.removeOp, err = r.globalSvc.Delete(ctx, &computepb.DeleteBackendServiceRequest{
		Project:        *r.project,
		BackendService: *r.Name,
	})
	return err
}

func (r *ComputeBackendService) removeRegion(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteRegionBackendServiceRequest{
		Project:        *r.project,
		Region:         *r.region,
		BackendService: *r.Name,
	})
	return err
}

func (r *ComputeBackendService) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeBackendService) String() string {
	return *r.Name
}

func (r *ComputeBackendService) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeBackendServiceResource)
			return removeErr
		}
	}

	return nil
}
