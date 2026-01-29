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

const CloudRunJobResource = "CloudRunJob"

func init() {
	registry.Register(&registry.Registration{
		Name:     CloudRunJobResource,
		Scope:    nuke.Project,
		Resource: &CloudRunJob{},
		Lister:   &CloudRunJobLister{},
	})
}

type CloudRunJobLister struct {
	svc *run.JobsClient
}

func (l *CloudRunJobLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *CloudRunJobLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "run.googleapis.com", CloudRunJobResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = run.NewJobsClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &runpb.ListJobsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListJobs(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate cloud run jobs")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &CloudRunJob{
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

type CloudRunJob struct {
	svc      *run.JobsClient
	removeOp *run.DeleteJobOperation
	Project  *string
	Region   *string
	FullName *string
	Name     *string           `description:"The name of the cloud run job"`
	Labels   map[string]string `property:"tagPrefix=label" description:"The labels associated with the cloud run job"`
}

func (r *CloudRunJob) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteJob(ctx, &runpb.DeleteJobRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *CloudRunJob) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudRunJob) String() string {
	return *r.Name
}

func (r *CloudRunJob) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if _, err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("cloud run job remove op polling encountered error")
		return err
	}

	return nil
}
