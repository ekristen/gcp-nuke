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

const CloudDeployTargetResource = "CloudDeployTarget"

func init() {
	registry.Register(&registry.Registration{
		Name:     CloudDeployTargetResource,
		Scope:    nuke.Project,
		Resource: &CloudDeployTarget{},
		Lister:   &CloudDeployTargetLister{},
	})
}

type CloudDeployTargetLister struct {
	svc *clouddeploy.Service
}

func (l *CloudDeployTargetLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "clouddeploy.googleapis.com", CloudDeployTargetResource); err != nil {
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
	req := l.svc.Projects.Locations.Targets.List(parent)
	if err := req.Pages(ctx, func(page *clouddeploy.ListTargetsResponse) error {
		for _, target := range page.Targets {
			nameParts := strings.Split(target.Name, "/")
			name := nameParts[len(nameParts)-1]

			resources = append(resources, &CloudDeployTarget{
				svc:      l.svc,
				FullName: ptr.String(target.Name),
				Name:     ptr.String(name),
				Project:  opts.Project,
				Region:   opts.Region,
				Labels:   target.Labels,
			})
		}
		return nil
	}); err != nil {
		return resources, err
	}

	return resources, nil
}

type CloudDeployTarget struct {
	svc      *clouddeploy.Service
	Project  *string
	Region   *string
	FullName *string
	Name     *string           `description:"The name of the target"`
	Labels   map[string]string `property:"tagPrefix=label" description:"The labels associated with the target"`
}

func (r *CloudDeployTarget) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.Targets.Delete(*r.FullName).Context(ctx).Do()
	return err
}

func (r *CloudDeployTarget) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudDeployTarget) String() string {
	return *r.Name
}
