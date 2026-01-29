package resources

import (
	"context"

	"github.com/gotidy/ptr"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/gcputil"
	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const FirebaseAuthProviderResource = "FirebaseAuthProvider"

func init() {
	registry.Register(&registry.Registration{
		Name:     FirebaseAuthProviderResource,
		Scope:    nuke.Project,
		Resource: &FirebaseAuthProvider{},
		Lister:   &FirebaseAuthProviderLister{},
	})
}

type FirebaseAuthProviderLister struct {
	svc *gcputil.IdentityPlatformService
}

func (l *FirebaseAuthProviderLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "identitytoolkit.googleapis.com", FirebaseAuthProviderResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = gcputil.NewIdentityPlatformService(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	cfg, err := l.svc.GetProjectConfig(ctx, *opts.Project)
	if err != nil {
		return nil, err
	}

	if cfg.SignIn == nil {
		return resources, nil
	}

	if cfg.SignIn.Email != nil && cfg.SignIn.Email.Enabled {
		resources = append(resources, &FirebaseAuthProvider{
			svc:     l.svc,
			project: opts.Project,
			Name:    ptr.String("email"),
		})
	}
	if cfg.SignIn.Phone != nil && cfg.SignIn.Phone.Enabled {
		resources = append(resources, &FirebaseAuthProvider{
			svc:     l.svc,
			project: opts.Project,
			Name:    ptr.String("phone"),
		})
	}
	if cfg.SignIn.Anonymous != nil && cfg.SignIn.Anonymous.Enabled {
		resources = append(resources, &FirebaseAuthProvider{
			svc:     l.svc,
			project: opts.Project,
			Name:    ptr.String("anonymous"),
		})
	}

	return resources, nil
}

type FirebaseAuthProvider struct {
	svc     *gcputil.IdentityPlatformService
	project *string
	Name    *string
}

func (r *FirebaseAuthProvider) Remove(ctx context.Context) error {
	baseCfg := &gcputil.ProjectConfig{}

	if r.Name == ptr.String("email") {
		baseCfg.SignIn.Email = &gcputil.ProviderConfig{Enabled: false}
	} else if r.Name == ptr.String("phone") {
		baseCfg.SignIn.Phone = &gcputil.ProviderConfig{Enabled: false}
	} else if r.Name == ptr.String("anonymous") {
		baseCfg.SignIn.Anonymous = &gcputil.ProviderConfig{Enabled: false}
	}

	_, err := r.svc.UpdateProjectConfig(ctx, *r.project, baseCfg)
	return err
}

func (r *FirebaseAuthProvider) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *FirebaseAuthProvider) String() string {
	return *r.Name
}
