package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ArtifactRegistryRepositoryResource = "ArtifactRegistryRepository"

func init() {
	registry.Register(&registry.Registration{
		Name:     ArtifactRegistryRepositoryResource,
		Scope:    nuke.Project,
		Resource: &ArtifactRegistryRepository{},
		Lister:   &ArtifactRegistryRepositoryLister{},
	})
}

type ArtifactRegistryRepositoryLister struct {
	svc *artifactregistry.Client
}

func (l *ArtifactRegistryRepositoryLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "artifactregistry.googleapis.com", ArtifactRegistryRepositoryResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = artifactregistry.NewClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &artifactregistrypb.ListRepositoriesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListRepositories(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate artifact registry repositories")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &ArtifactRegistryRepository{
			svc:      l.svc,
			project:  opts.Project,
			region:   opts.Region,
			Name:     &name,
			FullName: &resp.Name,
			Format:   resp.Format.String(),
			Labels:   resp.Labels,
		})
	}

	return resources, nil
}

func (l *ArtifactRegistryRepositoryLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type ArtifactRegistryRepository struct {
	svc      *artifactregistry.Client
	removeOp *artifactregistry.DeleteRepositoryOperation
	project  *string
	region   *string
	Name     *string
	FullName *string
	Format   string
	Labels   map[string]string `property:"tagPrefix=label"`
}

func (r *ArtifactRegistryRepository) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteRepository(ctx, &artifactregistrypb.DeleteRepositoryRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *ArtifactRegistryRepository) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ArtifactRegistryRepository) String() string {
	return *r.Name
}

func (r *ArtifactRegistryRepository) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
