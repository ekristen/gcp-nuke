package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotidy/ptr"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const KMSKeyResource = "KMSKey"

func init() {
	registry.Register(&registry.Registration{
		Name:   KMSKeyResource,
		Scope:  nuke.Project,
		Lister: &KMSKeyLister{},
	})
}

type KMSKeyLister struct {
	svc *kms.KeyManagementClient
}

func (l *KMSKeyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	if *opts.Region == "global" {
		return nil, liberror.ErrSkipRequest("resource is regional")
	}

	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = kms.NewKeyManagementRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	// NOTE: you might have to modify the code below to actually work, this currently does not
	// inspect the aws sdk instead is a jumping off point
	req := &kmspb.ListKeyRingsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListKeyRings(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		resources = append(resources, &KMSKey{
			svc:     l.svc,
			Name:    ptr.String(resp.Name),
			Project: opts.Project,
		})
	}

	return resources, nil
}

type KMSKey struct {
	svc     *kms.KeyManagementClient
	Project *string
	Region  *string
	Name    *string
}

func (r *KMSKey) Remove(ctx context.Context) error {
	reqKeyVersions := &kmspb.ListCryptoKeyVersionsRequest{
		Parent: *r.Name,
	}
	itKeyVersions := r.svc.ListCryptoKeyVersions(ctx, reqKeyVersions)
	for {
		respKeyVersions, err := itKeyVersions.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		if isDestroyed(respKeyVersions) {
			continue
		}

		reqKeyVersion := &kmspb.DestroyCryptoKeyVersionRequest{
			Name: respKeyVersions.Name,
		}
		_, err = r.svc.DestroyCryptoKeyVersion(ctx, reqKeyVersion)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *KMSKey) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *KMSKey) String() string {
	return *r.Name
}

// isDestroyed checks if a key version is destroyed
func isDestroyed(keyVersion *kmspb.CryptoKeyVersion) bool {
	if keyVersion == nil {
		return true
	}
	return keyVersion.State == kmspb.CryptoKeyVersion_DESTROYED ||
		keyVersion.State == kmspb.CryptoKeyVersion_DESTROY_SCHEDULED
}
