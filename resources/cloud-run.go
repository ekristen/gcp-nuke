package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotidy/ptr"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudRunResource = "CloudRun"

func init() {
	registry.Register(&registry.Registration{
		Name:   CloudRunResource,
		Scope:  nuke.Project,
		Lister: &CloudRunLister{},
	})
}

type CloudRunLister struct {
	svc *run.ServicesClient
}

func (l *CloudRunLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = run.NewServicesClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	// NOTE: you might have to modify the code below to actually work, this currently does not
	// inspect the aws sdk instead is a jumping off point
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
	Name     *string
	Labels   map[string]string
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
