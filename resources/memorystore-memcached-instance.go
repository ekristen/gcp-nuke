package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	memcache "cloud.google.com/go/memcache/apiv1"
	"cloud.google.com/go/memcache/apiv1/memcachepb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const MemorystoreMemcachedInstanceResource = "MemorystoreMemcachedInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:     MemorystoreMemcachedInstanceResource,
		Scope:    nuke.Project,
		Resource: &MemorystoreMemcachedInstance{},
		Lister:   &MemorystoreMemcachedInstanceLister{},
	})
}

type MemorystoreMemcachedInstanceLister struct {
	svc *memcache.CloudMemcacheClient
}

func (l *MemorystoreMemcachedInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Regional, "memcache.googleapis.com"); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = memcache.NewCloudMemcacheClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &memcachepb.ListInstancesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListInstances(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate memorystore memcached instances")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &MemorystoreMemcachedInstance{
			svc:       l.svc,
			project:   opts.Project,
			region:    opts.Region,
			Name:      &name,
			FullName:  &resp.Name,
			State:     resp.State.String(),
			NodeCount: resp.NodeCount,
			Labels:    resp.Labels,
		})
	}

	return resources, nil
}

func (l *MemorystoreMemcachedInstanceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type MemorystoreMemcachedInstance struct {
	svc       *memcache.CloudMemcacheClient
	removeOp  *memcache.DeleteInstanceOperation
	project   *string
	region    *string
	Name      *string
	FullName  *string
	State     string
	NodeCount int32
	Labels    map[string]string `property:"tagPrefix=label"`
}

func (r *MemorystoreMemcachedInstance) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteInstance(ctx, &memcachepb.DeleteInstanceRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *MemorystoreMemcachedInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *MemorystoreMemcachedInstance) String() string {
	return *r.Name
}

func (r *MemorystoreMemcachedInstance) HandleWait(ctx context.Context) error {
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
