package safe_asserts

/*
There are manyaces in this library, specifically in the resource code, that
we need to perform type assertions. These assertions need to be hanled safely,
to avoid panics, and must handle various cases like pulling from complex
structures, default values, etc.
*/

//Gets a string value from an interface map.
func GetString(sourceMap map[string]interface{}, key string) (string, bool) {
	outString := ""
	assertedString, assertionSuccess := sourceMap[key].(string)
	if assertionSuccess {
		outString = assertedString
	}
	return outString, assertionSuccess
}

//Gets an interface list from an interface map.
func GetInterfaceList(sourceInterface map[string]interface{}, key string) ([]interface{}, bool) {
	outList := []interface{}{}
	success := false
	value, valueExists := sourceInterface[key]
	if valueExists {
		assertedList, assertionSuccess := value.([]interface{})
		if assertionSuccess {
			outList = assertedList
			success = assertionSuccess
		}
	}

	return outList, success
}

//Converts a an interface to an interface map.
func ToMap(sourceInterface interface{}) (map[string]interface{}, bool) {
	outMap := map[string]interface{}{}
	success := false
	assertedMap, assertionSuccess := sourceInterface.(map[string]interface{})
	if assertionSuccess {
		outMap = assertedMap
		success = assertionSuccess
	}
	return outMap, success
}

//Gets an interface map from an interface map
func GetMap(sourceInterface map[string]interface{}, key string) (map[string]interface{}, bool) {
	outMap := map[string]interface{}{}
	success := false
	value, valueExists := sourceInterface[key]
	if valueExists {
		outMap, success = ToMap(value)
	}
	return outMap, success
}

//Gets an int64 from an interface map.
func GetInt64(sourceInterface map[string]interface{}, key string, defaultVal int64) (int64, bool) {
	outInt64 := defaultVal
	success := false
	value, valueExists := sourceInterface[key]
	if valueExists {
		assertedInt64, assertionSuccess := value.(int64)
		if assertionSuccess {
			outInt64 = assertedInt64
			success = assertionSuccess
		}
	}
	return outInt64, success
}
