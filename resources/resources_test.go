package resources

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	rl.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
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

	rl.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, rl.Count(), rl.CountReady())
	assert.Equal(t, rl.CountBroken(), 0)
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

	rl.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.Equal(t, rl.Count(), len(uList.Items))
	assert.Equal(t, rl.CountReady(), len(uList.Items)/2)
	assert.Equal(t, rl.CountBroken(), len(uList.Items)/2)
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
	assert.Equal(t, rl.CountBroken(), rl.Count())
}

//We can't fully test this object
//Its primary public method requires talking to k8s - that's a
//huge part of its value. However, we can test everything the method does
func TestResourceCounterMixedMultipleNamespaces(t *testing.T) {
	rc := ResourceCounter{
		Query: ResourceCounterQuery{
			Namespaces: []string{"some-namespace", "some-other-namespace"},
			OwnerGUID:  GUID,
			GVK: schema.GroupVersionKind{
				Group:   "apps",
				Kind:    "Deployment",
				Version: "v1",
			},
		},
		ReadyRequirements: ResourceConditionReadyRequirements{
			Type:   "Available",
			Status: "Ready",
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

	rl.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
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
			GVK: schema.GroupVersionKind{
				Group:   "apps",
				Kind:    "Deployment",
				Version: "v1",
			},
		},
		ReadyRequirements: ResourceConditionReadyRequirements{
			Type:   "Available",
			Status: "Ready",
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

	rl.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	rc.countInNamespace(rl)

	assert.Equal(t, rc.CountManaged, 4)
	assert.Equal(t, rc.CountReady, 3)
	assert.Equal(t, len(rc.BrokenLog), 1)
}

func TestReadyDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentReady)
	r := Resource{}
	r.Parse(unstruct)
	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.True(t, r.IsReady())
}

func TestReadyDeploymentWithWrongReadyRequirements(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentReady)

	r := Resource{}
	r.Parse(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "What?",
	})

	assert.False(t, r.IsReady())
}

func TestBadConditionsDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentBadConditions)

	r := Resource{}
	r.Parse(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.False(t, r.IsReady())
}

func TestNoConditionsDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentNoConditions)

	r := Resource{}
	r.Parse(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.False(t, r.IsReady())
}

func TestBadGenerationDeployment(t *testing.T) {
	unstruct := ConvertJSONToUnstructured(JSONDeploymentBadGeneration)

	r := Resource{}
	r.Parse(unstruct)

	r.AddReadyRequirements(ResourceConditionReadyRequirements{
		Type:   "Available",
		Status: "Ready",
	})

	assert.False(t, r.IsReady())
}
