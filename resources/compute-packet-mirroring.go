package resources

import (
	"context"
	"errors"

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

const ComputePacketMirroringResource = "ComputePacketMirroring"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputePacketMirroringResource,
		Scope:    nuke.Project,
		Resource: &ComputePacketMirroring{},
		Lister:   &ComputePacketMirroringLister{},
	})
}

type ComputePacketMirroringLister struct {
	svc *compute.PacketMirroringsClient
}

func (l *ComputePacketMirroringLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewPacketMirroringsRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListPacketMirroringsRequest{
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
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		resources = append(resources, &ComputePacketMirroring{
			svc:     l.svc,
			Name:    resp.Name,
			project: opts.Project,
			region:  opts.Region,
		})
	}

	return resources, nil
}

type ComputePacketMirroring struct {
	svc     *compute.PacketMirroringsClient
	project *string
	region  *string
	Name    *string `description:"Name of the packet mirroring configuration."`
}

func (r *ComputePacketMirroring) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeletePacketMirroringRequest{
		Project:         *r.project,
		Region:          *r.region,
		PacketMirroring: *r.Name,
	})
	return err
}

func (r *ComputePacketMirroring) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputePacketMirroring) String() string {
	return *r.Name
}
