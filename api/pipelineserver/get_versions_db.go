package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/db"
)

func (s *Server) GetVersionsDB(pipelineDB db.PipelineDB) http.Handler {
	// logger := s.logger.Session("get-versions-db-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionsDB, _ := pipelineDB.LoadVersionsDB()
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(versionsDB)
	})

}
