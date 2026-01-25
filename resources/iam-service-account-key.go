package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"

	iamadmin "cloud.google.com/go/iam/admin/apiv1"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const IAMServiceAccountKeyResource = "IAMServiceAccountKey"

func init() {
	registry.Register(&registry.Registration{
		Name:     IAMServiceAccountKeyResource,
		Scope:    nuke.Project,
		Resource: &IAMServiceAccountKey{},
		Lister:   &IAMServiceAccountKeyLister{},
	})
}

type IAMServiceAccountKeyLister struct {
	svc *iamadmin.IamClient
}

func (l *IAMServiceAccountKeyLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *IAMServiceAccountKeyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "iam.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = iamadmin.NewIamClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	saLister := &IAMServiceAccountLister{
		svc: l.svc,
	}

	sas, err := saLister.ListServiceAccounts(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, sa := range sas {
		keys, err := l.svc.ListServiceAccountKeys(ctx, &adminpb.ListServiceAccountKeysRequest{
			Name: sa.Name,
		})
		if err != nil {
			return resources, err
		}

		for _, key := range keys.Keys {
			keyParts := strings.Split(key.Name, "/")
			uniqueID := keyParts[len(keyParts)-1]

			resources = append(resources, &IAMServiceAccountKey{
				svc:                 l.svc,
				project:             opts.Project,
				name:                ptr.String(key.Name),
				ID:                  ptr.String(uniqueID),
				Algorithm:           ptr.String(key.KeyAlgorithm.String()),
				ManagedType:         ptr.String(key.KeyType.String()),
				ServiceAccount:      ptr.String(sa.DisplayName),
				ServiceAccountID:    ptr.String(sa.UniqueId),
				ServiceAccountEmail: ptr.String(sa.Email),
				Disabled:            ptr.Bool(key.Disabled),
			})
		}
	}

	return resources, nil
}

type IAMServiceAccountKey struct {
	svc                 *iamadmin.IamClient
	project             *string
	name                *string
	ID                  *string
	Algorithm           *string
	ManagedType         *string
	ServiceAccount      *string
	ServiceAccountID    *string
	ServiceAccountEmail *string
	Disabled            *bool
}

func (r *IAMServiceAccountKey) Filter() error {
	if *r.ManagedType == "SYSTEM_MANAGED" {
		return fmt.Errorf("will not remove system managed key")
	}
	return nil
}

func (r *IAMServiceAccountKey) Remove(ctx context.Context) error {
	return r.svc.DeleteServiceAccountKey(ctx, &adminpb.DeleteServiceAccountKeyRequest{
		Name: *r.name,
	})
}

func (r *IAMServiceAccountKey) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *IAMServiceAccountKey) String() string {
	return fmt.Sprintf("%s -> %s", *r.ServiceAccountEmail, *r.ID)
}
