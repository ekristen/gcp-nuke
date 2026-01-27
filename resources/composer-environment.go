package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	composer "cloud.google.com/go/orchestration/airflow/service/apiv1"
	"cloud.google.com/go/orchestration/airflow/service/apiv1/servicepb"
	"google.golang.org/api/iterator"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComposerEnvironmentResource = "ComposerEnvironment"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComposerEnvironmentResource,
		Scope:    nuke.Project,
		Resource: &ComposerEnvironment{},
		Lister:   &ComposerEnvironmentLister{},
	})
}

type ComposerEnvironmentLister struct {
	svc *composer.EnvironmentsClient
}

func (l *ComposerEnvironmentLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ComposerEnvironmentLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "composer.googleapis.com", ComposerEnvironmentResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = composer.NewEnvironmentsClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &servicepb.ListEnvironmentsRequest{
		Parent: "projects/" + *opts.Project + "/locations/" + *opts.Region,
	}

	it := l.svc.ListEnvironments(ctx, req)
	for {
		env, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate composer environments")
			break
		}

		nameParts := strings.Split(env.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &ComposerEnvironment{
			svc:      l.svc,
			Project:  opts.Project,
			Region:   opts.Region,
			FullName: ptr.String(env.Name),
			Name:     ptr.String(name),
			State:    ptr.String(env.State.String()),
			Labels:   env.Labels,
		})
	}

	return resources, nil
}

type ComposerEnvironment struct {
	svc      *composer.EnvironmentsClient
	removeOp *composer.DeleteEnvironmentOperation
	Project  *string
	Region   *string
	FullName *string
	Name     *string           `description:"The name of the Composer environment"`
	State    *string           `description:"The current state of the environment"`
	Labels   map[string]string `property:"tagPrefix=label" description:"Labels associated with the environment"`
}

func (r *ComposerEnvironment) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteEnvironment(ctx, &servicepb.DeleteEnvironmentRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *ComposerEnvironment) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComposerEnvironment) String() string {
	return *r.Name
}

func (r *ComposerEnvironment) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("composer environment remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
