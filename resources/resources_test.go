package resources

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	GUID = "5656-5656-5656-5656"
)

var JSONDeploymentReady = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 1,
		"namespace": "some-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "` + GUID + `"},
			{"uid": "2323-2323-2323-2323"}
		]
	},
	"status": {
		"observedGeneration": 1,
		"conditions": [
			{"status": "Ready", "type": "Available", "reason": "The new Bloc Party record is dope."},
			{"status": "Yeah", "type": "Happy", "reason": "It has upbeat The Cure vibes for sure"}
		]
	}
}
`

var JSONDeploymentBadConditions = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 1,
		"namespace": "some-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "2323-2323-2323-2323"}
		]
	},
	"status": {
		"observedGeneration": 2,
		"conditions": [
			{"status": "NOTREADY", "type": "Available", "reason": "Its called Alpha Games and as of this code"},
			{"status": "Yeah", "type": "Happy", "reason": "It just came out in the last week"}
		]
	}
}
`

var JSONDeploymentNoConditions = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 1,
		"namespace": "some-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "2323-2323-2323-2323"}
		]
	},
	"status": {
		"observedGeneration": 2,
		"conditions": []
	}
}
`

var JSONDeploymentBadGeneration = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 4,
		"namespace": "some-other-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "` + GUID + `"},
			{"uid": "2323-2323-2323-2323"}
		]
	},
	"status": {
		"observedGeneration": 1,
		"conditions": [
			{"status": "Ready", "type": "Available", "reason": "I liked Hymns. It was different. And its gospel influences were thought provoking."},
			{"status": "Yeah", "type": "Happy", "reason": "But Alpha Games better captures that hectic early 00's London indie rock sound."}
		]
	}
}
`

var JSONDeploymentNoReason = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 4,
		"namespace": "some-other-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "` + GUID + `"},
			{"uid": "2323-2323-2323-2323"}
		]
	},
	"status": {
		"observedGeneration": 1,
		"conditions": [
			{"status": "Ready", "type": "Available"},
			{"status": "Yeah", "type": "Happy", "reason": "But Alpha Games better captures that hectic early 00's London indie rock sound."}
		]
	}
}
`

var JSONDeploymentConditionNoStatus = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 4,
		"namespace": "some-other-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "` + GUID + `"},
			{"uid": "2323-2323-2323-2323"}
		]
	},
	"status": {
		"observedGeneration": 1,
		"conditions": [
			{"jomo": "Ready", "type": "Available"},
			{"jomo": "Yeah", "type": "Happy", "reason": "But Alpha Games better captures that hectic early 00's London indie rock sound."}
		]
	}
}
`

var JSONDeploymentConditionNoType = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 4,
		"namespace": "some-other-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "` + GUID + `"},
			{"uid": "2323-2323-2323-2323"}
		]
	},
	"status": {
		"observedGeneration": 1,
		"conditions": [
			{"status": "Ready", "fromo": "Available"},
			{"status": "Yeah", "fromo": "Happy", "reason": "But Alpha Games better captures that hectic early 00's London indie rock sound."}
		]
	}
}
`

var JSONDeploymentNoStatus = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"generation": 4,
		"namespace": "some-other-namespace",
		"name": "some-resource",
		"uid": "1212-1212-1212-1212",
		"resourceVersion": "1",
		"ownerReferences" : [
			{"uid": "` + GUID + `"},
			{"uid": "2323-2323-2323-2323"}
		]
	},
}
`

var JSONDeploymentNoMetadata = `
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"status": {
		"observedGeneration": 1,
		"conditions": [
			{"status": "Ready", "type": "Available"},
			{"status": "Yeah", "type": "Happy", "reason": "But Alpha Games better captures that hectic early 00's London indie rock sound."}
		]
	}
}
`

func TestDeploymentConditionWithNoStatus(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentConditionNoStatus))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, 0, rl.CountReady())
}

func TestDeploymentConditionWithNoType(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentConditionNoType))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, 0, rl.CountReady())
}

func TestDeploymentWithNoMetadata(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentNoMetadata))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, 1, rl.CountReady())
}

func TestConditionWithNoReason(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentNoReason))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, 0, rl.CountReady())
}

func TestDeploymentWithNoStatus(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentNoStatus))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, 0, rl.CountReady())
}

func TestMakeQueryUnregisteredType(t *testing.T) {
	obj := v1.Deployment{}
	scheme := runtime.NewScheme()
	namespaces := []string{"test"}
	var uid types.UID = "1234"

	_, err := MakeQuery(&obj, *scheme, namespaces, uid)

	assert.Error(t, err)
}

func TestMakeQueryRegisteredType(t *testing.T) {
	obj := v1.Deployment{}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(CommonGVKs.Deployment.GroupVersion(), &obj)
	namespaces := []string{"test"}
	var uid types.UID = "1234"

	query, err := MakeQuery(&obj, *scheme, namespaces, uid)

	assert.Nil(t, err)
	assert.Equal(t, query.GVK.Kind, "Deployment")
}

//A lot of our methods care about unstructured.Unstructured, so we need to be able to produce those
//in various states for tests. Thankfully we can unmarshall them from JSON!
func ConvertJSONToUnstructured(jsonInput string) unstructured.Unstructured {

	data := unstructured.Unstructured{}

	if err := json.Unmarshal([]byte(jsonInput), &data); err != nil {
		// handle error
		fmt.Printf("Oops, something went wrong: %+v", err)
		return data
	}

	return data
}

func TestFilterResourceListByGUID(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadConditions))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadGeneration))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentNoConditions))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	filteredList := rl.FilterByOwnerUID(GUID)

	assert.Equal(t, 4, len(filteredList.Resources))
}

func TestReadyResourceList(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, rl.Count(), rl.CountReady())
	assert.Equal(t, rl.Count()-rl.CountReady(), 0)
}

func TestMixedResourceList(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadConditions))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadGeneration))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentNoConditions))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, rl.CountReady(), len(uList.Items)/2)
	assert.Equal(t, rl.Count()-rl.CountReady(), len(uList.Items)/2)
}

func TestReadyResourceListNoReadyRequirements(t *testing.T) {
	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, rl.CountReady(), 0)
	assert.Equal(t, rl.Count()-rl.CountReady(), rl.Count())
}

//We can't fully test this object
//Its primary public method requires talking to k8s - that's a
//huge part of its value. However, we can test everything the method does
func TestResourceCounterMixedMultipleNamespaces(t *testing.T) {

	rc := ResourceCounter{
		Query: ResourceCounterQuery{
			Namespaces: []string{"some-namespace", "some-other-namespace"},
			OwnerGUID:  GUID,
			GVK:        CommonGVKs.Deployment,
		},
		ReadyRequirements: []ResourceConditionReadyRequirements{
			{
				Type:   "Available",
				Status: "Ready",
			},
		},
	}

	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadConditions))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadGeneration))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentNoConditions))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	rc.countInNamespace(rl)

	assert.Equal(t, rc.CountManaged, 4)
	assert.Equal(t, rc.CountReady, 3)
	assert.Equal(t, rc.BrokenLog[0], "some-resource/some-other-namespace")
}

func TestResourceCounterMixedSingleNamespaces(t *testing.T) {
	rc := ResourceCounter{
		Query: ResourceCounterQuery{
			Namespaces: []string{"some-namespace"},
			OwnerGUID:  GUID,
			GVK:        CommonGVKs.Deployment,
		},
		ReadyRequirements: []ResourceConditionReadyRequirements{
			{
				Type:   "Available",
				Status: "Ready",
			},
		},
	}

	uList := unstructured.UnstructuredList{}

	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentReady))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadConditions))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentBadGeneration))
	uList.Items = append(uList.Items, ConvertJSONToUnstructured(JSONDeploymentNoConditions))

	rl := ResourceList{}
	rl.SetListAndParse(uList)

	rl.AddReadyRequirementsFromSlice([]ResourceConditionReadyRequirements{
		{
			Type:   "Available",
			Status: "Ready",
		},
	})

	rc.countInNamespace(rl)

	assert.Equal(t, rc.CountManaged, 4)
	assert.Equal(t, rc.CountReady, 3)
	assert.Equal(t, len(rc.BrokenLog), 1)
}

func TestReadyDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentReady)
	r := MakeResource(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.True(t, r.IsReady())
}

func TestReadyDeploymentWithWrongReadyRequirements(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentReady)

	r := MakeResource(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "What?",
	})

	assert.False(t, r.IsReady())
}

func TestBadConditionsDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentBadConditions)

	r := MakeResource(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.False(t, r.IsReady())
}

func TestNoConditionsDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentNoConditions)

	r := MakeResource(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.False(t, r.IsReady())
}

func TestBadGenerationDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentBadGeneration)

	r := MakeResource(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.False(t, r.IsReady())
}
