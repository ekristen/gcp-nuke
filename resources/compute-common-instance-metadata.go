package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComputeCommonInstanceMetadataResource = "ComputeCommonInstanceMetadata"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeCommonInstanceMetadataResource,
		Scope:    nuke.Project,
		Resource: &ComputeCommonInstanceMetadata{},
		Lister:   &ComputeCommonInstanceMetadataLister{},
	})
}

type ComputeCommonInstanceMetadataLister struct {
	svc *compute.ProjectsClient
}

func (l *ComputeCommonInstanceMetadataLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewProjectsRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.GetProjectRequest{
		Project: *opts.Project,
	}

	proj, err := l.svc.Get(ctx, req)
	if err != nil {
		return nil, err
	}

	resources = append(resources, &ComputeCommonInstanceMetadata{
		svc:         l.svc,
		project:     proj.Name,
		Fingerprint: proj.CommonInstanceMetadata.Fingerprint,
		Items:       proj.CommonInstanceMetadata.Items,
	})

	return resources, nil
}

type ComputeCommonInstanceMetadata struct {
	svc         *compute.ProjectsClient
	removeOp    *compute.Operation
	project     *string
	Fingerprint *string
	Items       []*computepb.Items `property:"tagPrefix=item"`
}

func (r *ComputeCommonInstanceMetadata) Filter() error {
	if len(r.Items) == 0 {
		return fmt.Errorf("common instance metadata is empty")
	}
	if len(r.Items) == 1 && *r.Items[0].Key == "enable-oslogin" && *r.Items[0].Value == "true" {
		return fmt.Errorf("common instance metadata is default")
	}

	return nil
}

func (r *ComputeCommonInstanceMetadata) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.SetCommonInstanceMetadata(ctx, &computepb.SetCommonInstanceMetadataProjectRequest{
		Project: *r.project,
		MetadataResource: &computepb.Metadata{
			Items: []*computepb.Items{
				{
					Key:   ptr.String("enable-oslogin"),
					Value: ptr.String("true"),
				},
			},
		},
	})
	return err
}

func (r *ComputeCommonInstanceMetadata) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeCommonInstanceMetadata) String() string {
	return "common-instance-metadata"
}

func (r *ComputeCommonInstanceMetadata) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		if strings.Contains(err.Error(), "proto") && strings.Contains(err.Error(), "missing") {
			err = nil
		}
		if err != nil {
			logrus.
				WithField("resource", ComputeCommonInstanceMetadataResource).
				WithError(err).
				Trace("remove op polling encountered error")
			return err
		}
	}

	if !r.removeOp.Done() {
		return fmt.Errorf("operation still in progress")
	}

	return nil
}
