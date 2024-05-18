package resources

import (
	"context"
	"fmt"
	"github.com/gotidy/ptr"
	"google.golang.org/api/firebase/v1beta1"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const FirebaseWebAppResource = "FirebaseWebApp"

func init() {
	registry.Register(&registry.Registration{
		Name:   FirebaseWebAppResource,
		Scope:  nuke.Project,
		Lister: &FirebaseWebAppLister{},
	})
}

type FirebaseWebAppLister struct {
	svc *firebase.Service
}

func (l *FirebaseWebAppLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = firebase.NewService(ctx)
		if err != nil {
			return nil, err
		}
	}

	resp, err := l.svc.Projects.WebApps.List(fmt.Sprintf("projects/%s", *opts.Project)).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, app := range resp.Apps {
		resources = append(resources, &FirebaseWebApp{
			svc:         l.svc,
			project:     opts.Project,
			region:      opts.Region,
			fullName:    ptr.String(app.Name),
			DisplayName: ptr.String(app.DisplayName),
			AppID:       ptr.String(app.AppId),
			State:       ptr.String(app.State),
		})
	}

	return resources, nil
}

type FirebaseWebApp struct {
	svc         *firebase.Service
	project     *string
	region      *string
	fullName    *string
	DisplayName *string
	AppID       *string
	State       *string
}

func (r *FirebaseWebApp) Remove(ctx context.Context) error {
	_, err := r.svc.Projects.WebApps.Remove(*r.fullName, &firebase.RemoveWebAppRequest{
		AllowMissing: true,
		Immediate:    true,
	}).Context(ctx).Do()
	return err
}

func (r *FirebaseWebApp) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *FirebaseWebApp) String() string {
	return *r.DisplayName
}
