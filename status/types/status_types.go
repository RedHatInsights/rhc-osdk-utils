package status

import (
	"context"

	cond "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//The status code in this lib was factored-out from Clowder. Originally there were two parallel code paths
//for status: one that was ClowdApp specific and one that was ClowdEnv specific. The overlap between these
//code paths was large: 80% or so. The goal was to unify those code paths into a single status checking system
//and then factor that out from Clowder into this library. The main challenge in unifying the code paths was
//type incompatibility. Most of that was solved with the StatusSource interface. Whatever implements that
//interface can participate in the status system. However, another problem was that each status checking type
//passed around a struct that represented the metrics it used to determine readiness. These types were largely
//the same, but not exactly. StatusSourceFigures is introduced to handle that. It encapsulates the figures implementing
//types care about. Different types can care about different figures, it doesn't matter - types can ignore what they don't care about.
//This struct can be extended with new members to satisfy new figures in the future without breaking any
//currently-existing implementing types
type StatusSourceFigures struct {
	ManagedDeployments int32
	ReadyDeployments   int32
	ManagedTopics      int32
	ReadyTopics        int32
}

//Defines an interface for objects that want to participate in the status system
type StatusSource interface {
	//Set the StatusSource to a ready value via the provided bool. It is up to the implementing type
	//to decide what the implementation entails. For example, a ClowdApp in Clowder would set Status.Ready to the provided bool
	SetStatusReady(bool)
	//Get a list of namespaces names as an array of strings. The namespace list is used when counting deployments and so must
	//be implemented by the implementing type. Typically, implementation involves getting namespaces in the environment
	GetNamespaces(ctx context.Context, client client.Client) ([]string, error)
	//Accepts a StatusSourceFigures and allows the implementing type to set whatever figures it is interested in
	//based on the values in the passed-in struct
	SetDeploymentFigures(StatusSourceFigures)
	//Accepts a StatusSourceFigures and returns a bool that represents deployment readiness status based on the
	//passed-in struct. What figures an implementing type is interested-in is up to the implementing type.
	//For example, one implementing type may be interested in only deployments while another might be interested in
	//deployments and topics
	AreDeploymentsReady(StatusSourceFigures) bool
	//Allows the implementing type to gather readiness figures other than ManagedDeployments vs ReadyDeployments. This
	//method is called during GetResourceFigures. If the implementing type doesn't care about figures besides
	//deployments it is perfectly valid to return an empty StatusSourceFigures as a sort of NOOP. However,
	//if the implementing type cares about specific figures, such as Topics, the logic for deriving those figures
	//must be implemented here
	GetObjectSpecificFigures(context.Context, client.Client) (StatusSourceFigures, string, error)
	//Implementing-type specific logic for adding StatusSourceFigures structs
	//Why is this part of the interface and not just some function here in status or something?
	//The thinking is that each object can decide what part of the figures struct it cares
	//about. This allows us to encapsulate the implementation and get some more polymoprhism out
	//of the code
	AddDeploymentFigures(StatusSourceFigures, StatusSourceFigures) StatusSourceFigures
	cond.Setter
}
