package resources

import (
	"context"
	"fmt"
	"strings"

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
		Lister: &CloudSQLBackupLister{
			multiRegion:             make(map[string]string),
			instancesWithBackupRuns: make(map[string]struct{}),
		},
	})
}

type CloudSQLBackupLister struct {
	svc                     *sqladmin.Service
	multiRegion             map[string]string
	instancesWithBackupRuns map[string]struct{}
}

func isMultiRegionLocation(location string) bool {
	loc := strings.ToLower(location)
	return loc == "us" || loc == "eu" || loc == "asia"
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

	backupRuns, err := l.svc.BackupRuns.List(*opts.Project, "-").Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, backup := range backupRuns.Items {
		loc := strings.ToLower(backup.Location)
		isMultiRegion := isMultiRegionLocation(backup.Location)
		isAccountedFor := false

		if isMultiRegion {
			key := fmt.Sprintf("run-%d", backup.Id)
			if _, ok := l.multiRegion[key]; !ok {
				l.multiRegion[key] = loc
			} else {
				isAccountedFor = true
			}
		}

		if !isMultiRegion && loc != *opts.Region {
			continue
		}

		if isMultiRegion && isAccountedFor {
			continue
		}

		l.instancesWithBackupRuns[backup.Instance] = struct{}{}

		resources = append(resources, &CloudSQLBackup{
			svc:           l.svc,
			project:       opts.Project,
			backupRunID:   ptr.Int64(backup.Id),
			Instance:      ptr.String(backup.Instance),
			State:         ptr.String(backup.Status),
			Type:          ptr.String(backup.Type),
			Location:      ptr.String(backup.Location),
			isFinalBackup: false,
		})
	}

	parent := fmt.Sprintf("projects/%s", *opts.Project)
	backups, err := l.svc.Backups.ListBackups(parent).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, backup := range backups.Backups {
		// Skip non-FINAL backups for instances that have backup runs
		// (they're already covered by BackupRuns API)
		if backup.Type != "FINAL" {
			if _, hasBackupRuns := l.instancesWithBackupRuns[backup.Instance]; hasBackupRuns {
				continue
			}
		}

		loc := strings.ToLower(backup.Location)
		isMultiRegion := isMultiRegionLocation(backup.Location)
		isAccountedFor := false

		nameParts := strings.Split(backup.Name, "/")
		backupID := nameParts[len(nameParts)-1]

		if isMultiRegion {
			key := fmt.Sprintf("backup-%s", backupID)
			if _, ok := l.multiRegion[key]; !ok {
				l.multiRegion[key] = loc
			} else {
				isAccountedFor = true
			}
		}

		if !isMultiRegion && loc != *opts.Region {
			continue
		}

		if isMultiRegion && isAccountedFor {
			continue
		}

		resources = append(resources, &CloudSQLBackup{
			svc:           l.svc,
			project:       opts.Project,
			backupID:      ptr.String(backupID),
			Instance:      ptr.String(backup.Instance),
			State:         ptr.String(backup.State),
			Type:          ptr.String(backup.Type),
			Location:      ptr.String(backup.Location),
			isFinalBackup: true,
		})
	}

	return resources, nil
}

type CloudSQLBackup struct {
	svc      *sqladmin.Service
	deleteOp *sqladmin.Operation

	project       *string
	backupID      *string // For final/retained backups (project-level)
	backupRunID   *int64  // For instance backups (BackupRuns API)
	isFinalBackup bool    // Determines which delete API to use
	Instance      *string `description:"Name of the Cloud SQL instance"`
	State         *string `description:"State of the backup"`
	Type          *string `description:"Type of backup (AUTOMATED, ON_DEMAND, FINAL)"`
	Location      *string `description:"Location of the backup"`
}

func (r *CloudSQLBackup) Remove(ctx context.Context) (err error) {
	if r.isFinalBackup {
		name := fmt.Sprintf("projects/%s/backups/%s", *r.project, *r.backupID)
		r.deleteOp, err = r.svc.Backups.DeleteBackup(name).Context(ctx).Do()
	} else {
		r.deleteOp, err = r.svc.BackupRuns.Delete(*r.project, *r.Instance, *r.backupRunID).Context(ctx).Do()
	}
	return err
}

func (r *CloudSQLBackup) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudSQLBackup) String() string {
	var id string
	if r.isFinalBackup {
		id = *r.backupID
	} else {
		id = fmt.Sprintf("%d", *r.backupRunID)
	}

	if r.Instance != nil && *r.Instance != "" {
		return fmt.Sprintf("%s -> %s", *r.Instance, id)
	}
	return id
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
