package resources

import (
	"context"
	"errors"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/ekristen/gcp-nuke/pkg/nuke"
	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

const PSCProducerResource = "PSCProducer"

func init() {
	registry.Register(&registry.Registration{
		Name:     PSCProducerResource,
		Scope:    nuke.Project,
		Resource: &PSCProducer{},
		Lister:   &PSCProducerLister{},
		DependsOn: []string{
			"ComputeForwardingRule",
			"ComputeBackendService",
		},
	})
}

type PSCProducerLister struct {
	svc *compute.ServiceAttachmentsClient
}

func (l *PSCProducerLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *PSCProducerLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err == nil {
		regionalResources, err := l.listRegional(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional PSC producers")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *PSCProducerLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewServiceAttachmentsRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListServiceAttachmentsRequest{
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
			logrus.WithError(err).Error("unable to iterate PSC producers")
			break
		}

		resources = append(resources, &PSCProducer{
			svc:     l.svc,
			project: opts.Project,
			region:  opts.Region,
			Name:    resp.Name,
		})
	}

	return resources, nil
}

type PSCProducer struct {
	svc      *compute.ServiceAttachmentsClient
	removeOp *compute.Operation
	project  *string
	region   *string
	Name     *string
}

func (r *PSCProducer) Remove(ctx context.Context) error {
	var err error
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteServiceAttachmentRequest{
		Project:           *r.project,
		Region:            *r.region,
		ServiceAttachment: *r.Name,
	})
	if err != nil {
		return err
	}

	return r.HandleWait(ctx)
}

func (r *PSCProducer) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *PSCProducer) String() string {
	return *r.Name
}

func (r *PSCProducer) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("PSC producer remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Error("unable to delete PSC producer")
			return removeErr
		}
	}

	return nil
}
