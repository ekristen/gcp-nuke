package resources

import (
	"context"
	"strings"

	"github.com/gotidy/ptr"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/gcputil"
	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const FirebaseOAuthProviderResource = "FirebaseOAuthProvider"

func init() {
	registry.Register(&registry.Registration{
		Name:     FirebaseOAuthProviderResource,
		Scope:    nuke.Project,
		Resource: &FirebaseOAuthProvider{},
		Lister:   &FirebaseOAuthProviderLister{},
	})
}

type FirebaseOAuthProviderLister struct {
	svc *gcputil.IdentityPlatformService
}

func (l *FirebaseOAuthProviderLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "identitytoolkit.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = gcputil.NewIdentityPlatformService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	builtinProviders, err := l.svc.ListDefaultSupportedOAuthIdpConfigs(ctx, *opts.Project)
	if err != nil {
		return nil, err
	}

	for _, provider := range builtinProviders.DefaultSupportedIdpConfigs {
		parts := strings.Split(*provider.Name, "/")
		shortName := parts[len(parts)-1]

		resources = append(resources, &FirebaseOAuthProvider{
			svc:     l.svc,
			project: opts.Project,
			Name:    ptr.String(shortName),
			Type:    ptr.String("builtin"),
		})
	}

	customProviders, err := l.svc.ListOAuthIdpConfigs(ctx, *opts.Project)
	if err != nil {
		return nil, err
	}

	for _, provider := range customProviders.OAuthIdpConfigs {
		parts := strings.Split(*provider.Name, "/")
		shortName := parts[len(parts)-1]

		resources = append(resources, &FirebaseOAuthProvider{
			svc:     l.svc,
			project: opts.Project,
			Name:    ptr.String(shortName),
			Type:    ptr.String("custom"),
		})
	}

	return resources, nil
}

type FirebaseOAuthProvider struct {
	svc     *gcputil.IdentityPlatformService
	project *string
	Name    *string `description:"The name of the OAuth provider"`
	Type    *string `description:"The type of the OAuth provider, either builtin or custom"`
}

func (r *FirebaseOAuthProvider) Remove(ctx context.Context) error {
	if ptr.ToString(r.Type) == "builtin" {
		return r.svc.DeleteDefaultSupportedOAuthIdpConfig(ctx, *r.project, *r.Name)
	}

	return r.svc.DeleteOAuthIdpConfig(ctx, *r.project, *r.Name)
}

func (r *FirebaseOAuthProvider) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *FirebaseOAuthProvider) String() string {
	return *r.Name
}
