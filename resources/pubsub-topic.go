package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/iterator"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const PubSubTopicResource = "PubSubTopic"

func init() {
	registry.Register(&registry.Registration{
		Name:      PubSubTopicResource,
		Scope:     nuke.Project,
		Resource:  &PubSubTopic{},
		Lister:    &PubSubTopicLister{},
		DependsOn: []string{PubSubSubscriptionResource},
	})
}

type PubSubTopicLister struct {
	svc *pubsub.Client
}

func (l *PubSubTopicLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *PubSubTopicLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "pubsub.googleapis.com", PubSubTopicResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = pubsub.NewClient(ctx, *opts.Project, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &pubsubpb.ListTopicsRequest{
		Project: "projects/" + *opts.Project,
	}

	it := l.svc.TopicAdminClient.ListTopics(ctx, req)
	for {
		topic, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate pubsub topics")
			break
		}

		nameParts := strings.Split(topic.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &PubSubTopic{
			svc:      l.svc,
			Project:  opts.Project,
			FullName: ptr.String(topic.Name),
			Name:     ptr.String(name),
			Labels:   topic.Labels,
		})
	}

	return resources, nil
}

type PubSubTopic struct {
	svc      *pubsub.Client
	Project  *string
	FullName *string
	Name     *string           `description:"The name of the Pub/Sub topic"`
	Labels   map[string]string `property:"tagPrefix=label" description:"Labels associated with the topic"`
}

func (r *PubSubTopic) Remove(ctx context.Context) error {
	return r.svc.TopicAdminClient.DeleteTopic(ctx, &pubsubpb.DeleteTopicRequest{
		Topic: *r.FullName,
	})
}

func (r *PubSubTopic) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *PubSubTopic) String() string {
	return *r.Name
}
