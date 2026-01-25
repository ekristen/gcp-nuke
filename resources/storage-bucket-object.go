package resources

import (
	"context"
	"errors"

	"github.com/gotidy/ptr"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const StorageBucketObjectResource = "StorageBucketObject"

func init() {
	registry.Register(&registry.Registration{
		Name:     StorageBucketObjectResource,
		Scope:    nuke.Project,
		Resource: &StorageBucketObject{},
		Lister:   &StorageBucketObjectLister{},
	})
}

type StorageBucketObjectLister struct {
	svc *storage.Client
}

func (l *StorageBucketObjectLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *StorageBucketObjectLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "storage.googleapis.com"); err != nil {
		return resources, err
	}

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
		it := l.svc.Bucket(bucket.Name).Objects(ctx, &storage.Query{
			Versions: true,
		})
		for {
			resp, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate objects")
				break
			}

			resources = append(resources, &StorageBucketObject{
				svc:        l.svc,
				Name:       ptr.String(resp.Name),
				Bucket:     ptr.String(bucket.Name),
				Project:    opts.Project,
				Generation: ptr.Int64(resp.Generation),
				Metadata:   resp.Metadata,
			})
		}
	}

	return resources, nil
}

type StorageBucketObject struct {
	svc        *storage.Client
	Project    *string
	Region     *string
	Name       *string
	Bucket     *string
	Generation *int64
	Metadata   map[string]string
}

func (r *StorageBucketObject) Remove(ctx context.Context) error {
	obj := r.svc.Bucket(*r.Bucket).Object(*r.Name)
	if r.Generation != nil {
		obj = obj.Generation(*r.Generation)
	}
	return obj.Delete(ctx)
}

func (r *StorageBucketObject) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *StorageBucketObject) String() string {
	return *r.Name
}
