package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	networkconnectivity "cloud.google.com/go/networkconnectivity/apiv1"
	"cloud.google.com/go/networkconnectivity/apiv1/networkconnectivitypb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ServiceConnectionPolicyResource = "ServiceConnectionPolicy"

func init() {
	registry.Register(&registry.Registration{
		Name:     ServiceConnectionPolicyResource,
		Scope:    nuke.Project,
		Resource: &ServiceConnectionPolicy{},
		Lister:   &ServiceConnectionPolicyLister{},
	})
}

type ServiceConnectionPolicyLister struct {
	svc *networkconnectivity.CrossNetworkAutomationClient
}

func (l *ServiceConnectionPolicyLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *ServiceConnectionPolicyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "networkconnectivity.googleapis.com", ServiceConnectionPolicyResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = networkconnectivity.NewCrossNetworkAutomationClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &networkconnectivitypb.ListServiceConnectionPoliciesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListServiceConnectionPolicies(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate service connection policies")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &ServiceConnectionPolicy{
			svc:          l.svc,
			FullName:     ptr.String(resp.Name),
			Name:         ptr.String(name),
			Project:      opts.Project,
			Region:       opts.Region,
			ServiceClass: ptr.String(resp.ServiceClass),
			Network:      ptr.String(resp.Network),
			Labels:       resp.Labels,
		})
	}

	return resources, nil
}

type ServiceConnectionPolicy struct {
	svc          *networkconnectivity.CrossNetworkAutomationClient
	removeOp     *networkconnectivity.DeleteServiceConnectionPolicyOperation
	Project      *string
	Region       *string
	FullName     *string
	Name         *string           `description:"The name of the service connection policy"`
	ServiceClass *string           `description:"The service class (e.g., gcp-cloud-sql, gcp-memorystore-redis)"`
	Network      *string           `description:"The network this policy applies to"`
	Labels       map[string]string `property:"tagPrefix=label"`
}

func (r *ServiceConnectionPolicy) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteServiceConnectionPolicy(ctx, &networkconnectivitypb.DeleteServiceConnectionPolicyRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *ServiceConnectionPolicy) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ServiceConnectionPolicy) String() string {
	return *r.Name
}

func (r *ServiceConnectionPolicy) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("service connection policy remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
