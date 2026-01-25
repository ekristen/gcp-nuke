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

const VertexAIEndpointResource = "VertexAIEndpoint"

func init() {
	registry.Register(&registry.Registration{
		Name:     VertexAIEndpointResource,
		Scope:    nuke.Project,
		Resource: &VertexAIEndpoint{},
		Lister:   &VertexAIEndpointLister{},
	})
}

type VertexAIEndpointLister struct {
	svc *aiplatform.EndpointClient
}

func (l *VertexAIEndpointLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *VertexAIEndpointLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "aiplatform.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = aiplatform.NewEndpointClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &aiplatformpb.ListEndpointsRequest{
		Parent: "projects/" + *opts.Project + "/locations/" + *opts.Region,
	}

	it := l.svc.ListEndpoints(ctx, req)
	for {
		endpoint, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate vertex ai endpoints")
			break
		}

		nameParts := strings.Split(endpoint.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &VertexAIEndpoint{
			svc:         l.svc,
			Project:     opts.Project,
			Region:      opts.Region,
			FullName:    ptr.String(endpoint.Name),
			Name:        ptr.String(name),
			DisplayName: ptr.String(endpoint.DisplayName),
			Labels:      endpoint.Labels,
		})
	}

	return resources, nil
}

type VertexAIEndpoint struct {
	svc         *aiplatform.EndpointClient
	removeOp    *aiplatform.DeleteEndpointOperation
	Project     *string
	Region      *string
	FullName    *string
	Name        *string           `description:"The resource name of the endpoint"`
	DisplayName *string           `description:"The display name of the endpoint"`
	Labels      map[string]string `property:"tagPrefix=label" description:"Labels associated with the endpoint"`
}

func (r *VertexAIEndpoint) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteEndpoint(ctx, &aiplatformpb.DeleteEndpointRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *VertexAIEndpoint) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VertexAIEndpoint) String() string {
	return *r.DisplayName
}

func (r *VertexAIEndpoint) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("vertex ai endpoint remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
