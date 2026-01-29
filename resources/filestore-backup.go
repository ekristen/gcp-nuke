package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	filestore "cloud.google.com/go/filestore/apiv1"
	"cloud.google.com/go/filestore/apiv1/filestorepb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const FilestoreBackupResource = "FilestoreBackup"

func init() {
	registry.Register(&registry.Registration{
		Name:     FilestoreBackupResource,
		Scope:    nuke.Project,
		Resource: &FilestoreBackup{},
		Lister:   &FilestoreBackupLister{},
	})
}

type FilestoreBackupLister struct {
	svc *filestore.CloudFilestoreManagerClient
}

func (l *FilestoreBackupLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "file.googleapis.com", FilestoreBackupResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = filestore.NewCloudFilestoreManagerClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &filestorepb.ListBackupsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListBackups(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate filestore backups")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &FilestoreBackup{
			svc:            l.svc,
			project:        opts.Project,
			region:         opts.Region,
			Name:           &name,
			FullName:       &resp.Name,
			State:          resp.State.String(),
			SourceInstance: &resp.SourceInstance,
			Labels:         resp.Labels,
		})
	}

	return resources, nil
}

func (l *FilestoreBackupLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type FilestoreBackup struct {
	svc            *filestore.CloudFilestoreManagerClient
	removeOp       *filestore.DeleteBackupOperation
	project        *string
	region         *string
	Name           *string
	FullName       *string
	State          string
	SourceInstance *string
	Labels         map[string]string `property:"tagPrefix=label"`
}

func (r *FilestoreBackup) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteBackup(ctx, &filestorepb.DeleteBackupRequest{
		Name: *r.FullName,
	})
	if err != nil {
		logrus.WithError(err).WithField("backup", *r.Name).Trace("filestore backup delete error")
		return liberror.ErrWaitResource(fmt.Sprintf("delete failed: %v", err))
	}
	return nil
}

func (r *FilestoreBackup) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *FilestoreBackup) String() string {
	return *r.Name
}

func (r *FilestoreBackup) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
