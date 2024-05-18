package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotidy/ptr"
	"strings"

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

	req := &kmspb.ListKeyRingsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, *opts.Region),
	}
	it := l.svc.ListKeyRings(ctx, req)
	for {
		keyRing, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate kms-key")
			break
		}

		reqKeys := &kmspb.ListCryptoKeysRequest{
			Parent: keyRing.Name,
		}
		itKeys := l.svc.ListCryptoKeys(ctx, reqKeys)
		for {
			cryptoKey, err := itKeys.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate kms-key")
				break
			}

			nameParts := strings.Split(cryptoKey.Name, "/")
			name := nameParts[len(nameParts)-1]

			keyringNameParts := strings.Split(keyRing.Name, "/")
			keyringName := keyringNameParts[len(keyringNameParts)-1]

			reqPrimaryVersion := &kmspb.GetCryptoKeyVersionRequest{
				Name: cryptoKey.Primary.Name,
			}
			keyVersion, err := l.svc.GetCryptoKeyVersion(ctx, reqPrimaryVersion)
			if err != nil {
				logrus.WithError(err).Error("unable to get primary key version")
				break
			}

			resources = append(resources, &KMSKey{
				svc:      l.svc,
				project:  opts.Project,
				fullName: ptr.String(keyVersion.Name),
				Name:     ptr.String(name),
				Keyring:  ptr.String(keyringName),
				State:    ptr.String(keyVersion.State.String()),
			})
		}
	}

	return resources, nil
}

type KMSKey struct {
	svc      *kms.KeyManagementClient
	project  *string
	fullName *string
	Name     *string
	Keyring  *string
	State    *string
}

func (r *KMSKey) Remove(ctx context.Context) error {
	reqKeyVersions := &kmspb.ListCryptoKeyVersionsRequest{
		Parent: *r.fullName,
	}

	itKeyVersions := r.svc.ListCryptoKeyVersions(ctx, reqKeyVersions)
	for {
		respKeyVersions, err := itKeyVersions.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate key version")
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

func (r *KMSKey) Filter() error {
	if *r.State == kmspb.CryptoKeyVersion_DESTROY_SCHEDULED.String() {
		return fmt.Errorf("key is already scheduled for destruction")
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
