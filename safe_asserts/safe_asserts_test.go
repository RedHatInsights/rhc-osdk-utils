package safe_asserts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetString(t *testing.T) {
	sourceMap := map[string]interface{}{
		"key": "value",
	}
	outString, success := GetString(sourceMap, "key")
	assert.True(t, success)
	assert.Equal(t, "value", outString)

}

func TestGetStringBadKey(t *testing.T) {
	sourceMap := map[string]interface{}{
		"jomo": "value",
	}
	_, success := GetString(sourceMap, "key")
	assert.False(t, success)

}

func TestGetInterfaceList(t *testing.T) {
	sourceInterface := map[string]interface{}{
		"jomo": []interface{}{1, 2, 3},
	}
	_, success := GetInterfaceList(sourceInterface, "key")
	assert.False(t, success)
}

func TestGetInterfaceListBadKey(t *testing.T) {
	sourceInterface := map[string]interface{}{
		"key": []interface{}{1, 2, 3},
	}
	outList, success := GetInterfaceList(sourceInterface, "key")
	assert.True(t, success)
	assert.Equal(t, outList[0], 1)
}

func TestToMap(t *testing.T) {
	sourceJSON := []byte(`{"vals": [{"testKey":"testValue"}]}`)
	var testInterface interface{}
	json.Unmarshal(sourceJSON, &testInterface)
	outMap, success := ToMap(testInterface)
	assert.True(t, success)
	assert.Equal(t, outMap["vals"].([]interface{})[0].(map[string]interface{})["testKey"], "testValue")
}

func TestToMapBadAssert(t *testing.T) {
	_, success := ToMap(map[string]string{"wont": "work"})
	assert.False(t, success)
}

func TestGetMap(t *testing.T) {
	sourceJSON := []byte(`{"vals": [{"testKey":"testValue"}]}`)
	var testInterface interface{}
	json.Unmarshal(sourceJSON, &testInterface)
	sourceMap := map[string]interface{}{
		"test": testInterface,
	}
	outMap, success := GetMap(sourceMap, "test")
	assert.True(t, success)
	assert.Equal(t, outMap["vals"].([]interface{})[0].(map[string]interface{})["testKey"], "testValue")
}

func TestGetMapBadKey(t *testing.T) {
	sourceJSON := []byte(`{"jomo": [{"testKey":"testValue"}]}`)
	var testInterface interface{}
	json.Unmarshal(sourceJSON, &testInterface)
	sourceMap := map[string]interface{}{
		"quozo": testInterface,
	}
	_, success := GetMap(sourceMap, "test")
	assert.False(t, success)
}

func TestGetInt64(t *testing.T) {
	sourceMap := map[string]interface{}{
		"key": int64(1979),
	}
	outInt64, success := GetInt64(sourceMap, "key", 0)
	assert.True(t, success)
	assert.Equal(t, int64(1979), outInt64)
}

func TestGetInt64DefaultValue(t *testing.T) {
	sourceMap := map[string]interface{}{
		"jomo": int64(1979),
	}
	outInt64, success := GetInt64(sourceMap, "key", -1)
	assert.False(t, success)
	assert.Equal(t, int64(-1), outInt64)
}
