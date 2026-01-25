package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComputeNetworkEndpointGroupResource = "ComputeNetworkEndpointGroup"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeNetworkEndpointGroupResource,
		Scope:    nuke.Project,
		Resource: &ComputeNetworkEndpointGroup{},
		Lister:   &ComputeNetworkEndpointGroupLister{},
	})
}

type ComputeNetworkEndpointGroupLister struct {
	zonalSvc    *compute.NetworkEndpointGroupsClient
	regionalSvc *compute.RegionNetworkEndpointGroupsClient
	globalSvc   *compute.GlobalNetworkEndpointGroupsClient
}

func (l *ComputeNetworkEndpointGroupLister) Close() {
	if l.zonalSvc != nil {
		_ = l.zonalSvc.Close()
	}
	if l.regionalSvc != nil {
		_ = l.regionalSvc.Close()
	}
	if l.globalSvc != nil {
		_ = l.globalSvc.Close()
	}
}

func (l *ComputeNetworkEndpointGroupLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err == nil {
		globalResources, err := l.listGlobal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list global network endpoint groups")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err == nil {
		regionalResources, err := l.listRegional(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional network endpoint groups")
		} else {
			resources = append(resources, regionalResources...)
		}

		zonalResources, err := l.listZonal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list zonal network endpoint groups")
		} else {
			resources = append(resources, zonalResources...)
		}
	}

	return resources, nil
}

func (l *ComputeNetworkEndpointGroupLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.globalSvc == nil {
		var err error
		l.globalSvc, err = compute.NewGlobalNetworkEndpointGroupsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListGlobalNetworkEndpointGroupsRequest{
		Project: *opts.Project,
	}
	it := l.globalSvc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate global network endpoint groups")
			break
		}

		resources = append(resources, &ComputeNetworkEndpointGroup{
			globalSvc:   l.globalSvc,
			project:     opts.Project,
			Name:        resp.Name,
			NetworkType: resp.NetworkEndpointType,
			CreatedAt:   resp.CreationTimestamp,
		})
	}

	return resources, nil
}

func (l *ComputeNetworkEndpointGroupLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.regionalSvc == nil {
		var err error
		l.regionalSvc, err = compute.NewRegionNetworkEndpointGroupsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRegionNetworkEndpointGroupsRequest{
		Project: *opts.Project,
		Region:  *opts.Region,
	}
	it := l.regionalSvc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate regional network endpoint groups")
			break
		}

		resources = append(resources, &ComputeNetworkEndpointGroup{
			regionalSvc: l.regionalSvc,
			project:     opts.Project,
			region:      opts.Region,
			Name:        resp.Name,
			NetworkType: resp.NetworkEndpointType,
			CreatedAt:   resp.CreationTimestamp,
		})
	}

	return resources, nil
}

func (l *ComputeNetworkEndpointGroupLister) listZonal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.zonalSvc == nil {
		var err error
		l.zonalSvc, err = compute.NewNetworkEndpointGroupsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	for _, zone := range opts.Zones {
		req := &computepb.ListNetworkEndpointGroupsRequest{
			Project: *opts.Project,
			Zone:    zone,
		}
		it := l.zonalSvc.List(ctx, req)
		for {
			resp, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate zonal network endpoint groups")
				break
			}

			zoneCopy := zone
			resources = append(resources, &ComputeNetworkEndpointGroup{
				zonalSvc:    l.zonalSvc,
				project:     opts.Project,
				zone:        &zoneCopy,
				Name:        resp.Name,
				NetworkType: resp.NetworkEndpointType,
				CreatedAt:   resp.CreationTimestamp,
			})
		}
	}

	return resources, nil
}

type ComputeNetworkEndpointGroup struct {
	zonalSvc    *compute.NetworkEndpointGroupsClient
	regionalSvc *compute.RegionNetworkEndpointGroupsClient
	globalSvc   *compute.GlobalNetworkEndpointGroupsClient
	removeOp    *compute.Operation
	project     *string
	region      *string
	zone        *string
	Name        *string
	NetworkType *string
	CreatedAt   *string
}

func (r *ComputeNetworkEndpointGroup) Remove(ctx context.Context) error {
	if r.zonalSvc != nil {
		return r.removeZonal(ctx)
	} else if r.regionalSvc != nil {
		return r.removeRegional(ctx)
	} else if r.globalSvc != nil {
		return r.removeGlobal(ctx)
	}

	return errors.New("unable to determine service")
}

func (r *ComputeNetworkEndpointGroup) removeGlobal(ctx context.Context) (err error) {
	r.removeOp, err = r.globalSvc.Delete(ctx, &computepb.DeleteGlobalNetworkEndpointGroupRequest{
		Project:              *r.project,
		NetworkEndpointGroup: *r.Name,
	})
	return err
}

func (r *ComputeNetworkEndpointGroup) removeRegional(ctx context.Context) (err error) {
	r.removeOp, err = r.regionalSvc.Delete(ctx, &computepb.DeleteRegionNetworkEndpointGroupRequest{
		Project:              *r.project,
		Region:               *r.region,
		NetworkEndpointGroup: *r.Name,
	})
	return err
}

func (r *ComputeNetworkEndpointGroup) removeZonal(ctx context.Context) (err error) {
	r.removeOp, err = r.zonalSvc.Delete(ctx, &computepb.DeleteNetworkEndpointGroupRequest{
		Project:              *r.project,
		Zone:                 *r.zone,
		NetworkEndpointGroup: *r.Name,
	})
	return err
}

func (r *ComputeNetworkEndpointGroup) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeNetworkEndpointGroup) String() string {
	return *r.Name
}

func (r *ComputeNetworkEndpointGroup) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())
			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Errorf("unable to delete %s", ComputeNetworkEndpointGroupResource)
			return removeErr
		}
	}

	return nil
}
