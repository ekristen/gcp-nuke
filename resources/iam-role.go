package resources

import (
	"context"
	"fmt"
	"github.com/gotidy/ptr"
	"strings"

	iamadmin "cloud.google.com/go/iam/admin/apiv1"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const IAMRoleResource = "IAMRole"

func init() {
	registry.Register(&registry.Registration{
		Name:   IAMRoleResource,
		Scope:  nuke.Project,
		Lister: &IAMRoleLister{},
	})
}

type IAMRoleLister struct {
	svc *iamadmin.IamClient
}

func (l *IAMRoleLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

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
