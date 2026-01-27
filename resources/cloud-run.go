package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudRunResource = "CloudRun"

func init() {
	registry.Register(&registry.Registration{
		Name:     CloudRunResource,
		Scope:    nuke.Project,
		Resource: &CloudRun{},
		Lister:   &CloudRunLister{},
	})
}

type CloudRunLister struct {
	svc *run.ServicesClient
}

func (l *CloudRunLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *CloudRunLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "run.googleapis.com", CloudRunResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = run.NewServicesClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &runpb.ListServicesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListServices(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &CloudRun{
			svc:      l.svc,
			FullName: ptr.String(resp.Name),
			Name:     ptr.String(name),
			Project:  opts.Project,
			Region:   opts.Region,
			Labels:   resp.Labels,
		})
	}

	return resources, nil
}

type CloudRun struct {
	svc      *run.ServicesClient
	removeOp *run.DeleteServiceOperation
	Project  *string
	Region   *string
	FullName *string
	Name     *string           `description:"The name of the cloud run"`
	Labels   map[string]string `property:"tagPrefix=label" description:"The labels associated with the cloud run"`
}

func (r *CloudRun) Filter() error {
	if r.Labels != nil && r.Labels["goog-managed-by"] == "cloudfunctions" {
		return errors.New("cannot remove cloud run that is managed by cloud functions")
	}

	return nil
}

func (r *CloudRun) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteService(ctx, &runpb.DeleteServiceRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *CloudRun) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudRun) String() string {
	return *r.Name
}

func (r *CloudRun) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if _, err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("network remove op polling encountered error")
		return err
	}

	return nil
}
