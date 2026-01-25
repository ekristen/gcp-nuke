package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gotidy/ptr"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const SecretManagerSecretResource = "SecretManagerSecret"

func init() {
	registry.Register(&registry.Registration{
		Name:     SecretManagerSecretResource,
		Scope:    nuke.Project,
		Resource: &SecretManagerSecret{},
		Lister:   &SecretManagerSecretLister{},
	})
}

type SecretManagerSecretLister struct {
	svc *secretmanager.Client
}

func (l *SecretManagerSecretLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *SecretManagerSecretLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Global, "secretmanager.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = secretmanager.NewRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", *opts.Project),
	}
	it := l.svc.ListSecrets(ctx, req)
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

		resources = append(resources, &SecretManagerSecret{
			svc:        l.svc,
			fullName:   ptr.String(resp.Name),
			Name:       ptr.String(name),
			project:    opts.Project,
			CreateTime: resp.CreateTime.AsTime(),
			Labels:     resp.Labels,
		})
	}

	return resources, nil
}

type SecretManagerSecret struct {
	svc        *secretmanager.Client
	project    *string
	fullName   *string
	Name       *string
	CreateTime time.Time
	Labels     map[string]string `property:"tagPrefix=label"`
}

func (r *SecretManagerSecret) Remove(ctx context.Context) error {
	return r.svc.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: *r.fullName,
	})
}

func (r *SecretManagerSecret) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *SecretManagerSecret) String() string {
	return *r.Name
}
