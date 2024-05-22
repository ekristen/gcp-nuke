package resources

import (
	"context"
	"errors"
	"github.com/gotidy/ptr"
	"strings"

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

const ComputeDiskResource = "ComputeDisk"

func init() {
	registry.Register(&registry.Registration{
		Name:   ComputeDiskResource,
		Scope:  nuke.Project,
		Lister: &ComputeDiskLister{},
	})
}

type ComputeDiskLister struct {
	svc *compute.DisksClient
}

func (l *ComputeDiskLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewDisksRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	for _, zone := range opts.Zones {
		req := &computepb.ListDisksRequest{
			Project: *opts.Project,
			Zone:    zone,
		}

		it := l.svc.List(ctx, req)

		for {
			resp, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate compute disks")
				break
			}

			typeParts := strings.Split(resp.GetType(), "/")
			typeName := typeParts[len(typeParts)-1]

			resources = append(resources, &ComputeDisk{
				svc:     l.svc,
				project: opts.Project,
				region:  opts.Region,
				Name:    resp.Name,
				Zone:    ptr.String(zone),
				Arch:    resp.Architecture,
				Size:    resp.SizeGb,
				Type:    ptr.String(typeName),
				Labels:  resp.Labels,
			})
		}
	}

	return resources, nil
}

type ComputeDisk struct {
	svc     *compute.DisksClient
	project *string
	region  *string
	Name    *string
	Zone    *string
	Arch    *string
	Size    *int64
	Type    *string
	Labels  map[string]string
}

func (r *ComputeDisk) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteDiskRequest{
		Project: *r.project,
		Zone:    *r.Zone,
		Disk:    *r.Name,
	})
	return err
}

func (r *ComputeDisk) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeDisk) String() string {
	return *r.Name
}
