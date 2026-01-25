package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	scheduler "cloud.google.com/go/scheduler/apiv1"
	"cloud.google.com/go/scheduler/apiv1/schedulerpb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudSchedulerJobResource = "CloudSchedulerJob"

func init() {
	registry.Register(&registry.Registration{
		Name:     CloudSchedulerJobResource,
		Scope:    nuke.Project,
		Resource: &CloudSchedulerJob{},
		Lister:   &CloudSchedulerJobLister{},
	})
}

type CloudSchedulerJobLister struct {
	svc *scheduler.CloudSchedulerClient
}

func (l *CloudSchedulerJobLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *CloudSchedulerJobLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "cloudscheduler.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = scheduler.NewCloudSchedulerRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &schedulerpb.ListJobsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListJobs(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			if strings.Contains(err.Error(), "not a valid location") {
				return resources, nil
			}
			logrus.WithError(err).Error("unable to iterate cloud scheduler jobs")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &CloudSchedulerJob{
			svc:      l.svc,
			FullName: ptr.String(resp.Name),
			Name:     ptr.String(name),
			Project:  opts.Project,
			Region:   opts.Region,
			State:    ptr.String(resp.State.String()),
		})
	}

	return resources, nil
}

type CloudSchedulerJob struct {
	svc      *scheduler.CloudSchedulerClient
	Project  *string
	Region   *string
	FullName *string
	Name     *string `description:"The name of the cloud scheduler job"`
	State    *string `description:"The state of the job (ENABLED, PAUSED, DISABLED, UPDATE_FAILED)"`
}

func (r *CloudSchedulerJob) Remove(ctx context.Context) error {
	return r.svc.DeleteJob(ctx, &schedulerpb.DeleteJobRequest{
		Name: *r.FullName,
	})
}

func (r *CloudSchedulerJob) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudSchedulerJob) String() string {
	return *r.Name
}
