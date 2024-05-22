package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotidy/ptr"
	"google.golang.org/genproto/googleapis/cloud/location"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/functions/apiv1"
	"cloud.google.com/go/functions/apiv1/functionspb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CloudFunctionResource = "CloudFunction"

func init() {
	registry.Register(&registry.Registration{
		Name:   CloudFunctionResource,
		Scope:  nuke.Project,
		Lister: &CloudFunctionLister{},
	})
}

type CloudFunctionLister struct {
	svc       *functions.CloudFunctionsClient
	locations []string
}

func (l *CloudFunctionLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *CloudFunctionLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = functions.NewCloudFunctionsRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}

		// TODO: determine locations, cache and skip locations not supported
		it := l.svc.ListLocations(ctx, &location.ListLocationsRequest{
			Name: fmt.Sprintf("projects/%s", *opts.Project),
		})
		for {
			resp, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return nil, err
			}

			l.locations = append(l.locations, resp.Name)
		}
	}

	parent := fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region)

	if !slices.Contains(l.locations, parent) {
		return nil, liberror.ErrSkipRequest(fmt.Sprintf("location %s not supported", *opts.Region))
	}

	req := &functionspb.ListFunctionsRequest{
		Parent: parent,
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

		resources = append(resources, &CloudFunction{
			svc:      l.svc,
			FullName: ptr.String(resp.Name),
			Name:     ptr.String(name),
			Project:  opts.Project,
			Region:   opts.Region,
			Labels:   resp.Labels,
			Status:   ptr.String(resp.Status.String()),
		})
	}

	return resources, nil
}

type CloudFunction struct {
	svc      *functions.CloudFunctionsClient
	removeOp *functions.DeleteFunctionOperation
	Project  *string
	Region   *string
	FullName *string `property:"-"`
	Name     *string `property:"Name"`
	Status   *string
	Labels   map[string]string
}

func (r *CloudFunction) Remove(ctx context.Context) (err error) {
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

func (r *CloudFunction) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CloudFunction) String() string {
	return *r.Name
}

func (r *CloudFunction) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.
			WithField("resource", CloudFunctionResource).
			WithError(err).
			Trace("remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return fmt.Errorf("operation still in progress")
	}

	return nil
}
