package conditionhandler

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The following function was modified from the kubnernetes repo under the apache license here
// https://github.com/kubernetes/kubernetes/blob/v1.21.1/pkg/api/v1/pod/util.go#L317-L367
func GetConditionFromList(conditions *[]v1.Condition, conditionType string) (int, *v1.Condition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range *conditions {
		if (*conditions)[i].Type == conditionType {
			return i, &(*conditions)[i]
		}
	}
	return -1, nil
}

// The following function was modified from the kubnernetes repo under the apache license here
// https://github.com/kubernetes/kubernetes/blob/v1.21.1/pkg/api/v1/pod/util.go#L317-L367
func GetCondition(conditions *[]v1.Condition, conditionType string) (int, *v1.Condition) {
	if len(*conditions) == 0 {
		return -1, nil
	}
	return GetConditionFromList(conditions, conditionType)
}

// The following function was modified from the kubnernetes repo under the apache license here
// https://github.com/kubernetes/kubernetes/blob/v1.21.1/pkg/api/v1/pod/util.go#L317-L367
func UpdateCondition(conditions *[]v1.Condition, condition *v1.Condition) bool {
	condition.LastTransitionTime = v1.Now()
	// Try to find this clowdapp condition.
	conditionIndex, oldCondition := GetCondition(conditions, condition.Type)

	if oldCondition == nil {
		// We are adding new pod condition.
		*conditions = append(*conditions, *condition)
		return true
	}
	// We are updating an existing condition, so we need to check if it has changed.
	if condition.Status == oldCondition.Status {
		condition.LastTransitionTime = oldCondition.LastTransitionTime
	}

	isEqual := condition.Status == oldCondition.Status &&
		condition.Reason == oldCondition.Reason &&
		condition.Message == oldCondition.Message &&
		condition.LastTransitionTime.Equal(&oldCondition.LastTransitionTime)

	(*conditions)[conditionIndex] = *condition
	// Return true if one of the fields have changed.
	return !isEqual
}
