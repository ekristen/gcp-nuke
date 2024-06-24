package resources

import (
	"context"
	"github.com/ekristen/gcp-nuke/pkg/nuke"
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
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "iam.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = iam.NewService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	workloadIdentityPoolLister := &IAMWorkloadIdentityPoolLister{}
	workloadIdentityPools, err := workloadIdentityPoolLister.ListPools(ctx, opts)
	if err != nil {
		return nil, err
	}

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
					project:     opts.Project,
					region:      opts.Region,
					fullName:    ptr.String(provider.Name),
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
	project     *string
	region      *string
	fullName    *string
	Name        *string
	Pool        *string
	Disabled    *bool
	DisplayName *string
	ExpireTime  *string
}

func (r *IAMWorkloadIdentityPoolProvider) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.Locations.WorkloadIdentityPools.Providers.Delete(*r.fullName).Context(ctx).Do()
	return err
}

func (r *IAMWorkloadIdentityPoolProvider) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *IAMWorkloadIdentityPoolProvider) String() string {
	return *r.Name
}
