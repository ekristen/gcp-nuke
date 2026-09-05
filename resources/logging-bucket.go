package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/logging/v2"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const LoggingBucketResource = "LoggingBucket"

func init() {
	registry.Register(&registry.Registration{
		Name:     LoggingBucketResource,
		Scope:    nuke.Project,
		Resource: &LoggingBucket{},
		Lister:   &LoggingBucketLister{},
		DependsOn: []string{
			LoggingSinkResource,
		},
	})
}

type LoggingBucketLister struct {
	svc *logging.Service
}

func (l *LoggingBucketLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	// Log buckets live under a location, which is either a region or the special "global"
	// location, so both geographies have to be walked.
	if err := opts.BeforeList(nuke.Global, "logging.googleapis.com", LoggingBucketResource); err == nil {
		globalResources, err := l.listLocation(ctx, opts, "global")
		if err != nil {
			logrus.WithError(err).Error("unable to list global")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "logging.googleapis.com", LoggingBucketResource); err == nil {
		regionalResources, err := l.listLocation(ctx, opts, *opts.Region)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *LoggingBucketLister) listLocation(ctx context.Context, opts *nuke.ListerOpts, location string) (
	[]resource.Resource, error,
) {
	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = logging.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	parent := fmt.Sprintf("projects/%s/locations/%s", *opts.Project, location)
	err := l.svc.Projects.Locations.Buckets.List(parent).Pages(ctx,
		func(page *logging.ListBucketsResponse) error {
			for _, bucket := range page.Buckets {
				// Name is the full resource name, e.g.
				// projects/my-project/locations/global/buckets/_Default
				nameParts := strings.Split(bucket.Name, "/")

				resources = append(resources, &LoggingBucket{
					svc:            l.svc,
					fullName:       ptr.String(bucket.Name),
					Name:           ptr.String(nameParts[len(nameParts)-1]),
					Location:       ptr.String(location),
					Description:    ptr.String(bucket.Description),
					RetentionDays:  ptr.Int64(bucket.RetentionDays),
					Locked:         ptr.Bool(bucket.Locked),
					LifecycleState: ptr.String(bucket.LifecycleState),
				})
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	return resources, nil
}

type LoggingBucket struct {
	svc            *logging.Service
	fullName       *string
	Name           *string `description:"Name of the log bucket"`
	Location       *string `description:"Location the log bucket resides in"`
	Description    *string `description:"Description of the log bucket"`
	RetentionDays  *int64  `description:"Number of days log entries are retained"`
	Locked         *bool   `description:"Whether the retention period is locked"`
	LifecycleState *string `description:"Lifecycle state of the log bucket"`
}

func (r *LoggingBucket) Filter() error {
	// _Required and _Default are created by Cloud Logging and cannot be deleted.
	if *r.Name == "_Required" || *r.Name == "_Default" {
		return fmt.Errorf("cannot delete the %s log bucket", *r.Name)
	}

	// A locked bucket cannot be deleted until its retention period expires.
	if *r.Locked {
		return fmt.Errorf("log bucket retention is locked")
	}

	if *r.LifecycleState == "DELETE_REQUESTED" {
		return fmt.Errorf("log bucket is already pending deletion")
	}

	return nil
}

// Remove marks the bucket for deletion. Cloud Logging moves it to DELETE_REQUESTED and purges it
// after 7 days; there is no way to purge it sooner.
func (r *LoggingBucket) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.Buckets.Delete(*r.fullName).Context(ctx).Do()
	return err
}

func (r *LoggingBucket) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *LoggingBucket) String() string {
	return *r.Name
}
