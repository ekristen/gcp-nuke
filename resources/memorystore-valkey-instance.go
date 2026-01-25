package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	memorystore "cloud.google.com/go/memorystore/apiv1"
	"cloud.google.com/go/memorystore/apiv1/memorystorepb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const MemorystoreValkeyInstanceResource = "MemorystoreValkeyInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:     MemorystoreValkeyInstanceResource,
		Scope:    nuke.Project,
		Resource: &MemorystoreValkeyInstance{},
		Lister:   &MemorystoreValkeyInstanceLister{},
	})
}

type MemorystoreValkeyInstanceLister struct {
	svc *memorystore.Client
}

func (l *MemorystoreValkeyInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "memorystore.googleapis.com"); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = memorystore.NewRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &memorystorepb.ListInstancesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListInstances(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate memorystore valkey instances")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &MemorystoreValkeyInstance{
			svc:        l.svc,
			project:    opts.Project,
			region:     opts.Region,
			Name:       &name,
			FullName:   &resp.Name,
			State:      resp.State.String(),
			ShardCount: resp.ShardCount,
			Labels:     resp.Labels,
		})
	}

	return resources, nil
}

func (l *MemorystoreValkeyInstanceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type MemorystoreValkeyInstance struct {
	svc        *memorystore.Client
	removeOp   *memorystore.DeleteInstanceOperation
	project    *string
	region     *string
	Name       *string
	FullName   *string
	State      string
	ShardCount int32
	Labels     map[string]string `property:"tagPrefix=label"`
}

func (r *MemorystoreValkeyInstance) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteInstance(ctx, &memorystorepb.DeleteInstanceRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *MemorystoreValkeyInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *MemorystoreValkeyInstance) String() string {
	return *r.Name
}

func (r *MemorystoreValkeyInstance) HandleWait(ctx context.Context) error {
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
