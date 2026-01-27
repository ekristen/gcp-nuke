package resources

import (
	"context"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	dataflow "google.golang.org/api/dataflow/v1b3"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const DataflowJobResource = "DataflowJob"

func init() {
	registry.Register(&registry.Registration{
		Name:     DataflowJobResource,
		Scope:    nuke.Project,
		Resource: &DataflowJob{},
		Lister:   &DataflowJobLister{},
	})
}

type DataflowJobLister struct {
	svc *dataflow.Service
}

func (l *DataflowJobLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "dataflow.googleapis.com", DataflowJobResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = dataflow.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	resp, err := l.svc.Projects.Locations.Jobs.List(*opts.Project, *opts.Region).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, job := range resp.Jobs {
		if job.CurrentState == "JOB_STATE_CANCELLED" ||
			job.CurrentState == "JOB_STATE_DRAINED" ||
			job.CurrentState == "JOB_STATE_DONE" ||
			job.CurrentState == "JOB_STATE_FAILED" {
			continue
		}

		resources = append(resources, &DataflowJob{
			svc:          l.svc,
			Project:      opts.Project,
			Region:       opts.Region,
			ID:           ptr.String(job.Id),
			Name:         ptr.String(job.Name),
			Type:         ptr.String(job.Type),
			CurrentState: ptr.String(job.CurrentState),
			CreateTime:   ptr.String(job.CreateTime),
			Labels:       job.Labels,
		})
	}

	return resources, nil
}

type DataflowJob struct {
	svc          *dataflow.Service
	Project      *string
	Region       *string
	ID           *string           `description:"The unique ID of the Dataflow job"`
	Name         *string           `description:"The name of the Dataflow job"`
	Type         *string           `description:"The type of the job (batch or streaming)"`
	CurrentState *string           `description:"The current state of the job"`
	CreateTime   *string           `description:"The time the job was created"`
	Labels       map[string]string `property:"tagPrefix=label" description:"Labels associated with the job"`
}

func (r *DataflowJob) Remove(ctx context.Context) error {
	updateJob := &dataflow.Job{
		RequestedState: "JOB_STATE_CANCELLED",
	}

	_, err := r.svc.Projects.Locations.Jobs.Update(*r.Project, *r.Region, *r.ID, updateJob).Context(ctx).Do()
	if err != nil {
		logrus.WithError(err).Warn("failed to cancel job, trying drain")
		updateJob.RequestedState = "JOB_STATE_DRAINED"
		_, err = r.svc.Projects.Locations.Jobs.Update(*r.Project, *r.Region, *r.ID, updateJob).Context(ctx).Do()
	}
	return err
}

func (r *DataflowJob) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *DataflowJob) String() string {
	return *r.Name
}
