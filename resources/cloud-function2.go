package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotidy/ptr"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/functions/apiv2"
	"cloud.google.com/go/functions/apiv2/functionspb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudFunction2Resource = "CloudFunction2"

func init() {
	registry.Register(&registry.Registration{
		Name:   CloudFunction2Resource,
		Scope:  nuke.Project,
		Lister: &CloudFunction2Lister{},
	})
}

type CloudFunction2Lister struct {
	svc *functions.FunctionClient
}

func (l *CloudFunction2Lister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *CloudFunction2Lister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "cloudfunctions.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = functions.NewFunctionRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}

		// TODO: determine locations, cache and skip locations not supported
	}

	req := &functionspb.ListFunctionsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListFunctions(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate cloud functions")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &CloudFunction2{
			svc:      l.svc,
			FullName: ptr.String(resp.Name),
			Name:     ptr.String(name),
			Project:  opts.Project,
			Region:   opts.Region,
			Labels:   resp.Labels,
			State:    ptr.String(resp.State.String()),
		})
	}

	return resources, nil
}

type CloudFunction2 struct {
	svc      *functions.FunctionClient
	removeOp *functions.DeleteFunctionOperation
	Project  *string
	Region   *string
	FullName *string `property:"-"`
	Name     *string `property:"Name"`
	Labels   map[string]string
	State    *string
}

func (r *CloudFunction2) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteFunction(ctx, &functionspb.DeleteFunctionRequest{
		Name: *r.FullName,
	})
	if err != nil && strings.Contains(err.Error(), "proto") && strings.Contains(err.Error(), "missing") {
		err = nil
	}
	if err != nil {
		logrus.WithError(err).Debug("error encountered on remove")
	}
	return err
}

func (r *CloudFunction2) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudFunction2) String() string {
	return *r.Name
}

func (r *CloudFunction2) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		if strings.Contains(err.Error(), "proto") && strings.Contains(err.Error(), "missing") {
			err = nil
		}
		if err != nil {
			logrus.
				WithField("resource", CloudFunction2Resource).
				WithError(err).
				Trace("remove op polling encountered error")
			return err
		}
	}

	if !r.removeOp.Done() {
		return fmt.Errorf("operation still in progress")
	}

	return nil
}
