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

const StorageBucketObjectResource = "StorageBucketObject"

func init() {
	registry.Register(&registry.Registration{
		Name:   StorageBucketObjectResource,
		Scope:  nuke.Project,
		Lister: &StorageBucketObjectLister{},
	})
}

type StorageBucketObjectLister struct {
	svc *storage.Client
}

func (l *StorageBucketObjectLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = storage.NewClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	bucketLister := &StorageBucketLister{}
	buckets, err := bucketLister.ListBuckets(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, bucket := range buckets {
		it := l.svc.Bucket(bucket.Name).Objects(ctx, nil)
		for {
			resp, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate networks")
				break
			}

			resources = append(resources, &StorageBucketObject{
				svc:      l.svc,
				Name:     ptr.String(resp.Name),
				Bucket:   ptr.String(bucket.Name),
				Project:  opts.Project,
				Metadata: resp.Metadata,
			})
		}
	}

	return resources, nil
}

type StorageBucketObject struct {
	svc      *storage.Client
	Project  *string
	Region   *string
	Name     *string
	Bucket   *string
	Metadata map[string]string
}

func (r *StorageBucketObject) Remove(ctx context.Context) error {
	return r.svc.Bucket(*r.Bucket).Object(*r.Name).Delete(ctx)
}

func (r *StorageBucketObject) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *StorageBucketObject) String() string {
	return *r.Name
}
