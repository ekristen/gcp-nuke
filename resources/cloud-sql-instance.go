package resources

import (
	"context"
	"fmt"
	"github.com/ekristen/libnuke/pkg/settings"
	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudSQLInstanceResource = "CloudSQLInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:   CloudSQLInstanceResource,
		Scope:  nuke.Project,
		Lister: &CloudSQLInstanceLister{},
		Settings: []string{
			"DisableDeletionProtection",
		},
	})
}

type CloudSQLInstanceLister struct {
	svc *sqladmin.Service
}

func (l *CloudSQLInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "sqladmin.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = sqladmin.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	resp, err := l.svc.Instances.List(*opts.Project).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, instance := range resp.Items {
		if instance.Region != *opts.Region {
			continue
		}

		resources = append(resources, &CloudSQLInstance{
			svc:              l.svc,
			project:          opts.Project,
			region:           opts.Region,
			Name:             ptr.String(instance.Name),
			State:            ptr.String(instance.State),
			Labels:           instance.Settings.UserLabels,
			CreationDate:     ptr.String(instance.CreateTime),
			DatabaseVersion:  ptr.String(instance.DatabaseVersion),
			instanceSettings: instance.Settings,
		})
	}

	return resources, nil
}

type CloudSQLInstance struct {
	svc      *sqladmin.Service
	deleteOp *sqladmin.Operation
	settings *settings.Setting

	project         *string
	region          *string
	Name            *string
	State           *string
	Labels          map[string]string
	CreationDate    *string
	DatabaseVersion *string

	instanceSettings *sqladmin.Settings
}

func (r *CloudSQLInstance) Settings(setting *settings.Setting) {
	r.settings = setting
}

func (r *CloudSQLInstance) Remove(ctx context.Context) (err error) {
	if disableErr := r.disableDeletionProtection(ctx); disableErr != nil {
		return disableErr
	}

	r.deleteOp, err = r.svc.Instances.Delete(*r.project, *r.Name).Context(ctx).Do()
	return err
}

func (r *CloudSQLInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudSQLInstance) String() string {
	return *r.Name
}

func (r *CloudSQLInstance) HandleWait(ctx context.Context) error {
	if r.deleteOp == nil {
		return nil
	}

	if op, err := r.svc.Operations.Get(*r.project, r.deleteOp.Name).Context(ctx).Do(); err == nil {
		if op.Status == "DONE" {
			if op.Error != nil {
				return fmt.Errorf("delete error on '%s': %s", op.TargetLink, op.Error.Errors[0].Message)
			}
		}
	} else {
		return err
	}

	return nil
}

func (r *CloudSQLInstance) disableDeletionProtection(ctx context.Context) error {
	if r.settings != nil && r.settings.Get("DisableDeletionProtection").(bool) {
		logrus.Trace("disabling deletion protection")

		r.instanceSettings.DeletionProtectionEnabled = false
		op, err := r.svc.Instances.Update(*r.project, *r.Name, &sqladmin.DatabaseInstance{
			Settings: r.instanceSettings,
		}).Context(ctx).Do()
		if err != nil {
			return err
		}

		for {
			op, err = r.svc.Operations.Get(*r.project, op.Name).Context(ctx).Do()
			if err != nil {
				return err
			}
			if op.Status == "DONE" {
				break
			}
		}
	}
	return nil
}
