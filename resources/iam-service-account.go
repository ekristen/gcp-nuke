package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/ekristen/libnuke/pkg/settings"
	"github.com/gotidy/ptr"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	iamadmin "cloud.google.com/go/iam/admin/apiv1"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const IAMServiceAccountResource = "IAMServiceAccount"

func init() {
	registry.Register(&registry.Registration{
		Name:   IAMServiceAccountResource,
		Scope:  nuke.Project,
		Lister: &IAMServiceAccountLister{},
		Settings: []string{
			"DeleteDefaultServiceAccounts",
		},
	})
}

type IAMServiceAccountLister struct {
	svc *iamadmin.IamClient
}

func (l *IAMServiceAccountLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = iamadmin.NewIamClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &adminpb.ListServiceAccountsRequest{
		Name: fmt.Sprintf("projects/%s", *opts.Project),
	}
	it := l.svc.ListServiceAccounts(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &IAMServiceAccount{
			svc:         l.svc,
			project:     opts.Project,
			fullName:    ptr.String(resp.Name),
			ID:          ptr.String(resp.UniqueId),
			Name:        ptr.String(name),
			Description: ptr.String(resp.Description),
		})
	}

	return resources, nil
}

type IAMServiceAccount struct {
	svc         *iamadmin.IamClient
	settings    *settings.Setting
	project     *string
	region      *string
	fullName    *string
	ID          *string
	Name        *string
	Description *string
}

func (r *IAMServiceAccount) Filter() error {
	deleteDefaultServiceAccounts := false
	if r.settings != nil && r.settings.Get("DeleteDefaultServiceAccounts").(bool) {
		deleteDefaultServiceAccounts = true
	}
	if !strings.Contains(*r.Name, ".iam.gserviceaccount.com") && !deleteDefaultServiceAccounts {
		return fmt.Errorf("will not remove default service account")
	}

	return nil
}

func (r *IAMServiceAccount) Remove(ctx context.Context) error {
	return r.svc.DeleteServiceAccount(ctx, &adminpb.DeleteServiceAccountRequest{
		Name: *r.fullName,
	})
}

func (r *IAMServiceAccount) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *IAMServiceAccount) String() string {
	return *r.Name
}

func (r *IAMServiceAccount) Settings(settings *settings.Setting) {
	r.settings = settings
}
