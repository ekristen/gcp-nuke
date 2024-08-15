package resources

import (
	"context"
	"errors"
	"fmt"
	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComputeTargetHTTPSProxyResource = "ComputeTargetHTTPSProxy"

func init() {
	registry.Register(&registry.Registration{
		Name:   ComputeTargetHTTPSProxyResource,
		Scope:  nuke.Project,
		Lister: &ComputeTargetHTTPSProxyLister{},
	})
}

type ComputeTargetHTTPSProxyLister struct {
	svc       *compute.RegionTargetHttpsProxiesClient
	globalSvc *compute.TargetHttpsProxiesClient
}

func (l *ComputeTargetHTTPSProxyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
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

func (l *ComputeTargetHTTPSProxyLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.globalSvc == nil {
		var err error
		l.globalSvc, err = compute.NewTargetHttpsProxiesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListTargetHttpsProxiesRequest{
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

		resources = append(resources, &ComputeTargetHTTPSProxy{
			globalSvc: l.globalSvc,
			project:   opts.Project,
			Name:      resp.Name,
		})
	}

	return resources, nil
}

func (l *ComputeTargetHTTPSProxyLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRegionTargetHttpsProxiesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRegionTargetHttpsProxiesRequest{
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

		certResource := &ComputeTargetHTTPSProxy{
			svc:     l.svc,
			project: opts.Project,
			region:  opts.Region,
			Name:    resp.Name,
		}

		resources = append(resources, certResource)
	}

	return resources, nil
}

type ComputeTargetHTTPSProxy struct {
	svc       *compute.RegionTargetHttpsProxiesClient
	globalSvc *compute.TargetHttpsProxiesClient
	removeOp  *compute.Operation
	project   *string
	region    *string
	Name      *string
	Type      *string
	Domain    *string
	ExpiresAt *string
}

func (r *ComputeTargetHTTPSProxy) Remove(ctx context.Context) error {
	if r.svc != nil {
		return r.removeRegional(ctx)
	} else if r.globalSvc != nil {
		return r.removeGlobal(ctx)
	}

	return errors.New("unable to determine service")
}

func (r *ComputeTargetHTTPSProxy) removeGlobal(ctx context.Context) (err error) {
	r.removeOp, err = r.globalSvc.Delete(ctx, &computepb.DeleteTargetHttpsProxyRequest{
		Project:          *r.project,
		TargetHttpsProxy: *r.Name,
	})
	return err
}

func (r *ComputeTargetHTTPSProxy) removeRegional(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteRegionTargetHttpsProxyRequest{
		Project:          *r.project,
		Region:           *r.region,
		TargetHttpsProxy: *r.Name,
	})
	return err
}

func (r *ComputeTargetHTTPSProxy) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeTargetHTTPSProxy) String() string {
	return *r.Name
}

func (r *ComputeTargetHTTPSProxy) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeTargetHTTPSProxyResource)
			return removeErr
		}
	}

	return nil
}
