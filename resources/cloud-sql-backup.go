package resources

import (
	"context"
	"fmt"

	"github.com/gotidy/ptr"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudSQLBackupResource = "CloudSQLBackup"

func init() {
	registry.Register(&registry.Registration{
		Name:     CloudSQLBackupResource,
		Scope:    nuke.Project,
		Resource: &CloudSQLBackup{},
		Lister:   &CloudSQLBackupLister{},
	})
}

type CloudSQLBackupLister struct {
	svc *sqladmin.Service
}

func (l *CloudSQLBackupLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "sqladmin.googleapis.com", CloudSQLBackupResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = sqladmin.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	instances, err := l.svc.Instances.List(*opts.Project).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, instance := range instances.Items {
		if instance.Region != *opts.Region {
			continue
		}

		backups, err := l.svc.BackupRuns.List(*opts.Project, instance.Name).Context(ctx).Do()
		if err != nil {
			continue
		}

		for _, backup := range backups.Items {
			resources = append(resources, &CloudSQLBackup{
				svc:        l.svc,
				project:    opts.Project,
				Instance:   ptr.String(instance.Name),
				ID:         ptr.Int64(backup.Id),
				Status:     ptr.String(backup.Status),
				Type:       ptr.String(backup.Type),
				StartTime:  ptr.String(backup.StartTime),
				EndTime:    ptr.String(backup.EndTime),
				Location:   ptr.String(backup.Location),
				BackupKind: ptr.String(backup.BackupKind),
			})
		}
	}

	return resources, nil
}

type CloudSQLBackup struct {
	svc      *sqladmin.Service
	deleteOp *sqladmin.Operation

	project    *string
	Instance   *string `description:"Name of the Cloud SQL instance"`
	ID         *int64  `description:"Backup run ID"`
	Status     *string `description:"Status of the backup (SUCCESSFUL, FAILED, etc.)"`
	Type       *string `description:"Type of backup (AUTOMATED, ON_DEMAND)"`
	StartTime  *string `description:"Start time of the backup"`
	EndTime    *string `description:"End time of the backup"`
	Location   *string `description:"Location of the backup"`
	BackupKind *string `description:"Kind of backup (SNAPSHOT, PHYSICAL)"`
}

func (r *CloudSQLBackup) Remove(ctx context.Context) (err error) {
	r.deleteOp, err = r.svc.BackupRuns.Delete(*r.project, *r.Instance, *r.ID).Context(ctx).Do()
	return err
}

func (r *CloudSQLBackup) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudSQLBackup) String() string {
	return fmt.Sprintf("%s -> %d", *r.Instance, *r.ID)
}

func (r *CloudSQLBackup) HandleWait(ctx context.Context) error {
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
