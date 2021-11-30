package conditionhandler

import (
	"fmt"
	"testing"
	"time"

	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionSearch(t *testing.T) {

	var Bing = "Bing"
	var Boo = "Boo"
	var Bloop = "Bloop"

	conditions := []v1.Condition{{
		Type:    Bing,
		Status:  v1.ConditionTrue,
		Reason:  "Not sure",
		Message: "Help",
	}}

	idx, cond := GetCondition(&conditions, Bing)
	assert.Equal(t, idx, 0)
	assert.Equal(t, cond.Type, Bing)

	idx, _ = GetCondition(&[]v1.Condition{}, Boo)
	assert.Equal(t, idx, -1)

	stTime := v1.Now()
	fmt.Printf("ST%v\n", stTime)

	update := UpdateCondition(&conditions, &v1.Condition{
		Type:    Bloop,
		Status:  v1.ConditionFalse,
		Reason:  "I'm sure",
		Message: "You failed",
	})

	idx, cond = GetCondition(&conditions, Bloop)
	assert.Equal(t, idx, 1)
	assert.Equal(t, cond.Status, v1.ConditionFalse)
	assert.Equal(t, update, true)

	time.Sleep(time.Second * 2)

	update = UpdateCondition(&conditions, &v1.Condition{
		Type:    Bloop,
		Status:  v1.ConditionFalse,
		Reason:  "I'm sure",
		Message: "You failed",
	})

	idx, cond = GetCondition(&conditions, Bloop)
	assert.Equal(t, idx, 1)
	assert.Equal(t, update, false)
	duration := cond.LastTransitionTime.Sub(stTime.Time)
	assert.Check(t, duration < time.Second)

	UpdateCondition(&conditions, &v1.Condition{
		Type:               Bloop,
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.Time{},
		Reason:             "I'm sure",
		Message:            "You failed",
	})

	time.Sleep(time.Second * 2)

	idx, cond = GetCondition(&conditions, Bloop)
	assert.Equal(t, idx, 1)
	duration = cond.LastTransitionTime.Sub(stTime.Time)
	assert.Check(t, duration > time.Second*2)
	assert.Check(t, duration < time.Second*5)

}
