package resources

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"cloud.google.com/go/bigtable"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const BigtableInstanceResource = "BigtableInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:     BigtableInstanceResource,
		Scope:    nuke.Project,
		Resource: &BigtableInstance{},
		Lister:   &BigtableInstanceLister{},
		DependsOn: []string{
			BigtableTableResource,
		},
	})
}

type BigtableInstanceLister struct {
	svc *bigtable.InstanceAdminClient
}

func (l *BigtableInstanceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *BigtableInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "bigtable.googleapis.com", BigtableInstanceResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = bigtable.NewInstanceAdminClient(ctx, *opts.Project)
		if err != nil {
			return nil, err
		}
	}

	instances, err := l.svc.Instances(ctx)
	if err != nil {
		logrus.WithError(err).Error("unable to list bigtable instances")
		return resources, nil
	}

	for _, inst := range instances {
		resources = append(resources, &BigtableInstance{
			svc:         l.svc,
			project:     opts.Project,
			Name:        inst.Name,
			DisplayName: inst.DisplayName,
			State:       fmt.Sprintf("%v", inst.InstanceState),
			Labels:      inst.Labels,
		})
	}

	return resources, nil
}

type BigtableInstance struct {
	svc         *bigtable.InstanceAdminClient
	project     *string
	Name        string
	DisplayName string
	State       string
	Labels      map[string]string `property:"tagPrefix=label"`
}

func (r *BigtableInstance) Remove(ctx context.Context) error {
	return r.svc.DeleteInstance(ctx, r.Name)
}

func (r *BigtableInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *BigtableInstance) String() string {
	return r.Name
}
