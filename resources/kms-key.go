package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const KMSKeyResource = "KMSKey"

func init() {
	registry.Register(&registry.Registration{
		Name:     KMSKeyResource,
		Scope:    nuke.Project,
		Resource: &KMSKey{},
		Lister:   &KMSKeyLister{},
	})
}

type KMSKeyLister struct {
	svc *kms.KeyManagementClient
}

func (l *KMSKeyLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "cloudkms.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = kms.NewKeyManagementRESTClient(ctx, opts.ClientOptions...)
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
	reqKeyVersion := &kmspb.DestroyCryptoKeyVersionRequest{
		Name: *r.fullName,
	}
	_, err := r.svc.DestroyCryptoKeyVersion(ctx, reqKeyVersion)
	if err != nil {
		return err
	}

	return nil
}

func (r *KMSKey) Filter() error {
	if *r.State == kmspb.CryptoKeyVersion_DESTROYED.String() {
		return fmt.Errorf("key is already destroyed")
	}
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
