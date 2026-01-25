package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	"google.golang.org/api/iterator"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const SpannerInstanceResource = "SpannerInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:      SpannerInstanceResource,
		Scope:     nuke.Project,
		Resource:  &SpannerInstance{},
		Lister:    &SpannerInstanceLister{},
		DependsOn: []string{SpannerDatabaseResource},
	})
}

type SpannerInstanceLister struct {
	svc *instance.InstanceAdminClient
}

func (l *SpannerInstanceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *SpannerInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "spanner.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = instance.NewInstanceAdminClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &instancepb.ListInstancesRequest{
		Parent: "projects/" + *opts.Project,
	}

	it := l.svc.ListInstances(ctx, req)
	for {
		inst, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate spanner instances")
			break
		}

		nameParts := strings.Split(inst.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &SpannerInstance{
			svc:       l.svc,
			Project:   opts.Project,
			FullName:  ptr.String(inst.Name),
			Name:      ptr.String(name),
			NodeCount: ptr.Int32(inst.NodeCount),
			State:     ptr.String(inst.State.String()),
			Labels:    inst.Labels,
		})
	}

	return resources, nil
}

type SpannerInstance struct {
	svc       *instance.InstanceAdminClient
	Project   *string
	FullName  *string
	Name      *string           `description:"The name of the Spanner instance"`
	State     *string           `description:"The current state of the instance"`
	NodeCount *int32            `description:"The number of nodes in the instance"`
	Labels    map[string]string `property:"tagPrefix=label" description:"Labels associated with the instance"`
}

func (r *SpannerInstance) Remove(ctx context.Context) error {
	return r.svc.DeleteInstance(ctx, &instancepb.DeleteInstanceRequest{
		Name: *r.FullName,
	})
}

func (r *SpannerInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *SpannerInstance) String() string {
	return *r.Name
}
