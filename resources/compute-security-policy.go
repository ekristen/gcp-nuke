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

const ComputeSecurityPolicyResource = "ComputeSecurityPolicy"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeSecurityPolicyResource,
		Scope:    nuke.Project,
		Resource: &ComputeSecurityPolicy{},
		Lister:   &ComputeSecurityPolicyLister{},
	})
}

type ComputeSecurityPolicyLister struct {
	svc       *compute.RegionSecurityPoliciesClient
	globalSvc *compute.SecurityPoliciesClient
}

func (l *ComputeSecurityPolicyLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
	if l.globalSvc != nil {
		_ = l.globalSvc.Close()
	}
}

func (l *ComputeSecurityPolicyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com", ComputeSecurityPolicyResource); err == nil {
		globalResources, err := l.listGlobal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list global security policies")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com", ComputeSecurityPolicyResource); err == nil {
		regionalResources, err := l.listRegional(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional security policies")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *ComputeSecurityPolicyLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource
	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com", ComputeSecurityPolicyResource); err != nil {
		return resources, err
	}

	logrus.Debug("listing security policies")

	if l.globalSvc == nil {
		var err error
		l.globalSvc, err = compute.NewSecurityPoliciesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListSecurityPoliciesRequest{
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

		resources = append(resources, &ComputeSecurityPolicy{
			globalSvc: l.globalSvc,
			Name:      resp.Name,
			project:   opts.Project,
		})
	}

	return resources, nil
}

func (l *ComputeSecurityPolicyLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	logrus.Debug("listing security policies")

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRegionSecurityPoliciesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRegionSecurityPoliciesRequest{
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

		resources = append(resources, &ComputeSecurityPolicy{
			svc:       l.svc,
			Name:      resp.Name,
			project:   opts.Project,
			region:    opts.Region,
			CreatedAt: resp.CreationTimestamp,
			Labels:    resp.Labels,
		})
	}

	return resources, nil
}

type ComputeSecurityPolicy struct {
	svc       *compute.RegionSecurityPoliciesClient
	globalSvc *compute.SecurityPoliciesClient
	removeOp  *compute.Operation
	project   *string
	region    *string
	Name      *string
	CreatedAt *string
	Labels    map[string]string `property:"tagPrefix=label"`
}

func (r *ComputeSecurityPolicy) Remove(ctx context.Context) error {
	if r.svc != nil {
		return r.removeRegional(ctx)
	} else if r.globalSvc != nil {
		return r.removeGlobal(ctx)
	}

	return errors.New("unable to determine service")
}

func (r *ComputeSecurityPolicy) removeGlobal(ctx context.Context) (err error) {
	r.removeOp, err = r.globalSvc.Delete(ctx, &computepb.DeleteSecurityPolicyRequest{
		Project:        *r.project,
		SecurityPolicy: *r.Name,
	})
	return err
}

func (r *ComputeSecurityPolicy) removeRegional(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteRegionSecurityPolicyRequest{
		Project:        *r.project,
		Region:         *r.region,
		SecurityPolicy: *r.Name,
	})
	return err
}

func (r *ComputeSecurityPolicy) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeSecurityPolicy) String() string {
	return *r.Name
}

func (r *ComputeSecurityPolicy) HandleWait(ctx context.Context) error {
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
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Error("unable to delete network")
			return removeErr
		}
	}

	return nil
}
