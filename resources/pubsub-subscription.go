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

const PubSubSubscriptionResource = "PubSubSubscription"

func init() {
	registry.Register(&registry.Registration{
		Name:     PubSubSubscriptionResource,
		Scope:    nuke.Project,
		Resource: &PubSubSubscription{},
		Lister:   &PubSubSubscriptionLister{},
	})
}

type PubSubSubscriptionLister struct {
	svc *pubsub.Client
}

func (l *PubSubSubscriptionLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *PubSubSubscriptionLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "pubsub.googleapis.com", PubSubSubscriptionResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = pubsub.NewClient(ctx, *opts.Project, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &pubsubpb.ListSubscriptionsRequest{
		Project: "projects/" + *opts.Project,
	}

	it := l.svc.SubscriptionAdminClient.ListSubscriptions(ctx, req)
	for {
		sub, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate pubsub subscriptions")
			break
		}

		nameParts := strings.Split(sub.Name, "/")
		name := nameParts[len(nameParts)-1]

		var topicName string
		if sub.Topic != "" {
			topicParts := strings.Split(sub.Topic, "/")
			topicName = topicParts[len(topicParts)-1]
		}

		resources = append(resources, &PubSubSubscription{
			svc:      l.svc,
			Project:  opts.Project,
			FullName: ptr.String(sub.Name),
			Name:     ptr.String(name),
			Topic:    ptr.String(topicName),
			Labels:   sub.Labels,
		})
	}

	return resources, nil
}

type PubSubSubscription struct {
	svc      *pubsub.Client
	Project  *string
	FullName *string
	Name     *string           `description:"The name of the Pub/Sub subscription"`
	Topic    *string           `description:"The topic this subscription is attached to"`
	Labels   map[string]string `property:"tagPrefix=label" description:"Labels associated with the subscription"`
}

func (r *PubSubSubscription) Remove(ctx context.Context) error {
	return r.svc.SubscriptionAdminClient.DeleteSubscription(ctx, &pubsubpb.DeleteSubscriptionRequest{
		Subscription: *r.FullName,
	})
}

func (r *PubSubSubscription) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *PubSubSubscription) String() string {
	return *r.Name
}
