package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	alloydb "cloud.google.com/go/alloydb/apiv1"
	"cloud.google.com/go/alloydb/apiv1/alloydbpb"
	"google.golang.org/api/iterator"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const AlloyDBInstanceResource = "AlloyDBInstance"

func init() {
	registry.Register(&registry.Registration{
		Name:     AlloyDBInstanceResource,
		Scope:    nuke.Project,
		Resource: &AlloyDBInstance{},
		Lister:   &AlloyDBInstanceLister{},
	})
}

type AlloyDBInstanceLister struct {
	svc *alloydb.AlloyDBAdminClient
}

func (l *AlloyDBInstanceLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *AlloyDBInstanceLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "alloydb.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = alloydb.NewAlloyDBAdminClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	clusterReq := &alloydbpb.ListClustersRequest{
		Parent: "projects/" + *opts.Project + "/locations/" + *opts.Region,
	}

	clusterIt := l.svc.ListClusters(ctx, clusterReq)
	for {
		cluster, err := clusterIt.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate alloydb clusters")
			break
		}

		instanceReq := &alloydbpb.ListInstancesRequest{
			Parent: cluster.Name,
		}

		instanceIt := l.svc.ListInstances(ctx, instanceReq)
		for {
			instance, err := instanceIt.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate alloydb instances")
				break
			}

			nameParts := strings.Split(instance.Name, "/")
			name := nameParts[len(nameParts)-1]

			clusterParts := strings.Split(cluster.Name, "/")
			clusterName := clusterParts[len(clusterParts)-1]

			resources = append(resources, &AlloyDBInstance{
				svc:          l.svc,
				Project:      opts.Project,
				Region:       opts.Region,
				FullName:     ptr.String(instance.Name),
				Name:         ptr.String(name),
				Cluster:      ptr.String(clusterName),
				State:        ptr.String(instance.State.String()),
				InstanceType: ptr.String(instance.InstanceType.String()),
				Labels:       instance.Labels,
			})
		}
	}

	return resources, nil
}

type AlloyDBInstance struct {
	svc          *alloydb.AlloyDBAdminClient
	removeOp     *alloydb.DeleteInstanceOperation
	Project      *string
	Region       *string
	FullName     *string
	Name         *string           `description:"The name of the AlloyDB instance"`
	Cluster      *string           `description:"The cluster this instance belongs to"`
	State        *string           `description:"The current state of the instance"`
	InstanceType *string           `description:"The type of the instance (PRIMARY, READ_POOL)"`
	Labels       map[string]string `property:"tagPrefix=label" description:"Labels associated with the instance"`
}

func (r *AlloyDBInstance) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteInstance(ctx, &alloydbpb.DeleteInstanceRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *AlloyDBInstance) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *AlloyDBInstance) String() string {
	return *r.Name
}

func (r *AlloyDBInstance) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("alloydb instance remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
