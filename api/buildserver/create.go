package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/pivotal-golang/lager"
)

func (s *Server) CreateBuild(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("create-build")

	var plan atc.Plan
	err := json.NewDecoder(r.Body).Decode(&plan)
	if err != nil {
		hLog.Info("malformed-request", lager.Data{"error": err.Error()})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//TODO: Test/Fill this in with something better ASAP!
	team, err := s.db.GetTeamByName(atc.DefaultTeamName)
	build, err := s.db.CreateOneOffBuild(team.ID)

	if err != nil {
		hLog.Error("failed-to-create-one-off-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	engineBuild, err := s.engine.CreateBuild(hLog, build, plan)
	if err != nil {
		hLog.Error("failed-to-start-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	go engineBuild.Resume(hLog)

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(present.Build(build))
}
