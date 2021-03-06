package engine_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	execfakes "github.com/concourse/atc/exec/fakes"
)

var _ = Describe("Exec Engine with Try", func() {

	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *fakes.FakeBuildDelegateFactory
		fakeDB              *fakes.FakeEngineDB

		execEngine engine.Engine

		buildModel       db.Build
		expectedMetadata engine.StepMetadata
		logger           *lagertest.TestLogger

		fakeDelegate *fakes.FakeBuildDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(fakes.FakeBuildDelegateFactory)
		fakeDB = new(fakes.FakeEngineDB)

		execEngine = engine.NewExecEngine(fakeFactory, fakeDelegateFactory, fakeDB, "http://example.com")

		fakeDelegate = new(fakes.FakeBuildDelegate)
		fakeDelegateFactory.DelegateReturns(fakeDelegate)

		buildModel = db.Build{
			ID:           84,
			Name:         "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
		}

		expectedMetadata = engine.StepMetadata{
			BuildID:      84,
			BuildName:    "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
			ExternalURL:  "http://example.com",
		}
	})

	Context("running try steps", func() {
		var (
			taskStepFactory *execfakes.FakeStepFactory
			taskStep        *execfakes.FakeStep

			inputStepFactory *execfakes.FakeStepFactory
			inputStep        *execfakes.FakeStep
		)

		BeforeEach(func() {
			taskStepFactory = new(execfakes.FakeStepFactory)
			taskStep = new(execfakes.FakeStep)
			taskStep.ResultStub = successResult(true)
			taskStepFactory.UsingReturns(taskStep)
			fakeFactory.TaskReturns(taskStepFactory)

			inputStepFactory = new(execfakes.FakeStepFactory)
			inputStep = new(execfakes.FakeStep)
			inputStep.ResultStub = successResult(true)
			inputStepFactory.UsingReturns(inputStep)
			fakeFactory.GetReturns(inputStepFactory)
		})

		Context("constructing steps", func() {
			var (
				fakeDelegate          *fakes.FakeBuildDelegate
				fakeInputDelegate     *execfakes.FakeGetDelegate
				fakeExecutionDelegate *execfakes.FakeTaskDelegate
				inputPlan             atc.Plan
				planFactory           atc.PlanFactory
			)

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				fakeDelegate = new(fakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeInputDelegate = new(execfakes.FakeGetDelegate)
				fakeDelegate.InputDelegateReturns(fakeInputDelegate)

				fakeExecutionDelegate = new(execfakes.FakeTaskDelegate)
				fakeDelegate.ExecutionDelegateReturns(fakeExecutionDelegate)

				inputPlan = planFactory.NewPlan(atc.GetPlan{
					Name:     "some-input",
					Pipeline: "some-pipeline",
				})

				plan := planFactory.NewPlan(atc.TryPlan{
					Step: inputPlan,
				})

				build, err := execEngine.CreateBuild(logger, buildModel, plan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)
			})

			It("constructs the step correctly", func() {
				Expect(fakeFactory.GetCallCount()).To(Equal(1))
				logger, metadata, sourceName, workerID, workerMetadata, delegate, _, _, _, _, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(sourceName).To(Equal(exec.SourceName("some-input")))
				Expect(workerMetadata).To(Equal(worker.Metadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-input",
					PipelineName: "some-pipeline",
				}))
				Expect(workerID).To(Equal(worker.Identifier{
					BuildID: 84,
					PlanID:  inputPlan.ID,
				}))

				Expect(delegate).To(Equal(fakeInputDelegate))
				_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
				Expect(location).NotTo(BeNil())
			})
		})

		Context("when the inner step fails", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.ResultStub = successResult(false)
			})

			It("runs the next step", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.TryPlan{
						Step: planFactory.NewPlan(atc.GetPlan{
							Name: "some-input",
						}),
					}),
					Next: planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-resource",
						Config: &atc.TaskConfig{},
					}),
				})

				build, err := execEngine.CreateBuild(logger, buildModel, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))
				Expect(inputStep.ReleaseCallCount()).To(BeNumerically(">", 0))

				Expect(taskStep.RunCallCount()).To(Equal(1))
				Expect(inputStep.ReleaseCallCount()).To(BeNumerically(">", 0))
			})
		})
	})
})
