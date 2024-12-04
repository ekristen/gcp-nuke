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

const ComputeTargetHTTPProxyResource = "ComputeTargetHTTPProxy"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeTargetHTTPProxyResource,
		Scope:    nuke.Project,
		Resource: &ComputeTargetHTTPProxy{},
		Lister:   &ComputeTargetHTTPProxyLister{},
	})
}

type ComputeTargetHTTPProxyLister struct {
	svc       *compute.RegionTargetHttpProxiesClient
	globalSvc *compute.TargetHttpProxiesClient
}

func (l *ComputeTargetHTTPProxyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err == nil {
		globalResources, err := l.listGlobal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list global target proxies")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err == nil {
		regionalResources, err := l.listRegional(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional target proxies")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *ComputeTargetHTTPProxyLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.globalSvc == nil {
		var err error
		l.globalSvc, err = compute.NewTargetHttpProxiesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListTargetHttpProxiesRequest{
		Project: *opts.Project,
	}
	it := l.globalSvc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate")
			break
		}

		resources = append(resources, &ComputeTargetHTTPProxy{
			globalSvc: l.globalSvc,
			project:   opts.Project,
			Name:      resp.Name,
			CreatedAt: resp.CreationTimestamp,
		})
	}

	return resources, nil
}

func (l *ComputeTargetHTTPProxyLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRegionTargetHttpProxiesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRegionTargetHttpProxiesRequest{
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
			logrus.WithError(err).Error("unable to iterate")
			break
		}

		certResource := &ComputeTargetHTTPProxy{
			svc:       l.svc,
			project:   opts.Project,
			region:    opts.Region,
			Name:      resp.Name,
			CreatedAt: resp.CreationTimestamp,
		}

		resources = append(resources, certResource)
	}

	return resources, nil
}

type ComputeTargetHTTPProxy struct {
	svc       *compute.RegionTargetHttpProxiesClient
	globalSvc *compute.TargetHttpProxiesClient
	removeOp  *compute.Operation
	project   *string
	region    *string
	Name      *string
	CreatedAt *string
}

func (r *ComputeTargetHTTPProxy) Remove(ctx context.Context) error {
	if r.svc != nil {
		return r.removeRegional(ctx)
	} else if r.globalSvc != nil {
		return r.removeGlobal(ctx)
	}

	return errors.New("unable to determine service")
}

func (r *ComputeTargetHTTPProxy) removeGlobal(ctx context.Context) (err error) {
	r.removeOp, err = r.globalSvc.Delete(ctx, &computepb.DeleteTargetHttpProxyRequest{
		Project:         *r.project,
		TargetHttpProxy: *r.Name,
	})
	return err
}

func (r *ComputeTargetHTTPProxy) removeRegional(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteRegionTargetHttpProxyRequest{
		Project:         *r.project,
		Region:          *r.region,
		TargetHttpProxy: *r.Name,
	})
	return err
}

func (r *ComputeTargetHTTPProxy) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeTargetHTTPProxy) String() string {
	return *r.Name
}

func (r *ComputeTargetHTTPProxy) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeTargetHTTPProxyResource)
			return removeErr
		}
	}

	return nil
}
