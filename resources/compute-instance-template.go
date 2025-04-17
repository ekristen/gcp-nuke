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

const ComputeInstanceTemplateResource = "ComputeInstanceTemplate"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeInstanceTemplateResource,
		Scope:    nuke.Project,
		Resource: &ComputeInstanceTemplate{},
		Lister:   &ComputeInstanceTemplateLister{},
		DependsOn: []string{
			ComputeInstanceGroupResource,
			ComputeRegionalInstanceGroupResource,
		},
	})
}

type ComputeInstanceTemplateLister struct {
	svc *compute.InstanceTemplatesClient
}

func (l *ComputeInstanceTemplateLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	// For global resources, we need to ensure the region is set to "global"
	originalRegion := opts.Region
	globalRegion := "global"
	opts.Region = &globalRegion

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err == nil {
		globalResources, err := l.listGlobal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list global instance templates")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	// Restore the original region
	opts.Region = originalRegion

	return resources, nil
}

func (l *ComputeInstanceTemplateLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	logrus.Debug("listing instance templates")

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewInstanceTemplatesRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			logrus.WithError(err).Error("failed to create instance templates client")
			return nil, err
		}
	}

	req := &computepb.ListInstanceTemplatesRequest{
		Project: *opts.Project,
	}

	it := l.svc.List(ctx, req)

	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate compute instance templates")
			break
		}

		logrus.WithField("name", resp.Name).Debug("found instance template")

		resources = append(resources, &ComputeInstanceTemplate{
			svc:               l.svc,
			Name:              resp.Name,
			Project:           opts.Project,
			CreationTimestamp: resp.CreationTimestamp,
			Labels:            resp.Properties.Labels,
		})
	}

	return resources, nil
}

type ComputeInstanceTemplate struct {
	svc               *compute.InstanceTemplatesClient
	removeOp          *compute.Operation
	Project           *string
	Name              *string
	CreationTimestamp *string
	Labels            map[string]string `property:"tagPrefix=label"`
}

func (r *ComputeInstanceTemplate) Remove(ctx context.Context) error {
	var err error
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteInstanceTemplateRequest{
		Project:          *r.Project,
		InstanceTemplate: *r.Name,
	})
	return err
}

// HandleWait implements asynchronous operation tracking for instance template deletion
func (r *ComputeInstanceTemplate) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("instance template remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeInstanceTemplateResource)
			return removeErr
		}
	}

	return nil
}

func (r *ComputeInstanceTemplate) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeInstanceTemplate) String() string {
	return *r.Name
}
