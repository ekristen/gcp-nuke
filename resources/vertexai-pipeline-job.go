package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	aiplatform "cloud.google.com/go/aiplatform/apiv1"
	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"google.golang.org/api/iterator"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const VertexAIPipelineJobResource = "VertexAIPipelineJob"

func init() {
	registry.Register(&registry.Registration{
		Name:     VertexAIPipelineJobResource,
		Scope:    nuke.Project,
		Resource: &VertexAIPipelineJob{},
		Lister:   &VertexAIPipelineJobLister{},
	})
}

type VertexAIPipelineJobLister struct {
	svc *aiplatform.PipelineClient
}

func (l *VertexAIPipelineJobLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *VertexAIPipelineJobLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "aiplatform.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = aiplatform.NewPipelineClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &aiplatformpb.ListPipelineJobsRequest{
		Parent: "projects/" + *opts.Project + "/locations/" + *opts.Region,
	}

	it := l.svc.ListPipelineJobs(ctx, req)
	for {
		job, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate vertex ai pipeline jobs")
			break
		}

		nameParts := strings.Split(job.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &VertexAIPipelineJob{
			svc:         l.svc,
			Project:     opts.Project,
			Region:      opts.Region,
			FullName:    ptr.String(job.Name),
			Name:        ptr.String(name),
			DisplayName: ptr.String(job.DisplayName),
			State:       ptr.String(job.State.String()),
			Labels:      job.Labels,
		})
	}

	return resources, nil
}

type VertexAIPipelineJob struct {
	svc         *aiplatform.PipelineClient
	removeOp    *aiplatform.DeletePipelineJobOperation
	Project     *string
	Region      *string
	FullName    *string
	Name        *string           `description:"The resource name of the pipeline job"`
	DisplayName *string           `description:"The display name of the pipeline job"`
	State       *string           `description:"The current state of the pipeline job"`
	Labels      map[string]string `property:"tagPrefix=label" description:"Labels associated with the pipeline job"`
}

func (r *VertexAIPipelineJob) Remove(ctx context.Context) (err error) {
	state := *r.State
	if state == "PIPELINE_STATE_RUNNING" || state == "PIPELINE_STATE_PENDING" || state == "PIPELINE_STATE_QUEUED" {
		cancelErr := r.svc.CancelPipelineJob(ctx, &aiplatformpb.CancelPipelineJobRequest{
			Name: *r.FullName,
		})
		if cancelErr != nil {
			logrus.WithError(cancelErr).Warn("failed to cancel pipeline job before deletion")
		}
	}

	r.removeOp, err = r.svc.DeletePipelineJob(ctx, &aiplatformpb.DeletePipelineJobRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *VertexAIPipelineJob) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VertexAIPipelineJob) String() string {
	return *r.DisplayName
}

func (r *VertexAIPipelineJob) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("vertex ai pipeline job remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
