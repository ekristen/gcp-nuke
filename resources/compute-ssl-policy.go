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

const ComputeSSLPolicyResource = "ComputeSSLPolicy"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeSSLPolicyResource,
		Scope:    nuke.Project,
		Resource: &ComputeSSLPolicy{},
		Lister:   &ComputeSSLPolicyLister{},
		DependsOn: []string{
			ComputeTargetHTTPSProxyResource,
			ComputeTargetSSLProxyResource,
		},
	})
}

type ComputeSSLPolicyLister struct {
	svc       *compute.RegionSslPoliciesClient
	globalSvc *compute.SslPoliciesClient
}

func (l *ComputeSSLPolicyLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
	if l.globalSvc != nil {
		_ = l.globalSvc.Close()
	}
}

func (l *ComputeSSLPolicyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com", ComputeSSLPolicyResource); err == nil {
		globalResources, err := l.listGlobal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list global")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com", ComputeSSLPolicyResource); err == nil {
		regionalResources, err := l.listRegional(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *ComputeSSLPolicyLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.globalSvc == nil {
		var err error
		l.globalSvc, err = compute.NewSslPoliciesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListSslPoliciesRequest{
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

		resources = append(resources, &ComputeSSLPolicy{
			globalSvc:     l.globalSvc,
			project:       opts.Project,
			Name:          resp.Name,
			Profile:       resp.Profile,
			MinTLSVersion: resp.MinTlsVersion,
			CreatedAt:     resp.CreationTimestamp,
		})
	}

	return resources, nil
}

func (l *ComputeSSLPolicyLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRegionSslPoliciesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRegionSslPoliciesRequest{
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

		resources = append(resources, &ComputeSSLPolicy{
			svc:           l.svc,
			project:       opts.Project,
			region:        opts.Region,
			Name:          resp.Name,
			Profile:       resp.Profile,
			MinTLSVersion: resp.MinTlsVersion,
			CreatedAt:     resp.CreationTimestamp,
		})
	}

	return resources, nil
}

type ComputeSSLPolicy struct {
	svc           *compute.RegionSslPoliciesClient
	globalSvc     *compute.SslPoliciesClient
	removeOp      *compute.Operation
	project       *string
	region        *string
	Name          *string
	Profile       *string
	MinTLSVersion *string
	CreatedAt     *string
}

func (r *ComputeSSLPolicy) Remove(ctx context.Context) error {
	if r.svc != nil {
		return r.removeRegional(ctx)
	} else if r.globalSvc != nil {
		return r.removeGlobal(ctx)
	}

	return errors.New("unable to determine service")
}

func (r *ComputeSSLPolicy) removeGlobal(ctx context.Context) (err error) {
	r.removeOp, err = r.globalSvc.Delete(ctx, &computepb.DeleteSslPolicyRequest{
		Project:   *r.project,
		SslPolicy: *r.Name,
	})
	return err
}

func (r *ComputeSSLPolicy) removeRegional(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteRegionSslPolicyRequest{
		Project:   *r.project,
		Region:    *r.region,
		SslPolicy: *r.Name,
	})
	return err
}

func (r *ComputeSSLPolicy) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeSSLPolicy) String() string {
	return *r.Name
}

func (r *ComputeSSLPolicy) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeSSLPolicyResource)
			return removeErr
		}
	}

	return nil
}
