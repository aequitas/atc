package acceptance_test

import (
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/sclevine/agouti"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
)

var _ = Describe("Job Builds", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var atcPort uint16
	var pipelineDBFactory db.PipelineDBFactory
	var pipelineDB db.PipelineDB

	BeforeEach(func() {
		dbLogger := lagertest.NewTestLogger("test")
		postgresRunner.Truncate()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbLogger, dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(dbLogger, dbConn, bus, sqlDB)
		atcProcess, atcPort = startATC(atcBin, 1, true, BASIC_AUTH)
		_, err := dbConn.Query(`DELETE FROM teams WHERE name = 'main'`)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("viewing a jobs builds", func() {
		var page *agouti.Page

		BeforeEach(func() {
			var err error
			page, err = agoutiDriver.NewPage()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(page.Destroy()).To(Succeed())
		})

		homepage := func() string {
			return fmt.Sprintf("http://127.0.0.1:%d/pipelines/%s", atcPort, atc.DefaultPipelineName)
		}

		withPath := func(path string) string {
			return urljoiner.Join(homepage(), path)
		}

		Context("with a job in the configuration", func() {
			var build db.Build

			BeforeEach(func() {
				team, err := sqlDB.SaveTeam(db.Team{Name: atc.DefaultTeamName})
				Expect(err).NotTo(HaveOccurred())

				// job build data
				_, err = sqlDB.SaveConfig(team.Name, atc.DefaultPipelineName, atc.Config{
					Jobs: []atc.JobConfig{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				pipelineDB, err = pipelineDBFactory.BuildWithTeamNameAndName(team.Name, atc.DefaultPipelineName)
				Expect(err).NotTo(HaveOccurred())

				build, err = pipelineDB.CreateJobBuild("job-name")
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.StartBuild(build.ID, "", "")
				Expect(err).NotTo(HaveOccurred())

				sqlDB.SaveBuildEvent(build.ID, event.Log{
					Origin: event.Origin{
						Source: event.OriginSourceStdout,
						ID:     "some-id",
					},
					Payload: "hello this is a payload",
				})

				Expect(sqlDB.FinishBuild(build.ID, db.StatusSucceeded)).To(Succeed())

				myBuildInput := db.BuildInput{
					Name: "build-input-1",
					VersionedResource: db.VersionedResource{
						Resource:     "my-resource",
						PipelineName: atc.DefaultPipelineName,
						Version: db.Version{
							"ref": "thing",
						},
					},
				}

				_, err = sqlDB.SaveBuildInput(team.Name, build.ID, myBuildInput)
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.SaveBuildOutput(team.Name, build.ID, db.VersionedResource{
					Resource:     "some-output",
					PipelineName: atc.DefaultPipelineName,
					Version: db.Version{
						"thing": "output-version",
					},
				}, true)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with more then 100 job builds", func() {
				var testBuilds []db.Build

				BeforeEach(func() {
					for i := 1; i < 103; i++ {
						build, err := pipelineDB.CreateJobBuild("job-name")
						Expect(err).NotTo(HaveOccurred())
						testBuilds = append(testBuilds, build)
					}
				})

				It("can have paginated results", func() {
					// homepage -> job detail w/build info
					Expect(page.Navigate(homepage())).To(Succeed())
					// we will need to authenticate later to prove it is working for our page
					Authenticate(page, "admin", "password")
					Eventually(page.FindByLink("job-name")).Should(BeFound())
					Expect(page.FindByLink("job-name").Click()).To(Succeed())

					Eventually(page.All("#builds li").Count).Should(Equal(103))

					// job detail w/build info -> job detail
					Eventually(page.Find("h1 a")).Should(BeFound())
					Expect(page.Find("h1 a").Click()).To(Succeed())
					Eventually(page).Should(HaveURL(withPath("jobs/job-name")))
					Expect(page.All(".js-build").Count()).Should(Equal(100))

					Expect(page.First(".pagination .disabled .fa-arrow-left")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-right").Click()).To(Succeed())
					Eventually(page.All(".js-build").Count).Should(Equal(3))

					Expect(page.First(".pagination .disabled .fa-arrow-right")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-left").Click()).To(Succeed())
					Eventually(page.All(".js-build").Count).Should(Equal(100))
				})
			})

			It("can view the resource information of a job build", func() {
				// homepage -> job detail w/build info
				Expect(page.Navigate(homepage())).To(Succeed())
				// we will need to authenticate later to prove it is working for our page
				Authenticate(page, "admin", "password")
				Eventually(page.FindByLink("job-name")).Should(BeFound())
				Expect(page.FindByLink("job-name").Click()).To(Succeed())

				// job detail w/build info -> job detail
				Eventually(page).Should(HaveURL(withPath(fmt.Sprintf("jobs/job-name/builds/%d", build.ID))))
				Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("job-name #%d", build.ID)))
				Expect(page.Find("h1 a").Click()).To(Succeed())
				Eventually(page).Should(HaveURL(withPath("jobs/job-name")))

				Expect(page.Find(".builds-list li").Count()).Should(Equal(1))
				Expect(page.Find(".builds-list li:first-child a")).To(HaveText(fmt.Sprintf("#%d", build.ID)))

				buildTimes, err := page.Find(".builds-list li:first-child .build-times").Text()
				Expect(err).NotTo(HaveOccurred())
				Expect(buildTimes).To(ContainSubstring("started"))
				Expect(buildTimes).To(ContainSubstring("a few seconds ago"))
				Expect(buildTimes).To(ContainSubstring("succeeded"))
				Expect(buildTimes).To(ContainSubstring("a few seconds ago"))
				Expect(buildTimes).To(ContainSubstring("duration"))

				Expect(page.Find(".builds-list li:first-child .inputs")).Should(BeFound())
				Expect(page.Find(".builds-list li:first-child .inputs .resource-name")).To(HaveText("my-resource"))
				Expect(page.Find(".builds-list li:first-child .inputs .resource-version")).To(HaveText("ref: thing"))

				Expect(page.Find(".builds-list li:first-child .outputs")).Should(BeFound())
				Expect(page.Find(".builds-list li:first-child .outputs .resource-name")).To(HaveText("some-output"))
				Expect(page.Find(".builds-list li:first-child .outputs .resource-version")).To(HaveText("thing: output-version"))
			})
		})
	})
})
