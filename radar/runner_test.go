package radar_test

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		pipelineDB     *dbfakes.FakePipelineDB
		scannerFactory *fakes.FakeScannerFactory
		noop           bool
		syncInterval   time.Duration

		initialConfig atc.Config

		process ifrit.Process
	)

	BeforeEach(func() {
		scannerFactory = new(fakes.FakeScannerFactory)
		pipelineDB = new(dbfakes.FakePipelineDB)
		noop = false
		syncInterval = 100 * time.Millisecond

		initialConfig = atc.Config{
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
				},
				{
					Name: "some-other-resource",
				},
			},
		}

		pipelineDB.ScopedNameStub = func(thing string) string {
			return "pipeline:" + thing
		}
		pipelineDB.GetConfigReturns(initialConfig, 1, true, nil)

		scannerFactory.ScannerStub = func(lager.Logger, string) ifrit.Runner {
			return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				<-signals
				return nil
			})
		}
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(NewRunner(
			lagertest.NewTestLogger("test"),
			noop,
			scannerFactory,
			pipelineDB,
			syncInterval,
		))
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("scans for every configured resource", func() {
		Eventually(scannerFactory.ScannerCallCount).Should(Equal(2))

		_, resource := scannerFactory.ScannerArgsForCall(0)
		Expect(resource).To(Equal("some-resource"))

		_, resource = scannerFactory.ScannerArgsForCall(1)
		Expect(resource).To(Equal("some-other-resource"))
	})

	Context("when new resources are configured", func() {
		var updateConfig chan<- atc.Config

		BeforeEach(func() {
			configs := make(chan atc.Config)
			updateConfig = configs

			config := initialConfig

			pipelineDB.GetConfigStub = func() (atc.Config, db.ConfigVersion, bool, error) {
				select {
				case config = <-configs:
				default:
				}

				return config, 1, true, nil
			}
		})

		It("scans for them eventually", func() {
			Eventually(scannerFactory.ScannerCallCount).Should(Equal(2))

			_, resource := scannerFactory.ScannerArgsForCall(0)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scannerFactory.ScannerArgsForCall(1)
			Expect(resource).To(Equal("some-other-resource"))

			newConfig := initialConfig
			newConfig.Resources = append(newConfig.Resources, atc.ResourceConfig{
				Name: "another-resource",
			})

			updateConfig <- newConfig

			Eventually(scannerFactory.ScannerCallCount).Should(Equal(3))

			_, resource = scannerFactory.ScannerArgsForCall(2)
			Expect(resource).To(Equal("another-resource"))

			Consistently(scannerFactory.ScannerCallCount).Should(Equal(3))
		})
	})

	Context("when resources stop being able to check", func() {
		var scannerExit chan struct{}

		BeforeEach(func() {
			scannerExit = make(chan struct{})

			scannerFactory.ScannerStub = func(lager.Logger, string) ifrit.Runner {
				return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)

					select {
					case <-signals:
						return nil
					case <-scannerExit:
						return nil
					}
				})
			}
		})

		It("starts scanning again eventually", func() {
			Eventually(scannerFactory.ScannerCallCount).Should(Equal(2))

			_, resource := scannerFactory.ScannerArgsForCall(0)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scannerFactory.ScannerArgsForCall(1)
			Expect(resource).To(Equal("some-other-resource"))

			close(scannerExit)

			Eventually(scannerFactory.ScannerCallCount, 10*syncInterval).Should(Equal(4))

			_, resource = scannerFactory.ScannerArgsForCall(2)
			Expect(resource).To(Equal("some-resource"))

			_, resource = scannerFactory.ScannerArgsForCall(3)
			Expect(resource).To(Equal("some-other-resource"))
		})
	})

	Context("when in noop mode", func() {
		BeforeEach(func() {
			noop = true
		})

		It("does not start scanning resources", func() {
			Expect(scannerFactory.ScannerCallCount()).To(Equal(0))
		})
	})
})
