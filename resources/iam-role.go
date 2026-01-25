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

const IAMRoleResource = "IAMRole"

func init() {
	registry.Register(&registry.Registration{
		Name:     IAMRoleResource,
		Scope:    nuke.Project,
		Resource: &IAMRole{},
		Lister:   &IAMRoleLister{},
	})
}

type IAMRoleLister struct {
	svc *iamadmin.IamClient
}

func (l *IAMRoleLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *IAMRoleLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "iam.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = iamadmin.NewIamClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	var nextPageToken string

	for {
		req := &adminpb.ListRolesRequest{
			Parent:    fmt.Sprintf("projects/%s", *opts.Project),
			PageToken: nextPageToken,
		}

		resp, err := l.svc.ListRoles(ctx, req)
		if err != nil {
			return nil, err
		}

		for _, role := range resp.GetRoles() {
			roleParts := strings.Split(role.GetName(), "/")
			roleName := roleParts[len(roleParts)-1]
			resources = append(resources, &IAMRole{
				svc:     l.svc,
				project: opts.Project,
				Name:    ptr.String(roleName),
				Etag:    role.Etag,
				Stage:   ptr.String(role.GetStage().String()),
				Deleted: ptr.Bool(role.Deleted),
			})
		}

		if resp.GetNextPageToken() == "" {
			break
		}

		nextPageToken = resp.GetNextPageToken()
	}

	return resources, nil
}

type IAMRole struct {
	svc     *iamadmin.IamClient
	project *string
	Name    *string
	Stage   *string
	Etag    []byte
	Deleted *bool
}

func (r *IAMRole) Remove(ctx context.Context) error {
	_, err := r.svc.DeleteRole(ctx, &adminpb.DeleteRoleRequest{
		Name: fmt.Sprintf("projects/%s/roles/%s", *r.project, *r.Name),
		Etag: r.Etag,
	})
	return err
}

func (r *IAMRole) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *IAMRole) String() string {
	return *r.Name
}
