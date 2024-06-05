package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/ekristen/libnuke/pkg/settings"
	"strings"
	"time"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const StorageBucketResource = "StorageBucket"

func init() {
	registry.Register(&registry.Registration{
		Name:  StorageBucketResource,
		Scope: nuke.Project,
		Lister: &StorageBucketLister{
			multiRegion: make(map[string]string),
		},
		Settings: []string{
			"DeleteGoogleManagedBuckets",
		},
	})
}

type StorageBucketLister struct {
	svc         *storage.Client
	multiRegion map[string]string
}

func (l *StorageBucketLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *StorageBucketLister) ListBuckets(ctx context.Context, opts *nuke.ListerOpts) ([]*storage.BucketAttrs, error) {
	if l.svc == nil {
		var err error
		l.svc, err = storage.NewClient(ctx, opts.ClientOptions...)
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
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "storage.googleapis.com"); err != nil {
		return resources, err
	}

	buckets, err := l.ListBuckets(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, bucket := range buckets {
		loc := strings.ToLower(bucket.Location)
		logrus.WithFields(logrus.Fields{
			"bucket":   bucket.Name,
			"location": loc,
			"region":   *opts.Region,
		}).Debug("bucket details")

		isMultiRegion := false
		isAccountedFor := false
		if bucket.Location == "US" {
			isMultiRegion = true
			if _, ok := l.multiRegion[bucket.Name]; !ok {
				l.multiRegion[bucket.Name] = loc
			} else {
				isAccountedFor = true
			}
		}

		if !isMultiRegion && loc != *opts.Region {
			continue
		}

		if isMultiRegion && isAccountedFor {
			continue
		}

		resources = append(resources, &StorageBucket{
			svc:         l.svc,
			project:     opts.Project,
			region:      ptr.String(loc),
			Name:        ptr.String(bucket.Name),
			Labels:      bucket.Labels,
			MultiRegion: ptr.Bool(isMultiRegion),
		})
	}

	return resources, nil
}

type StorageBucket struct {
	svc         *storage.Client
	settings    *settings.Setting
	project     *string
	region      *string
	Name        *string
	Labels      map[string]string `property:"tagPrefix=label"`
	MultiRegion *bool
}

func (r *StorageBucket) Filter() error {
	deleteGoogleManagedBuckets := false
	managedByCloudFunctions := false
	managedByWho := ""

	if r.settings != nil {
		deleteGoogleManagedBuckets = r.settings.Get("DeleteGoogleManagedBuckets").(bool)
	}
	if r.Labels != nil {
		if v, ok := r.Labels["goog-managed-by"]; ok {
			managedByCloudFunctions = true
			managedByWho = v
		}
	}

	if managedByCloudFunctions && !deleteGoogleManagedBuckets {
		return fmt.Errorf("bucket is managed by %s", managedByWho)
	}

	return nil
}

func (r *StorageBucket) Remove(ctx context.Context) error {
	if _, err := r.svc.Bucket(*r.Name).Update(ctx, storage.BucketAttrsToUpdate{
		RetentionPolicy: nil,
		SoftDeletePolicy: &storage.SoftDeletePolicy{
			EffectiveTime:     time.Now(),
			RetentionDuration: 0,
		},
		Lifecycle:         &storage.Lifecycle{Rules: nil},
		VersioningEnabled: false,
	}); err != nil {
		logrus.WithError(err).Error("encountered error while updating bucket attrs")
		return err
	}

	if err := r.removeObjects(ctx); err != nil {
		logrus.WithError(err).Error("encountered error while emptying bucket")
		return err
	}

	err := r.svc.Bucket(*r.Name).Delete(ctx)
	if err != nil {
		logrus.WithError(err).Error("encountered error while removing bucket")
	}
	return err
}

func (r *StorageBucket) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *StorageBucket) String() string {
	return *r.Name
}

func (r *StorageBucket) Settings(settings *settings.Setting) {
	r.settings = settings
}

func (r *StorageBucket) removeObjects(ctx context.Context) error {
	it := r.svc.Bucket(*r.Name).Objects(ctx, &storage.Query{
		Versions: true,
	})
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return err
		}

		logrus.Debug("deleting object: ", resp.Name)
		if err := r.svc.Bucket(*r.Name).Object(resp.Name).Generation(resp.Generation).Delete(ctx); err != nil {
			return err
		}
	}

	return nil
}
