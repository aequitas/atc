package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

func Pipeline(savedPipeline db.SavedPipeline, config atc.Config) atc.Pipeline {

	pathForRoute, err := web.Routes.CreatePathForRoute(web.Pipeline, rata.Params{
		"pipeline":  savedPipeline.Name,
		"team_name": savedPipeline.TeamName,
	})

	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	return atc.Pipeline{
		Name:     savedPipeline.Name,
		TeamName: savedPipeline.TeamName,
		URL:      pathForRoute,
		Paused:   savedPipeline.Paused,
		Groups:   config.Groups,
	}
}
