package resources

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
)

const ZonalNetworkEndpointGroupResource = "ZonalNetworkEndpointGroup"

func init() {
	registry.Register(&registry.Registration{
		Name:   ZonalNetworkEndpointGroupResource,
		Scope:  nuke.Project,
		Lister: &ZonalNetworkEndpointGroupLister{},
	})
}

type ZonalNetworkEndpointGroupLister struct {
	svc *compute.NetworkEndpointGroupsClient
}

func (l *ZonalNetworkEndpointGroupLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *ZonalNetworkEndpointGroupLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewNetworkEndpointGroupsRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.AggregatedListNetworkEndpointGroupsRequest{
		Project: *opts.Project,
	}

	it := l.svc.AggregatedList(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate network endpoint groups")
			break
		}

		// resp is a NetworkEndpointGroupsScopedListPair which has Key (string) and Value (*computepb.NetworkEndpointGroupsScopedList)
		if resp.Value.NetworkEndpointGroups == nil {
			continue
		}

		for _, neg := range resp.Value.NetworkEndpointGroups {
			zoneName := path.Base(resp.Key) // Extract zone name from the Key
			resources = append(resources, &ZonalNetworkEndpointGroup{
				svc:          l.svc,
				project:      opts.Project,
				zone:         &zoneName,
				Name:         neg.Name,                // Assign directly since neg.Name is already *string
				negType:      neg.NetworkEndpointType, // neg.NetworkEndpointType is already *string
				creationDate: neg.CreationTimestamp,   // neg.CreationTimestamp is already *string
			})
		}
	}

	return resources, nil
}

type ZonalNetworkEndpointGroup struct {
	svc          *compute.NetworkEndpointGroupsClient
	removeOp     *compute.Operation
	project      *string
	zone         *string
	Name         *string
	negType      *string
	creationDate *string
}

func (r *ZonalNetworkEndpointGroup) Remove(ctx context.Context) error {
	var err error
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteNetworkEndpointGroupRequest{
		Project:              *r.project,
		Zone:                 *r.zone,
		NetworkEndpointGroup: *r.Name,
	})
	if err != nil {
		return err
	}

	return r.HandleWait(ctx)
}

func (r *ZonalNetworkEndpointGroup) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ZonalNetworkEndpointGroup) String() string {
	return *r.Name
}

// HandleWait is a hook into the libnuke resource lifecycle to allow for waiting on a resource to be removed.
func (r *ZonalNetworkEndpointGroup) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("network endpoint group remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.Done() && r.removeOp.Proto().GetError() != nil {
		removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
		logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Error("unable to delete zonal network endpoint group")
		return removeErr
	}

	return nil
}
