package exec_test

import (
	"errors"
	"os"
	"time"

	. "github.com/concourse/atc/exec"
	"github.com/pivotal-golang/clock/fakeclock"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/ifrit"
)

var _ = Describe("Timeout Step", func() {
	var (
		fakeStepFactoryStep *fakes.FakeStepFactory

		runStep *fakes.FakeStep

		timeout StepFactory
		step    Step

		startStep chan error
		process   ifrit.Process

		timeoutDuration string
		fakeClock       *fakeclock.FakeClock
	)

	BeforeEach(func() {
		startStep = make(chan error, 1)
		fakeStepFactoryStep = new(fakes.FakeStepFactory)
		runStep = new(fakes.FakeStep)
		fakeStepFactoryStep.UsingReturns(runStep)

		timeoutDuration = "1h"
		fakeClock = fakeclock.NewFakeClock(time.Now())
	})

	JustBeforeEach(func() {
		timeout = Timeout(fakeStepFactoryStep, timeoutDuration, fakeClock)
		step = timeout.Using(nil, nil)
		process = ifrit.Background(step)
	})

	Context("when the duration is invalid", func() {
		BeforeEach(func() {
			timeoutDuration = "nope"
		})

		It("errors immediately", func() {
			Expect(<-process.Wait()).To(HaveOccurred())
			Expect(process.Ready()).ToNot(BeClosed())
		})
	})

	Context("when the process goes beyond the duration", func() {
		var receivedSignals <-chan os.Signal

		BeforeEach(func() {
			s := make(chan os.Signal, 1)
			receivedSignals = s

			runStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				fakeClock.Increment(time.Hour)
				s <- <-signals
				return nil
			}
		})

		It("interrupts it", func() {
			<-process.Wait()

			Expect(receivedSignals).To(Receive(Equal(os.Interrupt)))
		})

		It("exits with no error", func() {
			Expect(<-process.Wait()).ToNot(HaveOccurred())
		})

		Describe("result", func() {
			It("is not successful", func() {
				Eventually(runStep.RunCallCount).Should(Equal(1))

				Expect(<-process.Wait()).To(Succeed())

				var success Success
				Expect(step.Result(&success)).To(BeTrue())
				Expect(bool(success)).To(BeFalse())
			})
		})
	})

	Context("when the step returns an error", func() {
		var someError error

		BeforeEach(func() {
			someError = errors.New("some error")
			runStep.ResultStub = successResult(false)
			runStep.RunReturns(someError)
		})

		It("returns the error", func() {
			var receivedError error
			Eventually(process.Wait()).Should(Receive(&receivedError))
			Expect(receivedError).NotTo(BeNil())
			Expect(receivedError).To(Equal(someError))
		})
	})

	Context("when the step completes within the duration", func() {
		BeforeEach(func() {
			runStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				fakeClock.Increment(time.Hour / 2)
				return nil
			}
		})

		It("does not interrupt it", func() {
			<-process.Wait()

			Expect(runStep.RunCallCount()).To(Equal(1))

			subSignals, _ := runStep.RunArgsForCall(0)
			Expect(subSignals).ToNot(Receive())
		})

		It("exits with no error", func() {
			Expect(<-process.Wait()).ToNot(HaveOccurred())
		})

		Context("when the step is successful", func() {
			BeforeEach(func() {
				runStep.ResultStub = successResult(true)
			})

			It("is successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var success Success
				Expect(step.Result(&success)).To(BeTrue())
				Expect(bool(success)).To(BeTrue())
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				runStep.ResultStub = successResult(false)
			})

			It("is not successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var success Success
				Expect(step.Result(&success)).To(BeTrue())
				Expect(bool(success)).To(BeFalse())
			})
		})

		Describe("signalling", func() {
			var receivedSignals <-chan os.Signal

			BeforeEach(func() {
				s := make(chan os.Signal, 1)
				receivedSignals = s

				runStep.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					fakeClock.Increment(time.Hour / 2)
					s <- <-signals
					return nil
				}
			})

			It("forwards the signal down", func() {
				process.Signal(os.Kill)

				Expect(<-process.Wait()).ToNot(HaveOccurred())
				Expect(<-receivedSignals).To(Equal(os.Kill))
			})
		})
	})
})
