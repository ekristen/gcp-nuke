package resources

import (
	"context"
	"errors"
	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const StorageBucketResource = "StorageBucket"

func init() {
	registry.Register(&registry.Registration{
		Name:   StorageBucketResource,
		Scope:  nuke.Project,
		Lister: &StorageBucketLister{},
	})
}

type StorageBucketLister struct {
	svc *storage.Client
}

func (l *StorageBucketLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *StorageBucketLister) ListBuckets(ctx context.Context, opts *nuke.ListerOpts) ([]*storage.BucketAttrs, error) {
	if l.svc == nil {
		var err error
		l.svc, err = storage.NewClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	var allBuckets []*storage.BucketAttrs

	it := l.svc.Buckets(ctx, *opts.Project)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		allBuckets = append(allBuckets, resp)
	}

	return allBuckets, nil
}

func (l *StorageBucketLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	buckets, err := l.ListBuckets(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, bucket := range buckets {
		if bucket.Location != *opts.Region {
			continue
		}

		resources = append(resources, &StorageBucket{
			svc:     l.svc,
			Name:    ptr.String(bucket.Name),
			Project: opts.Project,
			Labels:  bucket.Labels,
			Region:  ptr.String(bucket.Location),
		})
	}

	return resources, nil
}

type StorageBucket struct {
	svc     *storage.Client
	Project *string
	Region  *string
	Name    *string
	Labels  map[string]string
}

func (r *StorageBucket) Remove(ctx context.Context) error {
	return r.svc.Bucket(*r.Name).Delete(ctx)
}

func (r *StorageBucket) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *StorageBucket) String() string {
	return *r.Name
}
