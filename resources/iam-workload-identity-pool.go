package resources

import (
	"context"
	"fmt"
	"github.com/ekristen/gcp-nuke/pkg/nuke"
	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"
	"github.com/gotidy/ptr"
	"google.golang.org/api/iam/v1"
	"strings"
)

const IAMWorkloadIdentityPoolResource = "IAMWorkloadIdentityPool"

func init() {
	registry.Register(&registry.Registration{
		Name:   IAMWorkloadIdentityPoolResource,
		Scope:  nuke.Project,
		Lister: &IAMWorkloadIdentityPoolLister{},
	})
}

type IAMWorkloadIdentityPoolLister struct {
	svc *iam.Service
}

func (l *IAMWorkloadIdentityPoolLister) ListPools(ctx context.Context, opts *nuke.ListerOpts) ([]*iam.WorkloadIdentityPool, error) {
	if l.svc == nil {
		var err error
		l.svc, err = iam.NewService(ctx)
		if err != nil {
			return nil, err
		}
	}

	resourceName := fmt.Sprintf("projects/%s/locations/global", *opts.Project)
	var nextPageToken string

	var allPools []*iam.WorkloadIdentityPool

	for {
		call := l.svc.Projects.Locations.WorkloadIdentityPools.List(resourceName)
		if nextPageToken != "" {
			call.PageToken(nextPageToken)
		}

		resp, err := call.Context(ctx).Do()
		if err != nil {
			return nil, err
		}

		allPools = append(allPools, resp.WorkloadIdentityPools...)

		nextPageToken = resp.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	return allPools, nil
}

func (l *IAMWorkloadIdentityPoolLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

	workloadIdentityPools, err := l.ListPools(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, pool := range workloadIdentityPools {
		poolNameParts := strings.Split(pool.Name, "/")
		poolName := poolNameParts[len(poolNameParts)-1]
		resources = append(resources, &IAMWorkloadIdentityPool{
			svc:     l.svc,
			Project: opts.Project,
			Region:  opts.Region,
			Name:    ptr.String(poolName),
		})
	}

	return resources, nil
}

type IAMWorkloadIdentityPool struct {
	svc     *iam.Service
	Project *string
	Region  *string
	Name    *string
}

func (r *IAMWorkloadIdentityPool) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.WorkloadIdentityPools.Delete(*r.Name).Context(ctx).Do()
	return err
}

func (r *IAMWorkloadIdentityPool) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *IAMWorkloadIdentityPool) String() string {
	return *r.Name
}
