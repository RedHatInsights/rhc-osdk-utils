package safe_asserts

//We perform many type asserts in this code as we pull values from the unstructured
//objects. We need to handle these safely so as to avoid panics

func GetString(sourceMap map[string]interface{}, key string) (string, bool) {
	outString := ""
	assertedString, assertionSuccess := sourceMap[key].(string)
	if assertionSuccess {
		outString = assertedString
	}
	return outString, assertionSuccess
}

func GetInterfaceList(sourceInterface map[string]interface{}, key string) ([]interface{}, bool) {
	outList := []interface{}{}
	assertedList, assertionSuccess := sourceInterface[key].([]interface{})
	if assertionSuccess {
		outList = assertedList
	}
	return outList, assertionSuccess
}
