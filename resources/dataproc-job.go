package resources

import (
	"context"
	"errors"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	dataproc "cloud.google.com/go/dataproc/v2/apiv1"
	"cloud.google.com/go/dataproc/v2/apiv1/dataprocpb"
	"google.golang.org/api/iterator"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const DataprocJobResource = "DataprocJob"

func init() {
	registry.Register(&registry.Registration{
		Name:     DataprocJobResource,
		Scope:    nuke.Project,
		Resource: &DataprocJob{},
		Lister:   &DataprocJobLister{},
	})
}

type DataprocJobLister struct {
	svc *dataproc.JobControllerClient
}

func (l *DataprocJobLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *DataprocJobLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "dataproc.googleapis.com", DataprocJobResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = dataproc.NewJobControllerClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &dataprocpb.ListJobsRequest{
		ProjectId: *opts.Project,
		Region:    *opts.Region,
	}

	it := l.svc.ListJobs(ctx, req)
	for {
		job, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate dataproc jobs")
			break
		}

		state := job.Status.State.String()
		if state == "DONE" || state == "CANCELLED" || state == "ERROR" {
			continue
		}

		resources = append(resources, &DataprocJob{
			svc:     l.svc,
			Project: opts.Project,
			Region:  opts.Region,
			ID:      ptr.String(job.Reference.JobId),
			State:   ptr.String(state),
			Labels:  job.Labels,
		})
	}

	return resources, nil
}

type DataprocJob struct {
	svc     *dataproc.JobControllerClient
	Project *string
	Region  *string
	ID      *string           `description:"The ID of the Dataproc job"`
	State   *string           `description:"The current state of the job"`
	Labels  map[string]string `property:"tagPrefix=label" description:"Labels associated with the job"`
}

func (r *DataprocJob) Remove(ctx context.Context) error {
	_, err := r.svc.CancelJob(ctx, &dataprocpb.CancelJobRequest{
		ProjectId: *r.Project,
		Region:    *r.Region,
		JobId:     *r.ID,
	})
	if err != nil {
		logrus.WithError(err).Warn("failed to cancel dataproc job")
	}

	return r.svc.DeleteJob(ctx, &dataprocpb.DeleteJobRequest{
		ProjectId: *r.Project,
		Region:    *r.Region,
		JobId:     *r.ID,
	})
}

func (r *DataprocJob) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *DataprocJob) String() string {
	return *r.ID
}
