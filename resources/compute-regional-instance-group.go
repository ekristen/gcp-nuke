package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComputeRegionalInstanceGroupResource = "ComputeRegionalInstanceGroup"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeRegionalInstanceGroupResource,
		Scope:    nuke.Project,
		Resource: &ComputeRegionalInstanceGroup{},
		Lister:   &ComputeRegionalInstanceGroupLister{},
		DependsOn: []string{
			ComputeInstanceResource,
		},
	})
}

type ComputeRegionalInstanceGroupLister struct {
	svc *compute.RegionInstanceGroupManagersClient
}

func (l *ComputeRegionalInstanceGroupLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	// List regional instance groups
	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRegionInstanceGroupManagersRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			logrus.WithError(err).Error("failed to create regional instance group managers client")
			return nil, fmt.Errorf("failed to create regional instance group managers client: %v", err)
		}
	}

	req := &computepb.ListRegionInstanceGroupManagersRequest{
		Project: *opts.Project,
		Region:  *opts.Region,
	}

	it := l.svc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).WithField("region", *opts.Region).Error("unable to iterate regional compute instance groups")
			break
		}

		resources = append(resources, &ComputeRegionalInstanceGroup{
			svc:               l.svc,
			Name:              resp.Name,
			Project:           opts.Project,
			Region:            opts.Region,
			CreationTimestamp: resp.CreationTimestamp,
		})
	}

	return resources, nil
}

type ComputeRegionalInstanceGroup struct {
	svc               *compute.RegionInstanceGroupManagersClient
	Project           *string
	Name              *string
	Region            *string
	CreationTimestamp *string
}

func (r *ComputeRegionalInstanceGroup) Remove(ctx context.Context) error {
	// Regional instance group
	_, err := r.svc.Delete(ctx, &computepb.DeleteRegionInstanceGroupManagerRequest{
		Project:              *r.Project,
		Region:               *r.Region,
		InstanceGroupManager: *r.Name,
	})
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"project": *r.Project,
			"region":  *r.Region,
			"name":    *r.Name,
		}).Error("failed to delete regional instance group")
		return fmt.Errorf("failed to delete regional instance group: %v", err)
	}
	return nil
}

func (r *ComputeRegionalInstanceGroup) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeRegionalInstanceGroup) String() string {
	return *r.Name
}
