package resources

import (
	"context"

	"github.com/sirupsen/logrus"

	"cloud.google.com/go/bigtable"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const BigtableTableResource = "BigtableTable"

func init() {
	registry.Register(&registry.Registration{
		Name:     BigtableTableResource,
		Scope:    nuke.Project,
		Resource: &BigtableTable{},
		Lister:   &BigtableTableLister{},
	})
}

type BigtableTableLister struct {
	instanceSvc *bigtable.InstanceAdminClient
}

func (l *BigtableTableLister) Close() {
	if l.instanceSvc != nil {
		_ = l.instanceSvc.Close()
	}
}

func (l *BigtableTableLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "bigtable.googleapis.com", BigtableTableResource); err != nil {
		return resources, nil
	}

	if l.instanceSvc == nil {
		var err error
		l.instanceSvc, err = bigtable.NewInstanceAdminClient(ctx, *opts.Project, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	instances, err := l.instanceSvc.Instances(ctx)
	if err != nil {
		logrus.WithError(err).Error("unable to list bigtable instances")
		return resources, nil
	}

	for _, inst := range instances {
		adminClient, err := bigtable.NewAdminClient(ctx, *opts.Project, inst.Name, opts.ClientOptions...)
		if err != nil {
			logrus.WithError(err).Errorf("unable to create admin client for instance %s", inst.Name)
			continue
		}

		tables, err := adminClient.Tables(ctx)
		if err != nil {
			logrus.WithError(err).Errorf("unable to list tables for instance %s", inst.Name)
			_ = adminClient.Close()
			continue
		}

		for _, tableName := range tables {
			resources = append(resources, &BigtableTable{
				svc:      adminClient,
				project:  opts.Project,
				Instance: inst.Name,
				Name:     tableName,
			})
		}
	}

	return resources, nil
}

type BigtableTable struct {
	svc      *bigtable.AdminClient
	project  *string
	Instance string
	Name     string
}

func (r *BigtableTable) Remove(ctx context.Context) error {
	return r.svc.DeleteTable(ctx, r.Name)
}

func (r *BigtableTable) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *BigtableTable) String() string {
	return r.Instance + "/" + r.Name
}
