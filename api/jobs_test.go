package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	dbfakes "github.com/concourse/atc/db/fakes"
	schedulerfakes "github.com/concourse/atc/scheduler/fakes"
)

var _ = Describe("Jobs API", func() {
	var pipelineDB *dbfakes.FakePipelineDB

	BeforeEach(func() {
		pipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDBFactory.BuildWithTeamNameAndNameReturns(pipelineDB, nil)
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job")
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
			teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
			Expect(pipelineName).To(Equal("some-pipeline"))
			Expect(teamName).To(Equal("some-team"))
		})

		Context("when getting the job config succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetConfigReturns(atc.Config{
					Groups: []atc.GroupConfig{
						{
							Name: "group-1",
							Jobs: []string{"some-job"},
						},
						{
							Name: "group-2",
							Jobs: []string{"some-job"},
						},
					},

					Jobs: []atc.JobConfig{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get: "some-input",
								},
								{
									Get:      "some-name",
									Resource: "some-other-input",
									Params:   atc.Params{"secret": "params"},
									Passed:   []string{"a", "b"},
									Trigger:  true,
								},
								{
									Put: "some-output",
								},
								{
									Put:    "some-other-output",
									Params: atc.Params{"secret": "params"},
								},
							},
						},
					},
				}, 1, true, nil)
			})

			Context("when getting the build succeeds", func() {
				BeforeEach(func() {
					pipelineDB.GetJobFinishedAndNextBuildReturns(
						&db.Build{
							ID:           1,
							Name:         "1",
							JobName:      "some-job",
							PipelineName: "some-pipeline",
							TeamName:     "some-team",
							Status:       db.StatusSucceeded,
							StartTime:    time.Unix(1, 0),
							EndTime:      time.Unix(100, 0),
						},
						&db.Build{
							ID:           3,
							Name:         "2",
							JobName:      "some-job",
							PipelineName: "some-pipeline",
							TeamName:     "some-team",
							Status:       db.StatusStarted,
						},
						nil,
					)
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{}, errors.New("nope"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{
							ID:           1,
							Paused:       true,
							PipelineName: "some-pipeline",
							TeamName:     "some-team",
							Job: db.Job{
								Name: "job-1",
							},
						}, nil)
					})

					It("fetches by job", func() {
						Expect(pipelineDB.GetJobFinishedAndNextBuildCallCount()).To(Equal(1))

						jobName := pipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)
						Expect(jobName).To(Equal("some-job"))
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns the job's name, url, if it's paused, and any running and finished builds", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
							"name": "some-job",
							"paused": true,
							"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job",
							"next_build": {
								"id": 3,
								"name": "2",
								"job_name": "some-job",
								"status": "started",
								"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/2",
								"api_url": "/api/v1/builds/3",
								"pipeline_name":"some-pipeline",
								"team_name": "some-team"
							},
							"finished_build": {
								"id": 1,
								"name": "1",
								"job_name": "some-job",
								"status": "succeeded",
								"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/1",
								"api_url": "/api/v1/builds/1",
								"pipeline_name":"some-pipeline",
								"start_time": 1,
								"end_time": 100,
								"team_name": "some-team"
							},
							"inputs": [
								{
									"name": "some-input",
									"resource": "some-input",
									"trigger": false
								},
								{
									"name": "some-name",
									"resource": "some-other-input",
									"passed": ["a", "b"],
									"trigger": true
								}
							],
							"outputs": [
								{
									"name": "some-output",
									"resource": "some-output"
								},
								{
									"name": "some-other-output",
									"resource": "some-other-output"
								}
							],
							"groups": ["group-1", "group-2"]
						}`))

					})
				})

				Context("when there are no running or finished builds", func() {
					BeforeEach(func() {
						pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)
					})

					It("returns null as their entries", func() {
						var job atc.Job
						err := json.NewDecoder(response.Body).Decode(&job)
						Expect(err).NotTo(HaveOccurred())

						Expect(job.NextBuild).To(BeNil())
						Expect(job.FinishedBuild).To(BeNil())
					})
				})
			})

			Context("when getting the job's builds fails", func() {
				BeforeEach(func() {
					pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the job is not present in the config", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{
						Jobs: []atc.JobConfig{
							{Name: "other-job"},
						},
					}, 1, true, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when getting the job config fails", func() {
			Context("when the pipeline is no longer configured", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("with an unknown error", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{}, 0, false, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting the job config succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetConfigReturns(atc.Config{
					Groups: []atc.GroupConfig{
						{
							Name: "group-1",
							Jobs: []string{"job-1"},
						},
						{
							Name: "group-2",
							Jobs: []string{"job-1", "job-2"},
						},
					},

					Jobs: []atc.JobConfig{
						{
							Name: "job-1",
							Plan: atc.PlanSequence{{Get: "input-1"}, {Put: "output-1"}},
						},
						{
							Name: "job-2",
							Plan: atc.PlanSequence{{Get: "input-2"}, {Put: "output-2"}},
						},
						{
							Name: "job-3",
							Plan: atc.PlanSequence{{Get: "input-3"}, {Put: "output-3"}},
						},
					},
				}, 1, true, nil)
			})

			Context("when getting each job's builds succeeds", func() {
				BeforeEach(func() {
					call := 0

					pipelineDB.GetJobFinishedAndNextBuildStub = func(jobName string) (*db.Build, *db.Build, error) {
						call++

						var finishedBuild, nextBuild *db.Build

						switch call {
						case 1:
							Expect(jobName).To(Equal("job-1"))

							finishedBuild = &db.Build{
								ID:           1,
								Name:         "1",
								JobName:      jobName,
								PipelineName: "another-pipeline",
								TeamName:     "some-team",
								Status:       db.StatusSucceeded,
								StartTime:    time.Unix(1, 0),
								EndTime:      time.Unix(100, 0),
							}

							nextBuild = &db.Build{
								ID:           3,
								Name:         "2",
								JobName:      jobName,
								PipelineName: "another-pipeline",
								TeamName:     "some-team",
								Status:       db.StatusStarted,
							}

						case 2:
							Expect(jobName).To(Equal("job-2"))

							finishedBuild = &db.Build{
								ID:           4,
								Name:         "1",
								JobName:      "job-2",
								PipelineName: "another-pipeline",
								TeamName:     "some-team",
								Status:       db.StatusSucceeded,
								StartTime:    time.Unix(101, 0),
								EndTime:      time.Unix(200, 0),
							}

						case 3:
							Expect(jobName).To(Equal("job-3"))

						default:
							panic("unexpected call count")
						}

						return finishedBuild, nextBuild, nil
					}
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{}, errors.New("nope"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{
							ID:           1,
							Paused:       true,
							PipelineName: "another-pipeline",
							TeamName:     "some-team",
							Job: db.Job{
								Name: "job-1",
							},
						}, nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns each job's name, url, and any running and finished builds", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"name": "job-1",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1",
								"next_build": {
									"id": 3,
									"name": "2",
									"job_name": "job-1",
									"status": "started",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1/builds/2",
									"api_url": "/api/v1/builds/3",
									"pipeline_name":"another-pipeline",
									"team_name": "some-team"
								},
								"finished_build": {
									"id": 1,
									"name": "1",
									"job_name": "job-1",
									"status": "succeeded",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1/builds/1",
									"api_url": "/api/v1/builds/1",
									"pipeline_name":"another-pipeline",
									"start_time": 1,
									"end_time": 100,
									"team_name": "some-team"
								},
								"inputs": [{"name": "input-1", "resource": "input-1", "trigger": false}],
								"outputs": [{"name": "output-1", "resource": "output-1"}],
								"groups": ["group-1", "group-2"]
							},
							{
								"name": "job-2",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-2",
								"next_build": null,
								"finished_build": {
									"id": 4,
									"name": "1",
									"job_name": "job-2",
									"status": "succeeded",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-2/builds/1",
									"api_url": "/api/v1/builds/4",
									"pipeline_name":"another-pipeline",
									"start_time": 101,
									"end_time": 200,
									"team_name": "some-team"
								},
								"inputs": [{"name": "input-2", "resource": "input-2", "trigger": false}],
								"outputs": [{"name": "output-2", "resource": "output-2"}],
								"groups": ["group-2"]
							},
							{
								"name": "job-3",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-3",
								"next_build": null,
								"finished_build": null,
								"inputs": [{"name": "input-3", "resource": "input-3", "trigger": false}],
								"outputs": [{"name": "output-3", "resource": "output-3"}],
								"groups": []
							}
						]`))

					})
				})
			})

			Context("when getting the build fails", func() {
				BeforeEach(func() {
					pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when getting the job config fails", func() {
			Context("when the pipeline is no longer configured", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("with an unknown error", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{}, 0, false, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", func() {
		var response *http.Response
		var queryParams string

		JustBeforeEach(func() {
			var err error

			pipelineDB.GetPipelineNameReturns("some-pipeline")
			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds" + queryParams)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when no params are passed", func() {
			It("does not set defaults for since and until", func() {
				Expect(pipelineDB.GetJobBuildsCallCount()).To(Equal(1))

				jobName, page := pipelineDB.GetJobBuildsArgsForCall(0)
				Expect(jobName).To(Equal("some-job"))
				Expect(page).To(Equal(db.Page{
					Since: 0,
					Until: 0,
					Limit: 100,
				}))
			})
		})

		Context("when all the params are passed", func() {
			BeforeEach(func() {
				queryParams = "?since=2&until=3&limit=8"
			})

			It("passes them through", func() {
				Expect(pipelineDB.GetJobBuildsCallCount()).To(Equal(1))

				jobName, page := pipelineDB.GetJobBuildsArgsForCall(0)
				Expect(jobName).To(Equal("some-job"))
				Expect(page).To(Equal(db.Page{
					Since: 2,
					Until: 3,
					Limit: 8,
				}))
			})
		})

		Context("when getting the builds succeeds", func() {
			var returnedBuilds []db.Build

			BeforeEach(func() {
				queryParams = "?since=5&limit=2"

				returnedBuilds = []db.Build{
					{
						ID:           4,
						Name:         "2",
						JobName:      "some-job",
						PipelineName: "some-pipeline",
						TeamName:     "some-team",
						Status:       db.StatusStarted,
						StartTime:    time.Unix(1, 0),
						EndTime:      time.Unix(100, 0),
					},
					{
						ID:           2,
						Name:         "1",
						JobName:      "some-job",
						PipelineName: "some-pipeline",
						TeamName:     "some-team",
						Status:       db.StatusSucceeded,
						StartTime:    time.Unix(101, 0),
						EndTime:      time.Unix(200, 0),
					},
				}

				pipelineDB.GetJobBuildsReturns(returnedBuilds, db.Pagination{}, nil)
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns the builds", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[
					{
						"id": 4,
						"name": "2",
						"job_name": "some-job",
						"status": "started",
						"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/2",
						"api_url": "/api/v1/builds/4",
						"pipeline_name":"some-pipeline",
						"team_name":"some-team",
						"start_time": 1,
						"end_time": 100
					},
					{
						"id": 2,
						"name": "1",
						"job_name": "some-job",
						"status": "succeeded",
						"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/1",
						"api_url": "/api/v1/builds/2",
						"pipeline_name":"some-pipeline",
						"team_name":"some-team",
						"start_time": 101,
						"end_time": 200
					}
				]`))
			})

			Context("when next/previous pages are available", func() {
				BeforeEach(func() {
					pipelineDB.GetJobBuildsReturns(returnedBuilds, db.Pagination{
						Previous: &db.Page{Until: 4, Limit: 2},
						Next:     &db.Page{Since: 2, Limit: 2},
					}, nil)
				})

				It("returns Link headers per rfc5988", func() {
					Expect(response.Header["Link"]).To(ConsistOf([]string{
						fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?until=4&limit=2>; rel="previous"`, externalURL),
						fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?since=2&limit=2>; rel="next"`, externalURL),
					}))
				})
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				pipelineDB.GetJobBuildsReturns(nil, db.Pagination{}, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", func() {
		var request *http.Request
		var response *http.Response

		var fakeScheduler *schedulerfakes.FakeBuildScheduler

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("POST", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds", nil)
			Expect(err).NotTo(HaveOccurred())

			fakeScheduler = new(schedulerfakes.FakeBuildScheduler)
			fakeSchedulerFactory.BuildSchedulerReturns(fakeScheduler)
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
			teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
			Expect(pipelineName).To(Equal("some-pipeline"))
			Expect(teamName).To(Equal("some-team"))
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when getting the job config succeeds", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{
						Jobs: []atc.JobConfig{
							{
								Name: "some-job",
								Plan: atc.PlanSequence{
									{
										Get: "some-input",
									},
								},
							},
						},

						Resources: atc.ResourceConfigs{
							{Name: "resource-1", Type: "some-type"},
							{Name: "resource-2", Type: "some-other-type"},
						},
					}, 1, true, nil)
				})

				Context("when triggering the build succeeds", func() {
					BeforeEach(func() {
						fakeScheduler.TriggerImmediatelyReturns(db.Build{
							ID:           42,
							Name:         "1",
							JobName:      "some-job",
							PipelineName: "a-pipeline",
							TeamName:     "some-team",
							Status:       db.StatusStarted,
							StartTime:    time.Unix(1, 0),
							EndTime:      time.Unix(100, 0),
						}, nil, nil)
					})

					It("triggers using the current config", func() {
						Expect(fakeScheduler.TriggerImmediatelyCallCount()).To(Equal(1))

						_, job, resources := fakeScheduler.TriggerImmediatelyArgsForCall(0)
						Expect(job).To(Equal(atc.JobConfig{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get: "some-input",
								},
							},
						}))
						Expect(resources).To(Equal(atc.ResourceConfigs{
							{Name: "resource-1", Type: "some-type"},
							{Name: "resource-2", Type: "some-other-type"},
						}))
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns the build", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
							"id": 42,
							"name": "1",
							"job_name": "some-job",
							"status": "started",
							"url": "/teams/some-team/pipelines/a-pipeline/jobs/some-job/builds/1",
							"api_url": "/api/v1/builds/42",
							"pipeline_name": "a-pipeline",
							"team_name": "some-team",
							"start_time": 1,
							"end_time": 100
						}`))
					})
				})

				Context("when triggering the build fails", func() {
					BeforeEach(func() {
						fakeScheduler.TriggerImmediatelyReturns(db.Build{}, nil, errors.New("oh no!"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job is not present in the config", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{
							Jobs: []atc.JobConfig{
								{Name: "other-job"},
							},
						}, 1, true, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})

			Context("when getting the job config fails", func() {
				Context("when the pipeline is no longer configured", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("with an unknown error", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{}, 0, false, errors.New("oh no!"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/inputs", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/inputs")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("looked up the proper pipeline", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("some-pipeline"))
				Expect(teamName).To(Equal("some-team"))
			})

			Context("when getting the config succeeds", func() {
				Context("when it contains the requested job", func() {
					someJob := atc.JobConfig{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Get:      "some-input",
								Resource: "some-resource",
								Passed:   []string{"job-a", "job-b"},
								Params:   atc.Params{"some": "params"},
							},
							{
								Get:      "some-other-input",
								Resource: "some-other-resource",
								Passed:   []string{"job-c", "job-d"},
								Params:   atc.Params{"some": "other-params"},
								Tags:     []string{"some-tag"},
							},
						},
					}

					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{
							Jobs: atc.JobConfigs{
								someJob,
							},

							Resources: atc.ResourceConfigs{
								{
									Name:   "some-resource",
									Source: atc.Source{"some": "source"},
								},
								{
									Name:   "some-other-resource",
									Source: atc.Source{"some": "other-source"},
								},
							},
						}, 42, true, nil)
					})

					Context("when the versions can be loaded", func() {
						versionsDB := &algorithm.VersionsDB{}

						BeforeEach(func() {
							pipelineDB.LoadVersionsDBReturns(versionsDB, nil)
						})

						Context("when the input versions for the job can be determined", func() {
							BeforeEach(func() {
								pipelineDB.GetLatestInputVersionsReturns([]db.BuildInput{
									{
										Name: "some-input",
										VersionedResource: db.VersionedResource{
											Resource:     "some-resource",
											Type:         "some-type",
											Version:      db.Version{"some": "version"},
											PipelineName: "some-pipeline",
										},
									},
									{
										Name: "some-other-input",
										VersionedResource: db.VersionedResource{
											Resource:     "some-other-resource",
											Type:         "some-other-type",
											Version:      db.Version{"some": "other-version"},
											PipelineName: "some-pipeline",
										},
									},
								}, true, nil)
							})

							It("returns 200 OK", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("determined the inputs with the correct versions DB, job name, and inputs", func() {
								receivedVersionsDB, receivedJob, receivedInputs := pipelineDB.GetLatestInputVersionsArgsForCall(0)
								Expect(receivedVersionsDB).To(Equal(versionsDB))
								Expect(receivedJob).To(Equal("some-job"))
								Expect(receivedInputs).To(Equal(config.JobInputs(someJob)))
							})

							It("returns the inputs", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchJSON(`[
									{
										"name": "some-input",
										"resource": "some-resource",
										"type": "some-type",
										"source": {"some": "source"},
										"version": {"some": "version"},
										"params": {"some": "params"}
									},
									{
										"name": "some-other-input",
										"resource": "some-other-resource",
										"type": "some-other-type",
										"source": {"some": "other-source"},
										"version": {"some": "other-version"},
										"params": {"some": "other-params"},
										"tags": ["some-tag"]
									}
								]`))

							})
						})

						Context("when the job has no input versions available", func() {
							BeforeEach(func() {
								pipelineDB.GetLatestInputVersionsReturns(nil, false, nil)
							})

							It("returns 404", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when the input versions for the job can not be determined", func() {
							BeforeEach(func() {
								pipelineDB.GetLatestInputVersionsReturns(nil, false, errors.New("oh no!"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when the versions can not be loaded", func() {
						BeforeEach(func() {
							pipelineDB.LoadVersionsDBReturns(nil, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when it does not contain the requested job", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{
							Jobs: atc.JobConfigs{
								{
									Name: "some-bogus-job",
									Plan: atc.PlanSequence{},
								},
							},
						}, 42, true, nil)
					})

					It("returns 404 Not Found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})

			Context("when getting the config fails", func() {
				Context("when the pipeline is no longer configured", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("with an unknown error", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{}, 0, false, errors.New("oh no!"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
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

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/some-build")
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
			teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
			Expect(pipelineName).To(Equal("some-pipeline"))
			Expect(teamName).To(Equal("some-team"))
		})

		Context("when getting the build succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetJobBuildReturns(db.Build{
					ID:           1,
					Name:         "1",
					JobName:      "some-job",
					PipelineName: "a-pipeline",
					TeamName:     "some-team",
					Status:       db.StatusSucceeded,
					StartTime:    time.Unix(1, 0),
					EndTime:      time.Unix(100, 0),
				}, true, nil)
			})

			It("fetches by job and build name", func() {
				Expect(pipelineDB.GetJobBuildCallCount()).To(Equal(1))

				jobName, buildName := pipelineDB.GetJobBuildArgsForCall(0)
				Expect(jobName).To(Equal("some-job"))
				Expect(buildName).To(Equal("some-build"))
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns the build", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{
					"id": 1,
					"name": "1",
					"job_name": "some-job",
					"status": "succeeded",
					"url": "/teams/some-team/pipelines/a-pipeline/jobs/some-job/builds/1",
					"api_url": "/api/v1/builds/1",
					"pipeline_name": "a-pipeline",
					"team_name": "some-team",
					"start_time": 1,
					"end_time": 100
				}`))

			})
		})

		Context("when the build is not found", func() {
			BeforeEach(func() {
				pipelineDB.GetJobBuildReturns(db.Build{}, false, nil)
			})

			It("returns Not Found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				pipelineDB.GetJobBuildReturns(db.Build{}, false, errors.New("oh no!"))
			})

			It("returns Internal Server Error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/pause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/job-name/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the PipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("some-pipeline"))
				Expect(teamName).To(Equal("some-team"))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.PauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Expect(pipelineDB.PauseJobArgsForCall(0)).To(Equal("job-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					pipelineDB.PauseJobReturns(errors.New("welp"))
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

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/unpause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/job-name/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the PipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithTeamNameAndNameCallCount()).To(Equal(1))
				teamName, pipelineName := pipelineDBFactory.BuildWithTeamNameAndNameArgsForCall(0)
				Expect(pipelineName).To(Equal("some-pipeline"))
				Expect(teamName).To(Equal("some-team"))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Expect(pipelineDB.UnpauseJobArgsForCall(0)).To(Equal("job-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseJobReturns(errors.New("welp"))
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
})
