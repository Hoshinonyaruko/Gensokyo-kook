package helper

func SliceContains[T comparable](array []T, target T) bool {
	for _, item := range array {
		if item == target {
			return true
		}
	}
	return false
}
