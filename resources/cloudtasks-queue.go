package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/api/iterator"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudTasksQueueResource = "CloudTasksQueue"

func init() {
	registry.Register(&registry.Registration{
		Name:     CloudTasksQueueResource,
		Scope:    nuke.Project,
		Resource: &CloudTasksQueue{},
		Lister:   &CloudTasksQueueLister{},
	})
}

type CloudTasksQueueLister struct {
	svc *cloudtasks.Client
}

func (l *CloudTasksQueueLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *CloudTasksQueueLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "cloudtasks.googleapis.com", CloudTasksQueueResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = cloudtasks.NewClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &cloudtaskspb.ListQueuesRequest{
		Parent: "projects/" + *opts.Project + "/locations/" + *opts.Region,
	}

	it := l.svc.ListQueues(ctx, req)
	for {
		queue, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate cloud tasks queues")
			break
		}

		nameParts := strings.Split(queue.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &CloudTasksQueue{
			svc:      l.svc,
			Project:  opts.Project,
			Region:   opts.Region,
			FullName: ptr.String(queue.Name),
			Name:     ptr.String(name),
			State:    ptr.String(queue.State.String()),
		})
	}

	return resources, nil
}

type CloudTasksQueue struct {
	svc      *cloudtasks.Client
	Project  *string
	Region   *string
	FullName *string
	Name     *string `description:"The name of the Cloud Tasks queue"`
	State    *string `description:"The current state of the queue"`
}

func (r *CloudTasksQueue) Remove(ctx context.Context) error {
	return r.svc.DeleteQueue(ctx, &cloudtaskspb.DeleteQueueRequest{
		Name: *r.FullName,
	})
}

func (r *CloudTasksQueue) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudTasksQueue) String() string {
	return *r.Name
}
