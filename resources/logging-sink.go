package resources

import (
	"context"
	"fmt"

	"github.com/gotidy/ptr"

	"google.golang.org/api/logging/v2"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const LoggingSinkResource = "LoggingSink"

func init() {
	registry.Register(&registry.Registration{
		Name:     LoggingSinkResource,
		Scope:    nuke.Project,
		Resource: &LoggingSink{},
		Lister:   &LoggingSinkLister{},
	})
}

type LoggingSinkLister struct {
	svc *logging.Service
}

func (l *LoggingSinkLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	var resources []resource.Resource

	if err := opts.BeforeList(nuke.Global, "logging.googleapis.com", LoggingSinkResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = logging.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	parent := fmt.Sprintf("projects/%s", *opts.Project)
	req := l.svc.Projects.Sinks.List(parent)
	if err := req.Pages(ctx, func(page *logging.ListSinksResponse) error {
		for _, sink := range page.Sinks {
			resources = append(resources, &LoggingSink{
				svc:         l.svc,
				project:     opts.Project,
				Name:        ptr.String(sink.Name),
				Destination: ptr.String(sink.Destination),
				LogFilter:   ptr.String(sink.Filter),
				Disabled:    ptr.Bool(sink.Disabled),
			})
		}
		return nil
	}); err != nil {
		return resources, err
	}

	return resources, nil
}

type LoggingSink struct {
	svc         *logging.Service
	project     *string
	Name        *string `description:"Name of the logging sink"`
	Destination *string `description:"Export destination of the sink"`
	LogFilter   *string `description:"Advanced logs filter applied to the sink" property:"name=Filter"`
	Disabled    *bool   `description:"Whether the sink is disabled"`
}

func (r *LoggingSink) Filter() error {
	// The _Required and _Default sinks are created by Cloud Logging and cannot be deleted.
	if *r.Name == "_Required" || *r.Name == "_Default" {
		return fmt.Errorf("cannot delete the %s logging sink", *r.Name)
	}
	return nil
}

func (r *LoggingSink) Remove(ctx context.Context) error {
	sinkName := fmt.Sprintf("projects/%s/sinks/%s", *r.project, *r.Name)
	_, err := r.svc.Projects.Sinks.Delete(sinkName).Context(ctx).Do()
	return err
}

func (r *LoggingSink) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *LoggingSink) String() string {
	return *r.Name
}
