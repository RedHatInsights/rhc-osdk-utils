package resources

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GVKs struct {
	Deployment   schema.GroupVersionKind
	Kafka        schema.GroupVersionKind
	KafkaTopic   schema.GroupVersionKind
	KafkaConnect schema.GroupVersionKind
}

var CommonGVKs = GVKs{
	Deployment: schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	},
	Kafka: schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1beta2",
		Kind:    "Kafka",
	},
	KafkaConnect: schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1beta2",
		Kind:    "KafkaConnect",
	},
	KafkaTopic: schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1beta2",
		Kind:    "KafkaTopic",
	},
}

//An operator is interested in how many of a resource it manages and how many are ready
type ResourceFigures struct {
	Total int32
	Ready int32
}

//In order to status check resources we check their status conditions for a condition
//with a specific type and status. This type lets us define and pass those values around
type ResourceConditionReadyRequirements struct {
	Type   string
	Status string
}

//Represents the metadata we pull off the unstructured resource
type ResourceMetadata struct {
	Generation      int64
	Namespace       string
	Name            string
	UID             string
	ResourceVersion string
	OwnerUIDs       []string
}

//Represents the status we pull off the unstructured resource
//I only parse what I need to reduce the oppurtunity for bugs
type ResourceStatus struct {
	ObservedGeneration int64
}

//Make a resource from an unstructured, parse it, and return it
func MakeResource(source unstructured.Unstructured) Resource {
	res := Resource{}

	res.parseMetadata(source)
	res.parseStatusConditions(source)
	res.parseStatus(source)

	return res
}

func MakeQuery(typeSpecimen client.Object, scheme runtime.Scheme, namespaces []string, uid types.UID) (ResourceCounterQuery, error) {
	gvk, _, err := scheme.ObjectKinds(typeSpecimen)
	if err != nil {
		return ResourceCounterQuery{}, err
	}
	return ResourceCounterQuery{
		Namespaces: namespaces,
		//This creeps me out and I do not like it
		//Assuming the first entry is right feels very fly by night
		GVK:       gvk[0],
		OwnerGUID: string(uid),
	}, nil
}

//Represents a k8s resource in a type-neutral way
//We used to have lots of repeated code because we needed to perform
//the same operations on different resources, which are represented by
//the go k8s sdk as different types. This type provides a single
//type that can represent any kind of resource and perform common
//actions
type Resource struct {
	Status            ResourceStatus
	Metadata          ResourceMetadata
	Conditions        []map[string]string
	ReadyRequirements []ResourceConditionReadyRequirements
}

//Adds a resource ready requirement
func (r *Resource) AddReadyRequirements(requirements ResourceConditionReadyRequirements) {
	r.ReadyRequirements = append(r.ReadyRequirements, requirements)
}

//Returns true of the resource is owned by the given guid
func (r *Resource) IsOwnedBy(ownerUID string) bool {
	for _, ownerRef := range r.Metadata.OwnerUIDs {
		if ownerRef == ownerUID {
			return true
		}
	}
	return false
}

//Get the ready status
func (r *Resource) IsReady() bool {
	return r.readyConditionFound() && r.generationNumbersMatch()
}

//Returns true if the ready conditions are found
//We only care to find one matching condition. Not all need to match to be Ready
func (r *Resource) readyConditionFound() bool {
	for _, condition := range r.Conditions {
		for _, requirement := range r.ReadyRequirements {
			if condition["type"] == requirement.Type && condition["status"] == requirement.Status {
				return true
			}
		}
	}
	return false
}

//Returns true of the generation numbers are correct
func (r *Resource) generationNumbersMatch() bool {
	return r.Metadata.Generation <= r.Status.ObservedGeneration
}

//Gets the metadata from the source unstructured.Unstructured object
func (r *Resource) parseMetadata(source unstructured.Unstructured) {
	source.GetGeneration()
	source.GetNamespace()

	var ownerUIDs []string

	for _, ownerReference := range source.GetOwnerReferences() {
		ownerUIDs = append(ownerUIDs, string(ownerReference.UID))
	}

	r.Metadata = ResourceMetadata{
		Generation:      source.GetGeneration(),
		Namespace:       source.GetNamespace(),
		Name:            source.GetName(),
		UID:             string(source.GetUID()),
		ResourceVersion: source.GetResourceVersion(),
		OwnerUIDs:       ownerUIDs,
	}
}

func (r *Resource) interfaceMapHasKey(inMap map[string]interface{}, key string) bool {
	_, ok := inMap[key]
	return ok
}

//Parses a subset of the unstructures source status
func (r *Resource) parseStatus(source unstructured.Unstructured) {
	statusSource := source.Object["status"].(map[string]interface{})

	//observed
	var observedGen int64
	observedGen = -1

	if r.interfaceMapHasKey(statusSource, "observedGeneration") {
		observedGen = statusSource["observedGeneration"].(int64)
	}

	r.Status = ResourceStatus{
		ObservedGeneration: observedGen,
	}
}

//Parses the unstructured source metadata conditions into this Resource objects Conditions array of maps
func (r *Resource) parseStatusConditions(source unstructured.Unstructured) {
	status := source.Object["status"].(map[string]interface{})

	//If the source object doesn't have conditions we can just bail
	//They don't need to be there, we'll just get not ready without them which is fine
	//Note: This will happen frequently if a resource hasn't yet been reconciled
	if !r.interfaceMapHasKey(status, "conditions") {
		return
	}

	//Get the conditions from the status object as an array
	conditions := status["conditions"].([]interface{})
	//Iterate over the conditions
	for _, condition := range conditions {
		//Get the condition as a map
		conditionMap := condition.(map[string]interface{})
		//Get the condition parts
		condStatus := conditionMap["status"].(string)
		condType := conditionMap["type"].(string)
		condReason := conditionMap["reason"].(string)
		//Package the conditions up into an easy to use format
		outputConditionMap := map[string]string{
			"status": condStatus,
			"type":   condType,
			"reason": condReason,
		}
		//Add it to the output
		r.Conditions = append(r.Conditions, outputConditionMap)
	}
}

//The pattern other k8s SDKs follow is that resources are queried by some criteria
//resulting in a list object that then can be iterated over or acted on.
//ResourceList can query k8s for a list of resources by GVK and namespace
//which it then parses into Resource instances.
type ResourceList struct {
	source    unstructured.UnstructuredList
	Resources []Resource
}

//Get a count of the resources
func (r *ResourceList) Count() int {
	return len(r.Resources)
}

//Get a count of the ready resources
func (r *ResourceList) CountReady() int {
	count := 0
	for _, resource := range r.Resources {
		if resource.IsReady() {
			count += 1
		}
	}
	return count
}

type ResourceStatusBuckets struct {
	Ready  []Resource
	Broken []Resource
}

//Get a lists of resources bucketed by status
func (r *ResourceList) GetResourceStatusBuckets() ResourceStatusBuckets {
	retVal := ResourceStatusBuckets{}
	for _, resource := range r.Resources {
		if resource.IsReady() {
			retVal.Ready = append(retVal.Ready, resource)
		} else {
			retVal.Broken = append(retVal.Broken, resource)
		}
	}
	return retVal
}

//Get a new list filtered by a specific owner UID
func (r *ResourceList) FilterByOwnerUID(ownerUID string) ResourceList {
	newResourceList := ResourceList{}
	newResourceList.source = r.source
	for _, resource := range r.Resources {
		if resource.IsOwnedBy(ownerUID) {
			newResourceList.Resources = append(newResourceList.Resources, resource)
		}
	}
	return newResourceList
}

//Set the source unstructured list and then parse it
func (r *ResourceList) SetListAndParse(uList unstructured.UnstructuredList) {
	r.source = uList
	r.parseSource()
}

//Gets a ResourceList by a provided GVK and Namespace
func (r *ResourceList) GetByGVKAndNamespace(pClient client.Client, ctx context.Context, namespace string, gvk schema.GroupVersionKind) error {
	unstructuredObjects := unstructured.Unstructured{}

	unstructuredObjects.SetGroupVersionKind(gvk)

	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := pClient.List(ctx, &unstructuredObjects, opts...)
	if err != nil {
		return err
	}

	uList, err := unstructuredObjects.ToList()
	if err != nil {
		return err
	}

	r.SetListAndParse(*uList)

	return nil
}

//Parses the source unstructured.UnstructuredList into an array of Resources
func (r *ResourceList) parseSource() {
	for _, unstructured := range r.source.Items {
		r.Resources = append(r.Resources, MakeResource(unstructured))
	}
}

//Add resource ready requirements for all of the resources in the list
func (r *ResourceList) AddReadyRequirementsFromSlice(reqsList []ResourceConditionReadyRequirements) {
	var updatedResources = []Resource{}
	for _, resource := range r.Resources {
		for _, reqs := range reqsList {
			resource.AddReadyRequirements(reqs)
		}
		updatedResources = append(updatedResources, resource)
	}
	r.Resources = updatedResources
}

//Results that a resource counter provides back when its count method is called
type ResourceCounterResults struct {
	Managed       int
	Ready         int
	BrokenMessage string
}

//Represents a resource query for a count
//We count resources of a given GVK (which derive from OfType ), in a given set of namespaces, owned by a given guid
type ResourceCounterQuery struct {
	GVK        schema.GroupVersionKind
	Namespaces []string
	OwnerGUID  string
}

//Provides a simple API for getting common figures on Resources and ResourceLists
//The Count method returns a ResourceCounterResults instance
type ResourceCounter struct {
	CountManaged      int
	CountReady        int
	BrokenLog         []string
	Query             ResourceCounterQuery
	ReadyRequirements []ResourceConditionReadyRequirements
}

//Counts the resources
func (r *ResourceCounter) Count(ctx context.Context, pClient client.Client) ResourceCounterResults {
	for _, namespace := range r.Query.Namespaces {
		resourceList := r.GetResourceList(pClient, ctx, namespace)
		r.countInNamespace(resourceList)
	}
	return ResourceCounterResults{
		Managed:       r.CountManaged,
		Ready:         r.CountReady,
		BrokenMessage: r.getBrokenMessage(),
	}
}

//Counts up the managed resources in a given namespace
func (r *ResourceCounter) countInNamespace(resources ResourceList) {
	resources.AddReadyRequirementsFromSlice(r.ReadyRequirements)

	resources = resources.FilterByOwnerUID(r.Query.OwnerGUID)

	r.CountManaged += resources.Count()
	r.CountReady += resources.CountReady()
	r.generateBrokenLog(resources.GetResourceStatusBuckets().Broken)
}

func (r *ResourceCounter) GetResourceList(pClient client.Client, ctx context.Context, namespace string) ResourceList {
	resources := ResourceList{}
	resources.GetByGVKAndNamespace(pClient, ctx, namespace, r.Query.GVK)
	return resources
}

//Generates the text broken resource log
func (r *ResourceCounter) generateBrokenLog(brokenResourceList []Resource) {
	for _, resource := range brokenResourceList {
		r.BrokenLog = append(r.BrokenLog, fmt.Sprintf("%s/%s", resource.Metadata.Name, resource.Metadata.Namespace))
	}
}

//Returns the broken log, sorted and in a single string
func (r *ResourceCounter) getBrokenMessage() string {
	retVal := ""
	if len(r.BrokenLog) > 0 {
		sort.Strings(r.BrokenLog)
		retVal = fmt.Sprintf("broken resources: [%s]", strings.Join(r.BrokenLog, ", "))
	}
	return retVal
}
