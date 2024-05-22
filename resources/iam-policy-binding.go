package resources

import (
	"context"
	"fmt"
	"github.com/ekristen/libnuke/pkg/settings"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/cloudresourcemanager/v3"
	"strings"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const IAMPolicyBindingResource = "IAMPolicyBinding"

func init() {
	registry.Register(&registry.Registration{
		Name:   IAMPolicyBindingResource,
		Scope:  nuke.Project,
		Lister: &IAMPolicyBindingLister{},
		Settings: []string{
			"DeleteGoogleManaged",
		},
	})
}

type IAMPolicyBindingLister struct {
	svc *cloudresourcemanager.Service
}

func (l *IAMPolicyBindingLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region != "global" {
		return nil, liberror.ErrSkipRequest("resource is global")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = cloudresourcemanager.NewService(ctx)
		if err != nil {
			return nil, err
		}
	}

	resp, err := l.svc.Projects.
		GetIamPolicy(fmt.Sprintf("projects/%s", *opts.Project), &cloudresourcemanager.GetIamPolicyRequest{}).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	for _, binding := range resp.Bindings {
		for _, member := range binding.Members {
			iamPolicyBinding := &IAMPolicyBinding{
				svc:     l.svc,
				project: opts.Project,
				Role:    binding.Role,
				Member:  member,
			}

			parts := strings.Split(member, "@")
			if strings.HasSuffix(parts[1], ".gserviceaccount.com") && !strings.HasPrefix(parts[1], *opts.Project) {
				iamPolicyBinding.GoogleManaged = true
			}

			if strings.HasPrefix(iamPolicyBinding.Member, "deleted:") {
				iamPolicyBinding.IsDeleted = true
			}

			if strings.HasPrefix(iamPolicyBinding.Member, "serviceAccount:") {
				iamPolicyBinding.MemberType = "serviceAccount"
			} else if strings.HasPrefix(iamPolicyBinding.Member, "user:") {
				iamPolicyBinding.MemberType = "user"
			} else {
				iamPolicyBinding.MemberType = "unknown"
			}

			resources = append(resources, iamPolicyBinding)
		}

	}

	return resources, nil
}

type IAMPolicyBinding struct {
	svc           *cloudresourcemanager.Service
	settings      *settings.Setting
	project       *string
	Role          string
	Member        string
	MemberType    string
	IsDeleted     bool
	GoogleManaged bool
}

func (r *IAMPolicyBinding) Filter() error {
	deleteGoogleManaged := false

	if r.settings != nil {
		deleteGoogleManaged = r.settings.Get("DeleteGoogleManaged").(bool)
	}

	if r.GoogleManaged && !deleteGoogleManaged {
		return fmt.Errorf("binding is managed by Google")
	}

	return nil
}

func (r *IAMPolicyBinding) Remove(ctx context.Context) error {
	policy, err := r.svc.Projects.
		GetIamPolicy(fmt.Sprintf("projects/%s", *r.project), &cloudresourcemanager.GetIamPolicyRequest{}).
		Context(ctx).Do()
	if err != nil {
		return err
	}

	for _, binding := range policy.Bindings {
		if binding.Role == r.Role {
			for i, member := range binding.Members {
				if member == r.Member {
					// remove current index from slice
					binding.Members = append(binding.Members[:i], binding.Members[i+1:]...)
				}
			}
		}
	}

	// Set the updated IAM policy with an update mask
	_, err = r.svc.Projects.SetIamPolicy(fmt.Sprintf("projects/%s", *r.project), &cloudresourcemanager.SetIamPolicyRequest{
		Policy: policy,
	}).Context(ctx).Do()
	if err != nil {
		logrus.Errorf("error removing IAM policy binding: %v", err)
		return err
	}

	return nil
}

func (r *IAMPolicyBinding) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *IAMPolicyBinding) String() string {
	return fmt.Sprintf("%s -> %s", r.Member, r.Role)
}

func (r *IAMPolicyBinding) Settings(setting *settings.Setting) {
	r.settings = setting
}
