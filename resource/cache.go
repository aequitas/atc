package resource

import "github.com/concourse/baggageclaim"

const initializedProperty = "initialized"

type noopCache struct{}

func (noopCache) IsInitialized() (bool, error) { return false, nil }
func (noopCache) Initialize() error            { return nil }

type volumeCache struct {
	volume baggageclaim.Volume
}

func (cache volumeCache) IsInitialized() (bool, error) {
	props, err := cache.volume.Properties()
	if err != nil {
		return false, err
	}

	_, found := props[initializedProperty]
	return found, nil
}

func (cache volumeCache) Initialize() error {
	return cache.volume.SetProperty("initialized", "yep")
}
