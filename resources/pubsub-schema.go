package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	pubsub "cloud.google.com/go/pubsub/v2/apiv1"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/iterator"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const PubSubSchemaResource = "PubSubSchema"

func init() {
	registry.Register(&registry.Registration{
		Name:      PubSubSchemaResource,
		Scope:     nuke.Project,
		Resource:  &PubSubSchema{},
		Lister:    &PubSubSchemaLister{},
		DependsOn: []string{PubSubTopicResource},
	})
}

type PubSubSchemaLister struct {
	svc *pubsub.SchemaClient
}

func (l *PubSubSchemaLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *PubSubSchemaLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "pubsub.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = pubsub.NewSchemaClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &pubsubpb.ListSchemasRequest{
		Parent: "projects/" + *opts.Project,
	}

	it := l.svc.ListSchemas(ctx, req)
	for {
		schema, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate pubsub schemas")
			break
		}

		nameParts := strings.Split(schema.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &PubSubSchema{
			svc:      l.svc,
			Project:  opts.Project,
			FullName: ptr.String(schema.Name),
			Name:     ptr.String(name),
			Type:     ptr.String(schema.Type.String()),
		})
	}

	return resources, nil
}

type PubSubSchema struct {
	svc      *pubsub.SchemaClient
	Project  *string
	FullName *string
	Name     *string `description:"The name of the Pub/Sub schema"`
	Type     *string `description:"The type of the schema (AVRO, PROTOCOL_BUFFER)"`
}

func (r *PubSubSchema) Remove(ctx context.Context) error {
	return r.svc.DeleteSchema(ctx, &pubsubpb.DeleteSchemaRequest{
		Name: *r.FullName,
	})
}

func (r *PubSubSchema) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *PubSubSchema) String() string {
	return *r.Name
}
