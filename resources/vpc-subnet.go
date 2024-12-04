package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const VPCSubnetResource = "VPCSubnet"

func init() {
	registry.Register(&registry.Registration{
		Name:     VPCSubnetResource,
		Scope:    nuke.Project,
		Resource: &VPCSubnet{},
		Lister:   &VPCSubnetLister{},
	})
}

type VPCSubnetLister struct {
	svc *compute.SubnetworksClient
}

func (l *VPCSubnetLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *VPCSubnetLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewSubnetworksRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListSubnetworksRequest{
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

		networkParts := strings.Split(resp.GetNetwork(), "/")
		networkName := networkParts[len(networkParts)-1]

		// TODO: query network to determine if auto-subnet
		resources = append(resources, &VPCSubnet{
			svc:       l.svc,
			Name:      resp.Name,
			project:   opts.Project,
			region:    opts.Region,
			Network:   ptr.String(networkName),
			IPV4Range: resp.IpCidrRange,
			IPV6Range: resp.Ipv6CidrRange,
		})
	}

	return resources, nil
}

type VPCSubnet struct {
	svc       *compute.SubnetworksClient
	project   *string
	region    *string
	Name      *string
	Network   *string
	IPV4Range *string
	IPV6Range *string
}

func (r *VPCSubnet) Filter() error {
	if *r.Name == "default" && strings.HasSuffix(*r.IPV4Range, "/20") {
		return fmt.Errorf("cannot remove default auto-subnet")
	}
	return nil
}

func (r *VPCSubnet) Remove(ctx context.Context) error {
	_, err := r.svc.Delete(ctx, &computepb.DeleteSubnetworkRequest{
		Project:    *r.project,
		Region:     *r.region,
		Subnetwork: *r.Name,
	})
	return err
}

func (r *VPCSubnet) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VPCSubnet) String() string {
	return *r.Name
}
