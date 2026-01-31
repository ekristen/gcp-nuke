package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"

	"cloud.google.com/go/storage"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/settings"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const StorageBucketResource = "StorageBucket"

func init() {
	registry.Register(&registry.Registration{
		Name:     StorageBucketResource,
		Scope:    nuke.Project,
		Resource: &StorageBucket{},
		Lister: &StorageBucketLister{
			multiRegion: make(map[string]string),
		},
		Settings: []string{
			"DeleteGoogleManagedBuckets",
			"DisableDeletionProtection",
		},
	})
}

type StorageBucketLister struct {
	svc         *storage.Client
	multiRegion map[string]string
}

func (l *StorageBucketLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
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
	if err := opts.BeforeList(nuke.Regional, "storage.googleapis.com", StorageBucketResource); err != nil {
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

		isMultiRegion := bucket.LocationType == "multi-region" || bucket.LocationType == "dual-region"
		isAccountedFor := false
		if isMultiRegion {
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
			svc:                       l.svc,
			disableDeletionProtection: opts.DisableDeletionProtection,
			project:                   opts.Project,
			region:                    ptr.String(loc),
			Name:                      ptr.String(bucket.Name),
			Labels:                    bucket.Labels,
			MultiRegion:               ptr.Bool(isMultiRegion),
		})
	}

	return resources, nil
}

type StorageBucket struct {
	svc                       *storage.Client
	settings                  *settings.Setting
	disableDeletionProtection bool
	project                   *string
	region                    *string
	Name                      *string
	Labels                    map[string]string `property:"tagPrefix=label"`
	MultiRegion               *bool
}

func (r *StorageBucket) Filter() error {
	managedByCloudFunctions := false
	managedByWho := ""

	if r.Labels != nil {
		if v, ok := r.Labels["goog-managed-by"]; ok {
			managedByCloudFunctions = true
			managedByWho = v
		}
	}

	if managedByCloudFunctions && !r.settings.GetBool("DeleteGoogleManagedBuckets") {
		return fmt.Errorf("bucket is managed by %s", managedByWho)
	}

	return nil
}

func (r *StorageBucket) Remove(ctx context.Context) error {
	if r.settings.GetBool("DisableDeletionProtection") || r.disableDeletionProtection {
		if _, err := r.svc.Bucket(*r.Name).Update(ctx, storage.BucketAttrsToUpdate{
			RetentionPolicy: &storage.RetentionPolicy{
				RetentionPeriod: 0,
				EffectiveTime:   time.Now(),
				IsLocked:        false,
			},
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

type objectToDelete struct {
	name       string
	generation int64
}

func (r *StorageBucket) removeObjects(ctx context.Context) error {
	const maxConcurrency = 500

	var objects []objectToDelete
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
		objects = append(objects, objectToDelete{
			name:       resp.Name,
			generation: resp.Generation,
		})
	}

	softDeletedIt := r.svc.Bucket(*r.Name).Objects(ctx, &storage.Query{
		SoftDeleted: true,
	})
	for {
		resp, err := softDeletedIt.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			if strings.Contains(err.Error(), "Soft delete policy is required") {
				break
			}
			return err
		}
		objects = append(objects, objectToDelete{
			name:       resp.Name,
			generation: resp.Generation,
		})
	}

	if len(objects) == 0 {
		return nil
	}

	logrus.Debugf("deleting %d objects from bucket %s", len(objects), *r.Name)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var deletedCount int64
	var mu sync.Mutex

	for _, obj := range objects {
		obj := obj
		g.Go(func() error {
			if err := r.svc.Bucket(*r.Name).Object(obj.name).Generation(obj.generation).Delete(ctx); err != nil {
				if !errors.Is(err, storage.ErrObjectNotExist) {
					return err
				}
			}
			mu.Lock()
			deletedCount++
			if deletedCount%100 == 0 {
				logrus.Debugf("deleted %d/%d objects from bucket %s", deletedCount, len(objects), *r.Name)
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	logrus.Debugf("finished deleting %d objects from bucket %s", len(objects), *r.Name)
	return nil
}
