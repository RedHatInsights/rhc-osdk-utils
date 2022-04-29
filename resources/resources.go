package resources

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	OwnerReferences []string
}

type ResourceStatus struct {
	ObservedGeneration int64
}

//Represents a k8s resource in a type-neutral way
//We used to have lots of repeated code because we needed to perform
//the same operations on different resources, which are represented by
//the go k8s sdk as different types. This type provides a single
//type that can represent any kind of resource and perform common
//actions
type Resource struct {
	Source         unstructured.Unstructured
	Status         ResourceStatus
	Metadata       ResourceMetadata
	Conditions     []map[string]string
	ConditionClass ResourceConditionReadyRequirements
}

//Sets the resource ready requirements
func (r *Resource) SetReadyRequirements(class ResourceConditionReadyRequirements) {
	r.ConditionClass = class
}

//Returns true of the resource is owned by the given guid
func (r *Resource) IsOwnedBy(ownerUID string) bool {
	for _, ownerRef := range r.Metadata.OwnerReferences {
		if ownerRef == ownerUID {
			return true
		}
	}
	return false
}

//Parses a resource unstructured object to populate this Resource object
func (r *Resource) Parse(uObject unstructured.Unstructured) {
	r.Source = uObject
	r.parseMetadata()
	r.parseStatusConditions()
	r.parseStatus()
}

//Get the ready status
func (r *Resource) IsReady() bool {
	return r.readyConditionFound() && r.generationNumbersMatch()
}

//Returns true of the ready conditions are found
func (r *Resource) readyConditionFound() bool {
	for _, condition := range r.Conditions {
		if condition["type"] == r.ConditionClass.Type && condition["status"] == r.ConditionClass.Status {
			return true
		}
	}
	return false
}

//Returns true of the generation numbers are correct
func (r *Resource) generationNumbersMatch() bool {
	return r.Metadata.Generation > r.Status.ObservedGeneration
}

//Parses the unstructured source metadata into this Resource object's metadata map
func (r *Resource) parseMetadata() {
	rawMetadata := r.Source.Object["metadata"].(map[string]interface{})

	rawOwnerReferences := rawMetadata["ownerReferences"].([]interface{})
	var ownerReferences []string

	for _, ownerReference := range rawOwnerReferences {
		ownerReferences = append(ownerReferences, ownerReference.(map[string]interface{})["uid"].(string))
	}

	r.Metadata = ResourceMetadata{
		Generation:      rawMetadata["generation"].(int64),
		Namespace:       rawMetadata["namespace"].(string),
		Name:            rawMetadata["name"].(string),
		UID:             rawMetadata["uid"].(string),
		ResourceVersion: rawMetadata["resourceVersion"].(string),
		OwnerReferences: ownerReferences,
	}
}

func (r *Resource) interfaceMapHasKey(inMap map[string]interface{}, key string) bool {
	_, ok := inMap[key]
	return ok
}

//Parses a subset of the unstructures source status
func (r *Resource) parseStatus() {
	statusSource := r.Source.Object["status"].(map[string]interface{})

	//observed
	var observedGen int64

	if r.interfaceMapHasKey(statusSource, "observedGeneration") {
		observedGen = statusSource["observedGeneration"].(int64)
	}

	r.Status = ResourceStatus{
		ObservedGeneration: observedGen,
	}
}

//Parses the unstructured source metadata conditions into this Resource objects Conditions array of maps
func (r *Resource) parseStatusConditions() {
	status := r.Source.Object["status"].(map[string]interface{})

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

//Get a resource by a provided index
func (r *ResourceList) GetResourceByIndex(index int) Resource {
	return r.Resources[index]
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

//Get a count of the broken resources
func (r *ResourceList) CountBroken() int {
	count := 0
	for _, resource := range r.Resources {
		if !resource.IsReady() {
			count += 1
		}
	}
	return count
}

//Get a list of ready resources
func (r *ResourceList) GetReadyResources() []Resource {
	retVal := []Resource{}
	for _, resource := range r.Resources {
		if resource.IsReady() {
			retVal = append(retVal, resource)
		}
	}
	return retVal
}

//Get a list of broken resources
func (r *ResourceList) GetBrokenResources() []Resource {
	retVal := []Resource{}
	for _, resource := range r.Resources {
		if !resource.IsReady() {
			retVal = append(retVal, resource)
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

	r.source = *uList

	r.parseSource()

	return nil

}

//Parses the source unstructured.UnstructuredList into an array of Resources
func (r *ResourceList) parseSource() {
	for _, unstructured := range r.source.Items {
		resource := Resource{}
		resource.Parse(unstructured)
		r.Resources = append(r.Resources, resource)
	}
}

//Set the resource requirements for all of the resources in the list
func (r *ResourceList) SetReadyRequirements(reqs ResourceConditionReadyRequirements) {
	for _, resource := range r.Resources {
		resource.SetReadyRequirements(reqs)
	}
}

//Results that a resource counter provides back when its count method is called
type ResourceCounterResults struct {
	Managed       int
	Ready         int
	BrokenMessage string
}

//Represents a resource query for a count
//We count resources of a given GVK, in a given set of namespaces, owned by a given guid
type ResourceCounterQuery struct {
	GVK        schema.GroupVersionKind
	Namespaces []string
	OwnerGUID  string
}

/* GVK Examples
schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "Kafka",
	Version: "v1beta2",
}
GVK: schema.GroupVersionKind{
	Group:   "apps",
	Kind:    "Deployment",
	Version: "v1",
}
*/

//Provides a simple API for getting common figures on Resources and ResourceLists
//The Count method returns a ResourceCounterResults instance
type ResourceCounter struct {
	CountManaged      int
	CountReady        int
	BrokenLog         []string
	Query             ResourceCounterQuery
	ReadyRequirements ResourceConditionReadyRequirements
}

//Counts the resources
func (r *ResourceCounter) Count(ctx context.Context, pClient client.Client) ResourceCounterResults {
	for _, namespace := range r.Query.Namespaces {
		r.countInNamespace(ctx, pClient, namespace)
	}
	return ResourceCounterResults{
		Managed:       r.CountManaged,
		Ready:         r.CountReady,
		BrokenMessage: r.getBrokenMessage(),
	}
}

//Counts up the managed resources in a given namespace
func (r *ResourceCounter) countInNamespace(ctx context.Context, pClient client.Client, namespace string) {
	deployments := ResourceList{}
	deployments.GetByGVKAndNamespace(pClient, ctx, namespace, r.Query.GVK)

	deployments = deployments.FilterByOwnerUID(r.Query.OwnerGUID)

	deployments.SetReadyRequirements(r.ReadyRequirements)

	r.CountManaged += deployments.Count()
	r.CountReady += deployments.CountReady()
	r.generateBrokenLog(deployments.GetBrokenResources())
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
		retVal = fmt.Sprintf("broken deployments: [%s]", strings.Join(r.BrokenLog, ", "))
	}
	return retVal
}
