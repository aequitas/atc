package db_test

import (
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Keeping track of containers", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var database *db.SQLDB
	var savedPipeline db.SavedPipeline
	var pipelineDB db.PipelineDB

	BeforeEach(func() {
		var err error

		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		database = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)

		config := atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
				{
					Name: "some-other-job",
				},
				{
					Name: "some-random-job",
				},
			},
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
				},
				{
					Name: "some-other-resource",
					Type: "some-other-type",
				},
			},
		}

		savedPipeline, _, err = database.SaveConfig(atc.DefaultTeamName, "some-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		_, _, err = database.SaveConfig(atc.DefaultTeamName, "some-other-pipeline", config, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(nil, dbConn, nil, database)
		pipelineDB = pipelineDBFactory.Build(savedPipeline)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	getResourceID := func(name string) int {
		savedResource, err := pipelineDB.GetResource(name)
		Expect(err).NotTo(HaveOccurred())
		return savedResource.ID
	}

	getJobBuildID := func(jobName string) int {
		savedBuild, err := pipelineDB.CreateJobBuild(jobName)
		Expect(err).NotTo(HaveOccurred())
		return savedBuild.ID
	}

	getOneOffBuildID := func() int {
		savedBuild, err := database.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
		return savedBuild.ID
	}

	It("can create and get a resource container object", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				ResourceID:  getResourceID("some-resource"),
				CheckType:   "some-resource-type",
				CheckSource: atc.Source{"some": "source"},
				Stage:       db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:               "some-handle",
				WorkerName:           "some-worker",
				PipelineName:         "some-pipeline",
				Type:                 db.ContainerTypeCheck,
				WorkingDirectory:     "tmp/build/some-guid",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
			},
		}

		By("creating a container")
		_, err := database.CreateContainer(containerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		matchingHandleContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage: db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle: "some-handle",
			},
		}
		_, err = database.CreateContainer(matchingHandleContainer, time.Second)
		Expect(err).To(HaveOccurred())

		By("getting the saved info object by handle")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.ResourceID).To(Equal(containerToCreate.ResourceID))

		Expect(actualContainer.Handle).To(Equal(containerToCreate.Handle))
		Expect(actualContainer.StepName).To(Equal(""))
		Expect(actualContainer.ResourceName).To(Equal("some-resource"))
		Expect(actualContainer.PipelineID).To(Equal(savedPipeline.ID))
		Expect(actualContainer.PipelineName).To(Equal(savedPipeline.Name))
		Expect(actualContainer.BuildID).To(Equal(0))
		Expect(actualContainer.BuildName).To(Equal(""))
		Expect(actualContainer.Type).To(Equal(db.ContainerTypeCheck))
		Expect(actualContainer.ContainerMetadata.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.WorkingDirectory).To(Equal(containerToCreate.WorkingDirectory))
		Expect(actualContainer.CheckType).To(Equal(containerToCreate.CheckType))
		Expect(actualContainer.CheckSource).To(Equal(containerToCreate.CheckSource))
		Expect(actualContainer.EnvironmentVariables).To(Equal(containerToCreate.EnvironmentVariables))

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	It("can create and get a step container info object", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: 1111,
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:               "some-handle",
				WorkerName:           "some-worker",
				PipelineName:         "some-pipeline",
				StepName:             "some-step-container",
				Type:                 db.ContainerTypeTask,
				WorkingDirectory:     "tmp/build/some-guid",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
				User:                 "test-user",
				Attempts:             []int{1, 2, 4},
			},
		}

		By("creating a container")
		_, err := database.CreateContainer(containerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		By("trying to create a container with the same handle")
		duplicateHandleContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: 1112,
				PlanID:  "some-other-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "some-handle",
				WorkerName:   "some-worker",
				PipelineName: "some-pipeline",
				Type:         db.ContainerTypeTask,
			},
		}
		_, err = database.CreateContainer(duplicateHandleContainer, time.Second)
		Expect(err).To(HaveOccurred())

		By("trying to create a container with an insufficient step identifier")
		insufficientStepContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: 1113,
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "some-handle-2",
				WorkerName:   "some-worker",
				PipelineName: "some-pipeline",
				Type:         db.ContainerTypeTask,
			},
		}
		_, err = database.CreateContainer(insufficientStepContainer, time.Second)
		Expect(err).To(Equal(db.ErrInvalidIdentifier))

		By("trying to create a container with an insufficient check identifier")
		insufficientCheckContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				ResourceID: 72,
				CheckType:  "git",
				Stage:      db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "some-handle-3",
				WorkerName:   "some-worker",
				PipelineName: "some-pipeline",
				Type:         db.ContainerTypeCheck,
			},
		}
		_, err = database.CreateContainer(insufficientCheckContainer, time.Second)
		Expect(err).To(Equal(db.ErrInvalidIdentifier))

		By("getting the saved info object by handle")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.BuildID).To(Equal(containerToCreate.BuildID))
		Expect(actualContainer.PlanID).To(Equal(containerToCreate.PlanID))

		Expect(actualContainer.Handle).To(Equal(containerToCreate.Handle))
		Expect(actualContainer.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.PipelineID).To(Equal(savedPipeline.ID))
		Expect(actualContainer.PipelineName).To(Equal(containerToCreate.PipelineName))
		Expect(actualContainer.StepName).To(Equal(containerToCreate.StepName))
		Expect(actualContainer.BuildName).To(Equal(""))
		Expect(actualContainer.Type).To(Equal(containerToCreate.Type))
		Expect(actualContainer.WorkingDirectory).To(Equal(containerToCreate.WorkingDirectory))
		Expect(actualContainer.EnvironmentVariables).To(Equal(containerToCreate.EnvironmentVariables))
		Expect(actualContainer.User).To(Equal(containerToCreate.User))
		Expect(actualContainer.Attempts).To(Equal(containerToCreate.Attempts))

		Expect(actualContainer.ResourceID).To(Equal(0))
		Expect(actualContainer.ResourceName).To(Equal(""))
		Expect(actualContainer.CheckType).To(BeEmpty())
		Expect(actualContainer.CheckSource).To(BeEmpty())

		By("returning found = false when getting by a handle that does not exist")
		_, found, err = database.GetContainer("nope")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})

	It("can populate metadata that was omitted when creating the container", func() {
		savedBuild, err := pipelineDB.CreateJobBuild("some-job")
		Expect(err).NotTo(HaveOccurred())

		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				BuildID: savedBuild.ID,
				PlanID:  "some-plan-id",
				Stage:   db.ContainerStageRun,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:               "some-handle",
				WorkerName:           "some-worker",
				PipelineName:         "some-pipeline",
				StepName:             "some-step-container",
				Type:                 db.ContainerTypeTask,
				WorkingDirectory:     "tmp/build/some-guid",
				EnvironmentVariables: []string{"VAR1=val1", "VAR2=val2"},
				Attempts:             []int{1, 2, 4},
			},
		}

		By("creating a container with optional metadata fields omitted")
		_, err = database.CreateContainer(containerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		By("populating those fields when retrieving the container")
		actualContainer, found, err := database.GetContainer("some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.BuildName).To(Equal(savedBuild.Name))
		Expect(actualContainer.PipelineID).To(Equal(savedPipeline.ID))
		Expect(actualContainer.JobName).To(Equal("some-job"))
	})

	It("can update the time to live for a container info object", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  "update-ttl-plan",
				BuildID: 2000,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "some-handle",
				Type:         db.ContainerTypeTask,
				WorkerName:   "some-worker",
				PipelineName: "some-pipeline",
			},
		}

		_, err := database.CreateContainer(containerToCreate, 5*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		timeBefore := time.Now()
		err = database.UpdateExpiresAtOnContainer("some-handle", time.Second)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			_, found, err := database.GetContainer("some-handle")
			Expect(err).NotTo(HaveOccurred())
			return found
		}, 10*time.Second).Should(BeFalse())

		timeAfter := time.Now()
		Expect(timeAfter.Sub(timeBefore)).To(BeNumerically(">=", time.Second))
		Expect(timeAfter.Sub(timeBefore)).To(BeNumerically("<", 10*time.Second))
	})

	It("can reap a container", func() {
		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  "to-be-reaped-plan",
				BuildID: 1000,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "some-reaped-handle",
				Type:         db.ContainerTypeTask,
				WorkerName:   "some-worker",
				PipelineName: "some-pipeline",
			},
		}
		_, err := database.CreateContainer(containerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		_, found, err := database.GetContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		By("reaping an existing container")
		err = database.ReapContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())

		_, found, err = database.GetContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())

		By("not failing if the container's already been reaped")
		err = database.ReapContainer("some-reaped-handle")
		Expect(err).NotTo(HaveOccurred())
	})

	It("differentiates between a single step's containers with different stages", func() {
		someBuild, err := database.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())

		checkStageAContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID,
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-a",
			Stage:               db.ContainerStageCheck,
		}

		getStageAContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID,
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-a",
			Stage:               db.ContainerStageGet,
		}

		checkStageBContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID,
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-b",
			Stage:               db.ContainerStageCheck,
		}

		getStageBContainerID := db.ContainerIdentifier{
			BuildID:             someBuild.ID,
			PlanID:              atc.PlanID("some-task"),
			ImageResourceSource: atc.Source{"some": "source"},
			ImageResourceType:   "some-type-b",
			Stage:               db.ContainerStageGet,
		}

		runStageContainerID := db.ContainerIdentifier{
			BuildID: someBuild.ID,
			PlanID:  atc.PlanID("some-task"),
			Stage:   db.ContainerStageRun,
		}

		checkContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-a-handle",
				Type:   db.ContainerTypeCheck,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		getContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-a-handle",
				Type:   db.ContainerTypeGet,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		checkContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-b-handle",
				Type:   db.ContainerTypeCheck,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		getContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-b-handle",
				Type:   db.ContainerTypeGet,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		runContainer, err := database.CreateContainer(db.Container{
			ContainerIdentifier: runStageContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "run-handle",
				Type:   db.ContainerTypeTask,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		container, found, err := database.FindContainerByIdentifier(checkStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(checkStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(runStageContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(runContainer.ContainerIdentifier))
	})

	It("differentiates between a single resource's checking containers with different stages", func() {
		checkStageAContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-a",
			Stage:               db.ContainerStageCheck,
		}

		getStageAContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-a",
			Stage:               db.ContainerStageGet,
		}

		checkStageBContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-b",
			Stage:               db.ContainerStageCheck,
		}

		getStageBContainerID := db.ContainerIdentifier{
			ResourceID:          1,
			CheckSource:         atc.Source{"some": "source"},
			CheckType:           "some-type",
			ImageResourceSource: atc.Source{"some": "image-source"},
			ImageResourceType:   "some-image-type-b",
			Stage:               db.ContainerStageGet,
		}

		runStageContainerID := db.ContainerIdentifier{
			ResourceID:  1,
			CheckSource: atc.Source{"some": "source"},
			CheckType:   "some-type",
			Stage:       db.ContainerStageRun,
		}

		checkContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-a-handle",
				Type:   db.ContainerTypeCheck,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		getContainerA, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageAContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-a-handle",
				Type:   db.ContainerTypeGet,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		checkContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: checkStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "check-b-handle",
				Type:   db.ContainerTypeCheck,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		getContainerB, err := database.CreateContainer(db.Container{
			ContainerIdentifier: getStageBContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "get-b-handle",
				Type:   db.ContainerTypeGet,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		runContainer, err := database.CreateContainer(db.Container{
			ContainerIdentifier: runStageContainerID,
			ContainerMetadata: db.ContainerMetadata{
				Handle: "run-handle",
				Type:   db.ContainerTypeTask,
			},
		}, time.Minute)
		Expect(err).ToNot(HaveOccurred())

		container, found, err := database.FindContainerByIdentifier(checkStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageAContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerA.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(checkStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(checkContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(getStageBContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(getContainerB.ContainerIdentifier))

		container, found, err = database.FindContainerByIdentifier(runStageContainerID)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(container.ContainerIdentifier).To(Equal(runContainer.ContainerIdentifier))
	})

	type findContainersByDescriptorsExample struct {
		containersToCreate     []db.Container
		descriptorsToFilterFor db.Container
		expectedHandles        []string
	}

	DescribeTable("filtering containers by descriptors",
		func(exampleGenerator func() findContainersByDescriptorsExample) {
			var results []db.Container
			var handles []string
			var err error

			example := exampleGenerator()

			for _, containerToCreate := range example.containersToCreate {
				if containerToCreate.Type.String() == "" {
					containerToCreate.Type = db.ContainerTypeTask
				}

				_, err := database.CreateContainer(containerToCreate, time.Minute)
				Expect(err).NotTo(HaveOccurred())
			}

			results, err = database.FindContainersByDescriptors(example.descriptorsToFilterFor)
			Expect(err).NotTo(HaveOccurred())

			for _, result := range results {
				handles = append(handles, result.Handle)
			}

			Expect(handles).To(ConsistOf(example.expectedHandles))

			for _, containerToDelete := range example.containersToCreate {
				err = database.DeleteContainer(containerToDelete.Handle)
				Expect(err).NotTo(HaveOccurred())
			}
		},

		Entry("returns everything when no filters are passed", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("does not return things that the filter doesn't match", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{ResourceName: "some-resource"}},
				expectedHandles:        nil,
			}
		}),

		Entry("returns containers where the step name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							StepName:     "some-step",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
							StepName:     "some-other-step",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
							StepName:     "some-step",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{StepName: "some-step"}},
				expectedHandles:        []string{"a", "c"},
			}
		}),

		Entry("returns containers where the resource name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceID:  getResourceID("some-resource"),
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							ResourceName: "some-resource",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceID:  getResourceID("some-resource"),
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
							ResourceName: "some-resource",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							ResourceID:  getResourceID("some-other-resource"),
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
							ResourceName: "some-other-resource",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{ResourceName: "some-resource"}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("returns containers where the pipeline matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-Oother-worker",
							PipelineName: "some-pipeline",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{PipelineName: "some-pipeline"}},
				expectedHandles:        []string{"a", "c"},
			}
		}),

		Entry("returns containers where the type matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypePut,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypePut,
							WorkerName:   "some-other-worker",
							PipelineName: "some-other-pipeline",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeGet,
							WorkerName:   "some-Oother-worker",
							PipelineName: "some-pipeline",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{Type: db.ContainerTypePut}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("returns containers where the worker name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypePut,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypePut,
							WorkerName:   "some-worker",
							PipelineName: "some-other-pipeline",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeGet,
							WorkerName:   "some-other-worker",
							PipelineName: "some-pipeline",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{WorkerName: "some-worker"}},
				expectedHandles:        []string{"a", "b"},
			}
		}),

		Entry("returns containers where the check type matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:       db.ContainerStageRun,
							CheckSource: atc.Source{"some": "source"},
							CheckType:   "git",
							ResourceID:  1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							ResourceName: "some-resource",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:       db.ContainerStageRun,
							CheckType:   "nope",
							CheckSource: atc.Source{"some": "source"},
							ResourceID:  1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineName: "some-other-pipeline",
							ResourceName: "some-resource",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:       db.ContainerStageRun,
							CheckType:   "some-type",
							CheckSource: atc.Source{"some": "source"},
							ResourceID:  1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineName: "some-pipeline",
							ResourceName: "some-resource",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerIdentifier: db.ContainerIdentifier{CheckType: "some-type"}},
				expectedHandles:        []string{"c"},
			}
		}),

		Entry("returns containers where the check source matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage: db.ContainerStageRun,
							CheckSource: atc.Source{
								"some": "other-source",
							},
							CheckType:  "git",
							ResourceID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "a",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							ResourceName: "some-resource",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "b",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-other-pipeline",
							ResourceName: "some-resource",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage: db.ContainerStageRun,
							CheckSource: atc.Source{
								"some": "source",
							},
							CheckType:  "git",
							ResourceID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Handle:       "c",
							Type:         db.ContainerTypeCheck,
							WorkerName:   "some-other-worker",
							PipelineName: "some-pipeline",
							ResourceName: "some-resource",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerIdentifier: db.ContainerIdentifier{CheckSource: atc.Source{"some": "source"}}},
				expectedHandles:        []string{"c"},
			}
		}),

		Entry("returns containers where the job name matches", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{{
					ContainerIdentifier: db.ContainerIdentifier{
						Stage:   db.ContainerStageRun,
						BuildID: getJobBuildID("some-other-job"),
						PlanID:  "plan-id",
					},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						JobName:      "some-other-job",
						Handle:       "a",
					},
				},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: getJobBuildID("some-job"),
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Handle:       "b",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: getOneOffBuildID(),
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-other-worker",
							PipelineName: "",
							JobName:      "",
							Handle:       "c",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{JobName: "some-job"}},
				expectedHandles:        []string{"b"},
			}
		}),

		Entry("returns containers where the build ID matches", func() findContainersByDescriptorsExample {
			someBuildID := getJobBuildID("some-job")
			someOtherBuildID := getJobBuildID("some-job")
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: someBuildID,
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Handle:       "a",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: someOtherBuildID,
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							JobName:      "some-other-job",
							Handle:       "b",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerIdentifier: db.ContainerIdentifier{BuildID: someBuildID}},
				expectedHandles:        []string{"a"},
			}
		}),

		Entry("returns containers where the build name matches", func() findContainersByDescriptorsExample {
			savedBuild1, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			savedBuild2, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())
			savedBuild3, err := pipelineDB.CreateJobBuild("some-other-job")
			Expect(err).NotTo(HaveOccurred())
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: savedBuild1.ID,
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Handle:       "a",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: savedBuild2.ID,
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							BuildName:    savedBuild2.Name,
							Handle:       "b",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							BuildID: savedBuild3.ID,
							PlanID:  "plan-id",
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							JobName:      "some-other-job",
							// purposefully re-use the original build name to test that it
							// can return multiple containers
							BuildName: savedBuild1.Name,
							Handle:    "c",
						},
					},
				},
				descriptorsToFilterFor: db.Container{
					ContainerMetadata: db.ContainerMetadata{
						BuildName: savedBuild1.Name,
					},
				},
				expectedHandles: []string{"a", "c"},
			}
		}),

		Entry("returns containers where the attempts numbers match", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{{
					ContainerIdentifier: db.ContainerIdentifier{
						Stage:   db.ContainerStageRun,
						PlanID:  "plan-id",
						BuildID: 1234,
					},
					ContainerMetadata: db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
						PipelineName: "some-pipeline",
						Attempts:     []int{1, 2, 5},
						Handle:       "a",
					},
				},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							Attempts:     []int{1, 2},
							Handle:       "b",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-other-worker",
							PipelineName: "some-pipeline",
							Attempts:     []int{1},
							Handle:       "c",
						},
					},
				},
				descriptorsToFilterFor: db.Container{ContainerMetadata: db.ContainerMetadata{Attempts: []int{1, 2}}},
				expectedHandles:        []string{"b"},
			}
		}),

		Entry("returns containers where all fields match", func() findContainersByDescriptorsExample {
			return findContainersByDescriptorsExample{
				containersToCreate: []db.Container{
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							StepName:     "some-name",
							PipelineName: "some-pipeline",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							Handle:       "a",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							StepName:     "WROONG",
							PipelineName: "some-pipeline",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							Handle:       "b",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							StepName:     "some-name",
							PipelineName: "some-pipeline",
							Type:         db.ContainerTypeTask,
							WorkerName:   "some-worker",
							Handle:       "c",
						},
					},
					{
						ContainerIdentifier: db.ContainerIdentifier{
							Stage:   db.ContainerStageRun,
							PlanID:  "plan-id",
							BuildID: 1234,
						},
						ContainerMetadata: db.ContainerMetadata{
							WorkerName:   "some-worker",
							PipelineName: "some-pipeline",
							Type:         db.ContainerTypeTask,
							Handle:       "d",
						},
					},
				},
				descriptorsToFilterFor: db.Container{
					ContainerMetadata: db.ContainerMetadata{
						StepName:     "some-name",
						PipelineName: "some-pipeline",
						Type:         db.ContainerTypeTask,
						WorkerName:   "some-worker",
					},
				},
				expectedHandles: []string{"a", "c"},
			}
		}),
	)

	It("can find a single container info by identifier", func() {
		handle := "some-handle"
		otherHandle := "other-handle"

		containerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       handle,
				PipelineName: "some-pipeline",
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
			},
		}
		stepContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  atc.PlanID("plan-id"),
				BuildID: 555,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       otherHandle,
				PipelineName: "some-pipeline",
				WorkerName:   "some-worker",
				StepName:     "other-container",
				Type:         db.ContainerTypeTask,
			},
		}
		otherStepContainer := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:   db.ContainerStageRun,
				PlanID:  atc.PlanID("other-plan-id"),
				BuildID: 666,
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "very-other-handle",
				PipelineName: "some-pipeline",
				WorkerName:   "some-worker",
				StepName:     "other-container",
				Type:         db.ContainerTypeTask,
			},
		}

		_, err := database.CreateContainer(containerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())
		_, err = database.CreateContainer(stepContainerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())
		_, err = database.CreateContainer(otherStepContainer, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		all_containers := getAllContainers(dbConn)
		Expect(all_containers).To(HaveLen(3))

		By("returning a single matching resource container info")
		actualContainer, found, err := database.FindContainerByIdentifier(
			containerToCreate.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(actualContainer.Handle).To(Equal("some-handle"))
		Expect(actualContainer.WorkerName).To(Equal(containerToCreate.WorkerName))
		Expect(actualContainer.ResourceID).To(Equal(containerToCreate.ResourceID))

		By("returning a single matching step container info")
		actualStepContainer, found, err := database.FindContainerByIdentifier(
			stepContainerToCreate.ContainerIdentifier,
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(actualStepContainer.Handle).To(Equal("other-handle"))
		Expect(actualStepContainer.WorkerName).To(Equal(stepContainerToCreate.WorkerName))
		Expect(actualStepContainer.ResourceID).To(Equal(stepContainerToCreate.ResourceID))

		By("differentiating check containers based on their check source")
		newSourceContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "new-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "new-source-handle",
				PipelineName: "some-pipeline",
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
			},
		}

		_, err = database.CreateContainer(newSourceContainerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		foundNewSourceContainer, found, err := database.FindContainerByIdentifier(newSourceContainerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundNewSourceContainer.Handle).To(Equal(newSourceContainerToCreate.Handle))

		foundOldSourceContainer, found, err := database.FindContainerByIdentifier(containerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundOldSourceContainer.Handle).To(Equal(containerToCreate.Handle))

		By("differentiating check containers based on their check type")
		newCheckTypeContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-new-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "new-check-type-handle",
				PipelineName: "some-pipeline",
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
			},
		}

		_, err = database.CreateContainer(newCheckTypeContainerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		foundNewCheckTypeContainer, found, err := database.FindContainerByIdentifier(newCheckTypeContainerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundNewCheckTypeContainer.Handle).To(Equal(newCheckTypeContainerToCreate.Handle))

		foundOldCheckTypeContainer, found, err := database.FindContainerByIdentifier(containerToCreate.ContainerIdentifier)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundOldCheckTypeContainer.Handle).To(Equal(containerToCreate.Handle))

		By("erroring if more than one container matches the filter")
		matchingContainerToCreate := db.Container{
			ContainerIdentifier: db.ContainerIdentifier{
				Stage:       db.ContainerStageRun,
				CheckType:   "some-type",
				CheckSource: atc.Source{"some": "other-source"},
				ResourceID:  getResourceID("some-resource"),
			},
			ContainerMetadata: db.ContainerMetadata{
				Handle:       "matching-handle",
				PipelineName: "some-pipeline",
				ResourceName: "some-resource",
				WorkerName:   "some-worker",
				Type:         db.ContainerTypeCheck,
			},
		}

		createdMatchingContainer, err := database.CreateContainer(matchingContainerToCreate, time.Minute)
		Expect(err).NotTo(HaveOccurred())

		foundContainer, found, err := database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				ResourceID:  createdMatchingContainer.ResourceID,
				CheckType:   createdMatchingContainer.CheckType,
				CheckSource: createdMatchingContainer.CheckSource,
				Stage:       createdMatchingContainer.Stage,
			})
		Expect(err).To(HaveOccurred())
		Expect(err).To(Equal(db.ErrMultipleContainersFound))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				BuildID: createdMatchingContainer.BuildID,
			})
		Expect(err).To(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("still erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				PlanID: createdMatchingContainer.PlanID,
			})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("still erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				ResourceID: createdMatchingContainer.ResourceID,
				CheckType:  createdMatchingContainer.CheckType,
			})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("still erroring if not enough identifiers are passed in")
		foundContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				ResourceID:  createdMatchingContainer.ResourceID,
				CheckSource: createdMatchingContainer.CheckSource,
			})
		Expect(err).To(Equal(db.ErrInvalidIdentifier))
		Expect(found).To(BeFalse())
		Expect(foundContainer.Handle).To(BeEmpty())

		By("returning found of false if no containers match the filter")
		actualContainer, found, err = database.FindContainerByIdentifier(
			db.ContainerIdentifier{
				BuildID: 404,
				PlanID:  atc.PlanID("plan-id"),
				Stage:   db.ContainerStageRun,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(actualContainer.Handle).To(BeEmpty())

		By("removing it if the TTL has expired")
		ttl := 1 * time.Second

		err = database.UpdateExpiresAtOnContainer(otherHandle, -ttl)
		Expect(err).NotTo(HaveOccurred())
		_, found, err = database.FindContainerByIdentifier(
			stepContainerToCreate.ContainerIdentifier,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
	})
})

func getAllContainers(sqldb db.Conn) []db.Container {
	var container_slice []db.Container
	query := `SELECT worker_name, pipeline_id, resource_id, build_id, plan_id
	          FROM containers
						`
	rows, err := sqldb.Query(query)
	Expect(err).NotTo(HaveOccurred())
	defer rows.Close()

	for rows.Next() {
		var container db.Container
		rows.Scan(&container.WorkerName, &container.ResourceID, &container.BuildID, &container.PlanID)
		container_slice = append(container_slice, container)
	}
	return container_slice
}
