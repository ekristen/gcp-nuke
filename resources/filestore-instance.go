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

const FilestoreInstanceResource = "FilestoreInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:     FilestoreInstanceResource,
		Scope:    nuke.Project,
		Resource: &FilestoreInstance{},
		Lister:   &FilestoreInstanceLister{},
	})
}

type FilestoreInstanceLister struct {
	svc *filestore.CloudFilestoreManagerClient
}

func (l *FilestoreInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "file.googleapis.com", FilestoreInstanceResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = filestore.NewCloudFilestoreManagerClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	for _, zone := range opts.Zones {
		req := &filestorepb.ListInstancesRequest{
			Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, zone),
		}
		it := l.svc.ListInstances(ctx, req)
		for {
			resp, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate filestore instances")
				break
			}

			nameParts := strings.Split(resp.Name, "/")
			name := nameParts[len(nameParts)-1]

			zoneCopy := zone
			resources = append(resources, &FilestoreInstance{
				svc:      l.svc,
				project:  opts.Project,
				zone:     &zoneCopy,
				Name:     &name,
				FullName: &resp.Name,
				Tier:     resp.Tier.String(),
				State:    resp.State.String(),
				Labels:   resp.Labels,
			})
		}
	}

	return resources, nil
}

func (l *FilestoreInstanceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type FilestoreInstance struct {
	svc      *filestore.CloudFilestoreManagerClient
	removeOp *filestore.DeleteInstanceOperation
	project  *string
	zone     *string
	Name     *string
	FullName *string
	Tier     string
	State    string
	Labels   map[string]string `property:"tagPrefix=label"`
}

func (r *FilestoreInstance) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteInstance(ctx, &filestorepb.DeleteInstanceRequest{
		Name:  *r.FullName,
		Force: true,
	})
	if err != nil {
		logrus.WithError(err).WithField("instance", *r.Name).Trace("filestore delete error")
		return liberror.ErrWaitResource(fmt.Sprintf("delete failed: %v", err))
	}
	return nil
}

func (r *FilestoreInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *FilestoreInstance) String() string {
	return *r.Name
}

func (r *FilestoreInstance) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("remove op polling encountered error")
		return liberror.ErrWaitResource(fmt.Sprintf("poll failed: %v", err))
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
