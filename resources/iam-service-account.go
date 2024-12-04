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

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const IAMServiceAccountResource = "IAMServiceAccount"

func init() {
	registry.Register(&registry.Registration{
		Name:     IAMServiceAccountResource,
		Scope:    nuke.Project,
		Resource: &IAMServiceAccount{},
		Lister:   &IAMServiceAccountLister{},
		Settings: []string{
			"DeleteDefaultServiceAccounts",
		},
		DependsOn: []string{
			IAMServiceAccountKeyResource,
			IAMPolicyBindingResource,
		},
	})
}

type IAMServiceAccountLister struct {
	svc *iamadmin.IamClient
}

func (l *IAMServiceAccountLister) ListServiceAccounts(
	ctx context.Context, opts *nuke.ListerOpts) ([]*adminpb.ServiceAccount, error) {
	if l.svc == nil {
		var err error
		l.svc, err = iamadmin.NewIamClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	var serviceAccounts []*adminpb.ServiceAccount

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
			logrus.WithError(err).Error("unable to iterate service accounts")
			break
		}

		serviceAccounts = append(serviceAccounts, resp)
	}

	return serviceAccounts, nil
}

func (l *IAMServiceAccountLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "iam.googleapis.com"); err != nil {
		return resources, err
	}

	serviceAccounts, err := l.ListServiceAccounts(ctx, opts)
	if err != nil {
		return resources, err
	}

	for _, serviceAccount := range serviceAccounts {
		nameParts := strings.Split(serviceAccount.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &IAMServiceAccount{
			svc:         l.svc,
			project:     opts.Project,
			fullName:    ptr.String(serviceAccount.Name),
			ID:          ptr.String(serviceAccount.UniqueId),
			Name:        ptr.String(name),
			Description: ptr.String(serviceAccount.Description),
		})
	}

	return resources, nil
}

type IAMServiceAccount struct {
	svc         *iamadmin.IamClient
	settings    *settings.Setting
	project     *string
	fullName    *string
	ID          *string
	Name        *string
	Description *string
}

func (r *IAMServiceAccount) Filter() error {
	isDefaultServiceAccount := false
	deleteDefaultServiceAccounts := false
	if r.settings.GetBool("DeleteDefaultServiceAccounts") {
		deleteDefaultServiceAccounts = true
	}

	if !strings.Contains(*r.Name, ".iam.gserviceaccount.com") {
		isDefaultServiceAccount = true
	}
	if strings.HasPrefix(*r.Name, "project-service-account@") {
		isDefaultServiceAccount = true
	}
	if strings.HasPrefix(*r.Name, "firebase-adminsdk-") {
		isDefaultServiceAccount = true
	}

	if isDefaultServiceAccount && !deleteDefaultServiceAccounts {
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
