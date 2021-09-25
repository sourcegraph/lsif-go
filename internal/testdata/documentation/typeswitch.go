package testdata

func Switch(interfaceValue interface{}) bool {
	switch concreteValue := interfaceValue.(type) {
	case int:
		return concreteValue*3 > 10
	case bool:
		return !concreteValue
	default:
		return false
	}
}
