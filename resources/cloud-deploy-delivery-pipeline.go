package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"

	"google.golang.org/api/clouddeploy/v1"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudDeployDeliveryPipelineResource = "CloudDeployDeliveryPipeline"

func init() {
	registry.Register(&registry.Registration{
		Name:     CloudDeployDeliveryPipelineResource,
		Scope:    nuke.Project,
		Resource: &CloudDeployDeliveryPipeline{},
		Lister:   &CloudDeployDeliveryPipelineLister{},
	})
}

type CloudDeployDeliveryPipelineLister struct {
	svc *clouddeploy.Service
}

func (l *CloudDeployDeliveryPipelineLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "clouddeploy.googleapis.com", CloudDeployDeliveryPipelineResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = clouddeploy.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	parent := fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region)
	req := l.svc.Projects.Locations.DeliveryPipelines.List(parent)
	if err := req.Pages(ctx, func(page *clouddeploy.ListDeliveryPipelinesResponse) error {
		for _, pipeline := range page.DeliveryPipelines {
			nameParts := strings.Split(pipeline.Name, "/")
			name := nameParts[len(nameParts)-1]

			resources = append(resources, &CloudDeployDeliveryPipeline{
				svc:      l.svc,
				FullName: ptr.String(pipeline.Name),
				Name:     ptr.String(name),
				Project:  opts.Project,
				Region:   opts.Region,
				Labels:   pipeline.Labels,
			})
		}
		return nil
	}); err != nil {
		return resources, err
	}

	return resources, nil
}

type CloudDeployDeliveryPipeline struct {
	svc      *clouddeploy.Service
	Project  *string
	Region   *string
	FullName *string
	Name     *string           `description:"The name of the delivery pipeline"`
	Labels   map[string]string `property:"tagPrefix=label" description:"The labels associated with the delivery pipeline"`
}

func (r *CloudDeployDeliveryPipeline) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.DeliveryPipelines.Delete(*r.FullName).Force(true).Context(ctx).Do()
	return err
}

func (r *CloudDeployDeliveryPipeline) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudDeployDeliveryPipeline) String() string {
	return *r.Name
}
