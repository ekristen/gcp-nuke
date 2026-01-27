package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	redis "cloud.google.com/go/redis/apiv1"
	"cloud.google.com/go/redis/apiv1/redispb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const MemorystoreRedisInstanceResource = "MemorystoreRedisInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:     MemorystoreRedisInstanceResource,
		Scope:    nuke.Project,
		Resource: &MemorystoreRedisInstance{},
		Lister:   &MemorystoreRedisInstanceLister{},
	})
}

type MemorystoreRedisInstanceLister struct {
	svc *redis.CloudRedisClient
}

func (l *MemorystoreRedisInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "redis.googleapis.com", MemorystoreRedisInstanceResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = redis.NewCloudRedisClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &redispb.ListInstancesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListInstances(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate memorystore redis instances")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &MemorystoreRedisInstance{
			svc:          l.svc,
			project:      opts.Project,
			region:       opts.Region,
			Name:         &name,
			FullName:     &resp.Name,
			Tier:         resp.Tier.String(),
			State:        resp.State.String(),
			RedisVersion: &resp.RedisVersion,
			Labels:       resp.Labels,
		})
	}

	return resources, nil
}

func (l *MemorystoreRedisInstanceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type MemorystoreRedisInstance struct {
	svc          *redis.CloudRedisClient
	removeOp     *redis.DeleteInstanceOperation
	project      *string
	region       *string
	Name         *string
	FullName     *string
	Tier         string
	State        string
	RedisVersion *string
	Labels       map[string]string `property:"tagPrefix=label"`
}

func (r *MemorystoreRedisInstance) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteInstance(ctx, &redispb.DeleteInstanceRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *MemorystoreRedisInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *MemorystoreRedisInstance) String() string {
	return *r.Name
}

func (r *MemorystoreRedisInstance) HandleWait(ctx context.Context) error {
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
