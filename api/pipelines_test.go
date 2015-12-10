package api_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	dbfakes "github.com/concourse/atc/db/fakes"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
)

var _ = Describe("Pipelines API", func() {
	Describe("GET /api/v1/teams/:team_name/pipelines", func() {
		var response *http.Response

		BeforeEach(func() {
			pipelinesDB.GetAllActivePipelinesReturns([]db.SavedPipeline{
				{
					ID:       1,
					Paused:   false,
					TeamName: "some-team",
					Pipeline: db.Pipeline{
						Name: "a-pipeline",
						Config: atc.Config{
							Groups: atc.GroupConfigs{
								{
									Name:      "group1",
									Jobs:      []string{"job1", "job2"},
									Resources: []string{"resource1", "resource2"},
								},
							},
						},
					},
				},
				{
					ID:       2,
					Paused:   true,
					TeamName: "some-team",
					Pipeline: db.Pipeline{
						Name: "another-pipeline",
						Config: atc.Config{
							Groups: atc.GroupConfigs{
								{
									Name:      "group2",
									Jobs:      []string{"job3", "job4"},
									Resources: []string{"resource3", "resource4"},
								},
							},
						},
					},
				},
			}, nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/teams/some-team/pipelines", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 OK", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns application/json", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		It("returns all active pipelines", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(`[
      {
        "name": "a-pipeline",
        "url": "/teams/some-team/pipelines/a-pipeline",
				"paused": false,
				"team_name": "some-team",
				"groups": [
					{
						"name": "group1",
						"jobs": ["job1", "job2"],
						"resources": ["resource1", "resource2"]
					}
				]
      },{
        "name": "another-pipeline",
        "url": "/teams/some-team/pipelines/another-pipeline",
				"paused": true,
				"team_name": "some-team",
				"groups": [
					{
						"name": "group2",
						"jobs": ["job3", "job4"],
						"resources": ["resource3", "resource4"]
					}
				]
      }]`))
		})

		Context("when the call to get active pipelines fails", func() {
			BeforeEach(func() {
				pipelinesDB.GetAllActivePipelinesReturns(nil, errors.New("disaster"))
			})

			It("returns 500 internal server error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name", func() {
		var response *http.Response

		BeforeEach(func() {
			pipelinesDB.GetPipelineByTeamNameAndNameReturns(db.SavedPipeline{
				ID:       1,
				Paused:   false,
				TeamName: "some-team",
				Pipeline: db.Pipeline{
					Name: "some-specific-pipeline",
				},
			}, nil)

			configDB.GetConfigReturns(atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "group1",
						Jobs:      []string{"job1", "job2"},
						Resources: []string{"resource1", "resource2"},
					},
					{
						Name:      "group2",
						Jobs:      []string{"job3", "job4"},
						Resources: []string{"resource3", "resource4"},
					},
				},
			}, 42, nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/teams/some-team/pipelines/some-specific-pipeline", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 ok", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns application/json", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		It("returns a pipepine JSON", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(`
      {
        "name": "some-specific-pipeline",
        "url": "/teams/some-team/pipelines/some-specific-pipeline",
				"team_name": "some-team",
				"paused": false,
				"groups": [
					{
						"name": "group1",
						"jobs": ["job1", "job2"],
						"resources": ["resource1", "resource2"]
					},
					{
						"name": "group2",
						"jobs": ["job3", "job4"],
						"resources": ["resource3", "resource4"]
					}
				]
      }`))
		})

		Context("when the call to get pipeline fails", func() {
			BeforeEach(func() {
				pipelinesDB.GetPipelineByTeamNameAndNameReturns(db.SavedPipeline{}, errors.New("disaster"))
			})

			It("returns 500 error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when the call to get the pipeline config fails", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{}, 0, errors.New("disaster"))
			})

			It("returns 500 error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		It("looks up the pipeline in the db via the url param", func() {
			Expect(pipelinesDB.GetPipelineByTeamNameAndNameCallCount()).To(Equal(1))

			teamName, actualPipelineName := pipelinesDB.GetPipelineByTeamNameAndNameArgsForCall(0)
			Expect(actualPipelineName).To(Equal("some-specific-pipeline"))
			Expect(teamName).To(Equal("some-team"))
		})
	})

	Describe("DELETE /api/v1/teams/:team_name/pipelines/:pipeline_name", func() {
		var response *http.Response
		var pipelineDB *dbfakes.FakePipelineDB

		BeforeEach(func() {
			pipelineDB = new(dbfakes.FakePipelineDB)

			pipelineDBFactory.BuildWithTeamNameAndNameReturns(pipelineDB, nil)
		})

		JustBeforeEach(func() {
			pipelineName := "a-pipeline-name"
			req, err := http.NewRequest("DELETE", server.URL+"/api/v1/teams/some-team/pipelines/"+pipelineName, nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the user is logged in", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("returns 204 No Content", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNoContent))
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline-name"))
				Expect(teamName).To(Equal("some-team"))
			})

			It("deletes the named pipeline from the database", func() {
				Expect(pipelineDB.DestroyCallCount()).To(Equal(1))
			})

			Context("when an error occurs destroying the pipeline", func() {
				BeforeEach(func() {
					err := errors.New("disaster!")
					pipelineDB.DestroyReturns(err)
				})

				It("returns a 500 Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when the user is not logged in", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/pause", func() {
		var response *http.Response
		var pipelineDB *dbfakes.FakePipelineDB

		BeforeEach(func() {
			pipelineDB = new(dbfakes.FakePipelineDB)

			pipelineDBFactory.BuildWithTeamNameAndNameReturns(pipelineDB, nil)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/a-pipeline/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(teamName).To(Equal("some-team"))
			})

			Context("when pausing the pipeline succeeds", func() {
				BeforeEach(func() {
					pipelineDB.PauseReturns(nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when pausing the pipeline fails", func() {
				BeforeEach(func() {
					pipelineDB.PauseReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/unpause", func() {
		var response *http.Response
		var pipelineDB *dbfakes.FakePipelineDB

		BeforeEach(func() {
			pipelineDB = new(dbfakes.FakePipelineDB)

			pipelineDBFactory.BuildWithTeamNameAndNameReturns(pipelineDB, nil)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/a-pipeline/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
				Expect(teamName).To(Equal("some-team"))
			})

			Context("when unpausing the pipeline succeeds", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseReturns(nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when unpausing the pipeline fails", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/pipelines/ordering", func() {
		var response *http.Response
		var body io.Reader

		BeforeEach(func() {
			body = bytes.NewBufferString(`
				[
					"a-pipeline",
					"another-pipeline",
					"yet-another-pipeline",
					"one-final-pipeline",
					"just-kidding"
				]
			`)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/ordering", body)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("with invalid json", func() {
				BeforeEach(func() {
					body = bytes.NewBufferString(`{}`)
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("when ordering the pipelines succeeds", func() {
				BeforeEach(func() {
					pipelinesDB.OrderPipelinesReturns(nil)
				})

				It("orders the pipelines", func() {
					Expect(pipelinesDB.OrderPipelinesCallCount()).To(Equal(1))
					pipelineNames := pipelinesDB.OrderPipelinesArgsForCall(0)
					Expect(pipelineNames).To(Equal(
						[]string{
							"a-pipeline",
							"another-pipeline",
							"yet-another-pipeline",
							"one-final-pipeline",
							"just-kidding",
						},
					))

				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when ordering the pipelines fails", func() {
				BeforeEach(func() {
					pipelinesDB.OrderPipelinesReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/versions-db", func() {
		var response *http.Response
		var pipelineDB *dbfakes.FakePipelineDB

		BeforeEach(func() {
			pipelineDB = new(dbfakes.FakePipelineDB)

			pipelineDBFactory.BuildWithTeamNameAndNameReturns(pipelineDB, nil)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("GET", server.URL+"/api/v1/teams/some-team/pipelines/a-pipeline/versions-db", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				//construct Version db

				pipelineDB.LoadVersionsDBReturns(
					&algorithm.VersionsDB{
						ResourceVersions: []algorithm.ResourceVersion{
							{
								VersionID:  73,
								ResourceID: 127,
							},
						},
						BuildOutputs: []algorithm.BuildOutput{
							{
								ResourceVersion: algorithm.ResourceVersion{
									VersionID:  73,
									ResourceID: 127,
								},
								BuildID: 66,
								JobID:   13,
							},
						},
						JobIDs: map[string]int{
							"bad-luck-job": 13,
						},
						ResourceIDs: map[string]int{
							"resource-127": 127,
						},
						CachedAt: time.Unix(42, 0).UTC(),
					},
					nil,
				)
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns application/json", func() {
				Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns a json representation of all the versions in the pipeline", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{
				"ResourceVersions": [
					{
						"VersionID": 73,
						"ResourceID": 127
			    }
				],
				"BuildOutputs": [
					{
						"VersionID": 73,
						"ResourceID": 127,
						"BuildID": 66,
						"JobID": 13
					}
				],
				"JobIDs": {
						"bad-luck-job": 13
				},
				"ResourceIDs": {
					"resource-127": 127
				},
				"CachedAt": "1970-01-01T00:00:42Z"
				}`))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
