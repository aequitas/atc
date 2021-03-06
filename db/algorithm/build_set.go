package algorithm

type BuildSet map[int]struct{}

func (set BuildSet) Contains(buildID int) bool {
	_, found := set[buildID]
	return found
}

func (set BuildSet) Union(otherSet BuildSet) BuildSet {
	newSet := BuildSet{}

	for buildID, _ := range set {
		newSet[buildID] = struct{}{}
	}

	for buildID, _ := range otherSet {
		newSet[buildID] = struct{}{}
	}

	return newSet
}

func (set BuildSet) Intersect(otherSet BuildSet) BuildSet {
	result := BuildSet{}

	for key, val := range set {
		_, found := otherSet[key]
		if found {
			result[key] = val
		}
	}

	return result
}

func (set BuildSet) Equal(otherSet BuildSet) bool {
	if len(set) != len(otherSet) {
		return false
	}

	for x, _ := range set {
		if !otherSet.Contains(x) {
			return false
		}
	}

	return true
}
