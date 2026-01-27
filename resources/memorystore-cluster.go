package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	cluster "cloud.google.com/go/redis/cluster/apiv1"
	"cloud.google.com/go/redis/cluster/apiv1/clusterpb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const MemorystoreClusterResource = "MemorystoreCluster"

func init() {
	registry.Register(&registry.Registration{
		Name:     MemorystoreClusterResource,
		Scope:    nuke.Project,
		Resource: &MemorystoreCluster{},
		Lister:   &MemorystoreClusterLister{},
	})
}

type MemorystoreClusterLister struct {
	svc *cluster.CloudRedisClusterClient
}

func (l *MemorystoreClusterLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "redis.googleapis.com", MemorystoreClusterResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = cluster.NewCloudRedisClusterClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &clusterpb.ListClustersRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListClusters(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate memorystore clusters")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &MemorystoreCluster{
			svc:        l.svc,
			project:    opts.Project,
			region:     opts.Region,
			Name:       &name,
			FullName:   &resp.Name,
			State:      resp.State.String(),
			ShardCount: resp.ShardCount,
		})
	}

	return resources, nil
}

func (l *MemorystoreClusterLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type MemorystoreCluster struct {
	svc        *cluster.CloudRedisClusterClient
	removeOp   *cluster.DeleteClusterOperation
	project    *string
	region     *string
	Name       *string
	FullName   *string
	State      string
	ShardCount *int32
}

func (r *MemorystoreCluster) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteCluster(ctx, &clusterpb.DeleteClusterRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *MemorystoreCluster) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *MemorystoreCluster) String() string {
	return *r.Name
}

func (r *MemorystoreCluster) HandleWait(ctx context.Context) error {
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
