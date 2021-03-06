package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Volume(volume db.SavedVolume) atc.Volume {
	return atc.Volume{
		ID:                volume.Handle,
		TTLInSeconds:      int64(volume.ExpiresIn.Seconds()),
		ValidityInSeconds: int64(volume.TTL.Seconds()),
		ResourceVersion:   volume.ResourceVersion,
		WorkerName:        volume.WorkerName,
	}
}
