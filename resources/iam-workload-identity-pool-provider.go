package resources

import (
	"context"
	"github.com/ekristen/gcp-nuke/pkg/nuke"
	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"
	"github.com/gotidy/ptr"
	"google.golang.org/api/iam/v1"
	"strings"
)

const IAMWorkloadIdentityPoolProviderProviderResource = "IAMWorkloadIdentityPoolProvider"

func init() {
	registry.Register(&registry.Registration{
		Name:   IAMWorkloadIdentityPoolProviderProviderResource,
		Scope:  nuke.Project,
		Lister: &IAMWorkloadIdentityPoolProviderLister{},
	})
}

type IAMWorkloadIdentityPoolProviderLister struct {
	svc *iam.Service
}

func (l *IAMWorkloadIdentityPoolProviderLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = iam.NewService(ctx)
		if err != nil {
			return nil, err
		}
	}

	workloadIdentityPoolLister := &IAMWorkloadIdentityPoolLister{}
	workloadIdentityPools, err := workloadIdentityPoolLister.ListPools(ctx, opts)
	if err != nil {
		return nil, err
	}

	// NOTE: you might have to modify the code below to actually work, this currently does not
	// inspect the aws sdk instead is a jumping off point
	for _, workloadIdentityPool := range workloadIdentityPools {
		var nextPageToken string

		for {
			call := l.svc.Projects.Locations.WorkloadIdentityPools.Providers.List(workloadIdentityPool.Name)
			if nextPageToken != "" {
				call.PageToken(nextPageToken)
			}

			resp, err := call.Context(ctx).Do()
			if err != nil {
				return nil, err
			}

			for _, provider := range resp.WorkloadIdentityPoolProviders {
				providerNameParts := strings.Split(provider.Name, "/")
				providerName := providerNameParts[len(providerNameParts)-1]

				poolNameParts := strings.Split(workloadIdentityPool.Name, "/")
				poolName := poolNameParts[len(poolNameParts)-1]

				resources = append(resources, &IAMWorkloadIdentityPoolProvider{
					svc:         l.svc,
					Project:     opts.Project,
					Region:      opts.Region,
					Name:        ptr.String(providerName),
					Pool:        ptr.String(poolName),
					Disabled:    ptr.Bool(provider.Disabled),
					DisplayName: ptr.String(provider.DisplayName),
					ExpireTime:  ptr.String(provider.ExpireTime),
				})
			}

			nextPageToken = resp.NextPageToken
			if nextPageToken == "" {
				break
			}
		}
	}

	return resources, nil
}

type IAMWorkloadIdentityPoolProvider struct {
	svc         *iam.Service
	Project     *string
	Region      *string
	Name        *string
	Pool        *string
	Disabled    *bool
	DisplayName *string
	ExpireTime  *string
}

func (r *IAMWorkloadIdentityPoolProvider) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.WorkloadIdentityPools.Delete(*r.Name).Context(ctx).Do()
	return err
}

func (r *IAMWorkloadIdentityPoolProvider) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *IAMWorkloadIdentityPoolProvider) String() string {
	return *r.Name
}
