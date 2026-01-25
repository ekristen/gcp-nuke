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

const VertexAIModelResource = "VertexAIModel"

func init() {
	registry.Register(&registry.Registration{
		Name:      VertexAIModelResource,
		Scope:     nuke.Project,
		Resource:  &VertexAIModel{},
		Lister:    &VertexAIModelLister{},
		DependsOn: []string{VertexAIEndpointResource},
	})
}

type VertexAIModelLister struct {
	svc *aiplatform.ModelClient
}

func (l *VertexAIModelLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *VertexAIModelLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "aiplatform.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = aiplatform.NewModelClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &aiplatformpb.ListModelsRequest{
		Parent: "projects/" + *opts.Project + "/locations/" + *opts.Region,
	}

	it := l.svc.ListModels(ctx, req)
	for {
		model, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate vertex ai models")
			break
		}

		nameParts := strings.Split(model.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &VertexAIModel{
			svc:         l.svc,
			Project:     opts.Project,
			Region:      opts.Region,
			FullName:    ptr.String(model.Name),
			Name:        ptr.String(name),
			DisplayName: ptr.String(model.DisplayName),
			Labels:      model.Labels,
		})
	}

	return resources, nil
}

type VertexAIModel struct {
	svc         *aiplatform.ModelClient
	removeOp    *aiplatform.DeleteModelOperation
	Project     *string
	Region      *string
	FullName    *string
	Name        *string           `description:"The resource name of the model"`
	DisplayName *string           `description:"The display name of the model"`
	Labels      map[string]string `property:"tagPrefix=label" description:"Labels associated with the model"`
}

func (r *VertexAIModel) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteModel(ctx, &aiplatformpb.DeleteModelRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *VertexAIModel) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VertexAIModel) String() string {
	return *r.DisplayName
}

func (r *VertexAIModel) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("vertex ai model remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
