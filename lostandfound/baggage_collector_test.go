package lostandfound_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/baggageclaim"
	bcfakes "github.com/concourse/baggageclaim/fakes"
)

var _ = Describe("Baggage Collector", func() {

	var (
		fakeWorkerClient       *wfakes.FakeClient
		fakeWorker             *wfakes.FakeWorker
		fakeBaggageClaimClient *bcfakes.FakeClient

		fakeBaggageCollectorDB *fakes.FakeBaggageCollectorDB
		fakePipelineDBFactory  *dbfakes.FakePipelineDBFactory

		expectedOldResourceGracePeriod = 4 * time.Minute
		expectedLatestVersionTTL       = time.Duration(0)
		expectedOneOffTTL              = 5 * time.Hour

		baggageCollector lostandfound.BaggageCollector
	)

	type resourceConfigAndVersions struct {
		config            atc.ResourceConfig
		versions          []atc.Version
		versionsToDisable []int
	}

	type baggageCollectionExample struct {
		pipelineData map[string][]resourceConfigAndVersions
		volumeData   []db.Volume
		expectedTTLs map[string]time.Duration
	}

	DescribeTable("baggage collection",
		func(examples ...baggageCollectionExample) {
			var err error

			for _, example := range examples {
				fakeWorkerClient = new(wfakes.FakeClient)
				fakeWorker = new(wfakes.FakeWorker)
				fakeBaggageClaimClient = new(bcfakes.FakeClient)
				fakeWorkerClient.GetWorkerReturns(fakeWorker, nil)
				fakeWorker.VolumeManagerReturns(fakeBaggageClaimClient, true)
				baggageCollectorLogger := lagertest.NewTestLogger("test")

				fakeBaggageCollectorDB = new(fakes.FakeBaggageCollectorDB)
				fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)

				baggageCollector = lostandfound.NewBaggageCollector(
					baggageCollectorLogger,
					fakeWorkerClient,
					fakeBaggageCollectorDB,
					fakePipelineDBFactory,
					expectedOldResourceGracePeriod,
					expectedOneOffTTL,
				)

				var savedPipelines []db.SavedPipeline
				fakePipelineDBs := make(map[string]*dbfakes.FakePipelineDB)

				for name, data := range example.pipelineData {
					config := atc.Config{}

					for _, resourceData := range data {
						config.Resources = append(config.Resources, resourceData.config)
					}

					savedPipelines = append(savedPipelines, db.SavedPipeline{
						Pipeline: db.Pipeline{
							Name:   name,
							Config: config,
						},
					})

					fakePipelineDB := new(dbfakes.FakePipelineDB)

					savedVersionsForEachResource := make(map[string][]db.SavedVersionedResource)

					for _, resourceInfo := range data {
						var savedVersions []db.SavedVersionedResource

						for i, version := range resourceInfo.versions {
							disabled := false
							for _, j := range resourceInfo.versionsToDisable {
								if i == j {
									disabled = true
								}
							}

							if !disabled {
								savedVersions = append(savedVersions, db.SavedVersionedResource{
									Enabled: true,
									VersionedResource: db.VersionedResource{
										Version: db.Version(version),
									},
								})
							}
						}
						savedVersionsForEachResource[resourceInfo.config.Name] = savedVersions
					}

					fakePipelineDB.GetLatestEnabledVersionedResourceStub = func(resourceName string) (db.SavedVersionedResource, bool, error) {
						savedVersions := savedVersionsForEachResource[resourceName]

						if len(savedVersions) == 0 {
							return db.SavedVersionedResource{}, false, nil
						}

						return savedVersions[len(savedVersions)-1], true, nil
					}

					fakePipelineDBs[name] = fakePipelineDB
				}

				fakeBaggageCollectorDB.GetAllPipelinesReturns(savedPipelines, nil)

				fakePipelineDBFactory.BuildStub = func(savedPipeline db.SavedPipeline) db.PipelineDB {
					return fakePipelineDBs[savedPipeline.Name]
				}

				fakeVolumes := map[string]*bcfakes.FakeVolume{}

				var savedVolumes []db.SavedVolume
				for _, volume := range example.volumeData {
					savedVolumes = append(savedVolumes, db.SavedVolume{
						Volume: volume,
					})
					fakeVolumes[volume.Handle] = new(bcfakes.FakeVolume)
				}

				fakeBaggageCollectorDB.GetVolumesReturns(savedVolumes, nil)

				fakeBaggageClaimClient.LookupVolumeStub = func(_ lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
					vol, ok := fakeVolumes[handle]
					Expect(ok).To(BeTrue())
					return vol, true, nil
				}

				err = baggageCollector.Collect()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBaggageClaimClient.LookupVolumeCallCount()).To(Equal(len(example.expectedTTLs)))
				var actualHandles []string
				for i := 0; i < fakeBaggageClaimClient.LookupVolumeCallCount(); i++ {
					_, actualHandle := fakeBaggageClaimClient.LookupVolumeArgsForCall(i)
					actualHandles = append(actualHandles, actualHandle)
				}

				var expectedHandles []string
				for handle, expectedTTL := range example.expectedTTLs {
					Expect(fakeVolumes[handle].ReleaseCallCount()).To(Equal(1))
					actualTTL := fakeVolumes[handle].ReleaseArgsForCall(0)
					Expect(actualTTL).To(Equal(worker.FinalTTL(expectedTTL)))
					expectedHandles = append(expectedHandles, handle)
				}

				Expect(actualHandles).To(ConsistOf(expectedHandles))
				Expect(fakeBaggageCollectorDB.SetVolumeTTLCallCount()).To(Equal(len(example.expectedTTLs)))
				actualHandles = nil

				for i := 0; i < fakeBaggageCollectorDB.SetVolumeTTLCallCount(); i++ {
					actualHandle, actualTTL := fakeBaggageCollectorDB.SetVolumeTTLArgsForCall(i)
					actualHandles = append(actualHandles, actualHandle)

					Expect(actualTTL).To(Equal(example.expectedTTLs[actualHandle]))
				}

				Expect(actualHandles).To(ConsistOf(expectedHandles))
			}
		},
		Entry("when there are volumes cached for multiple versions of the resource", baggageCollectionExample{
			pipelineData: map[string][]resourceConfigAndVersions{
				"pipeline-a": []resourceConfigAndVersions{
					{
						config: atc.ResourceConfig{
							Name: "resource-a",
							Type: "some-a-type",
							Source: atc.Source{
								"some": "a-source",
							},
						},
						versions: []atc.Version{
							{"version": "older"},
							{"version": "latest"},
						},
					},
					{
						config: atc.ResourceConfig{
							Name: "resource-b",
							Type: "some-b-type",
							Source: atc.Source{
								"some": "b-source",
							},
						},
						versions: []atc.Version{
							{"version": "older"},
							{"version": "latest"},
						},
					},
				},
			},
			volumeData: []db.Volume{
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-1",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-other-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-2",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-other-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-3",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-b-type{"some":"b-source"}`,
					},
				},
				{
					WorkerName: "some-other-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-4",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-b-type{"some":"b-source"}`,
					},
				},
			},
			expectedTTLs: map[string]time.Duration{
				"some-volume-handle-1": expectedOldResourceGracePeriod,
				"some-volume-handle-3": expectedOldResourceGracePeriod,
			},
		}),
		Entry("it does not update ttls for the latest versions of a resource on each pipeline", baggageCollectionExample{
			pipelineData: map[string][]resourceConfigAndVersions{
				"pipeline-a": []resourceConfigAndVersions{
					{
						config: atc.ResourceConfig{
							Name: "resource-a",
							Type: "some-a-type",
							Source: atc.Source{
								"some": "a-source",
							},
						},
						versions: []atc.Version{
							{"version": "older"},
							{"version": "latest"},
						},
					},
				},
				"pipeline-b": []resourceConfigAndVersions{
					{
						config: atc.ResourceConfig{
							Name: "resource-a",
							Type: "some-a-type",
							Source: atc.Source{
								"some": "a-source",
							},
						},
						versions: []atc.Version{
							{"version": "older"},
							{"version": "latest"},
							{"version": "latest-in-b-but-not-yet-in-a"},
						},
					},
				},
			},
			volumeData: []db.Volume{
				{
					WorkerName: "some-other-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-1",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-other-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-2",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-other-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-3",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest-in-b-but-not-yet-in-a"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
			},
			expectedTTLs: map[string]time.Duration{
				"some-volume-handle-1": expectedOldResourceGracePeriod,
			},
		}),
		Entry("it sets the ttls of disabled versions to soon, and makes the most recent enabled version immortal", baggageCollectionExample{
			pipelineData: map[string][]resourceConfigAndVersions{
				"pipeline-a": []resourceConfigAndVersions{
					{
						config: atc.ResourceConfig{
							Name: "resource-a",
							Type: "some-a-type",
							Source: atc.Source{
								"some": "a-source",
							},
						},
						versions: []atc.Version{
							{"version": "older"},
							{"version": "latest-enabled-version"},
							{"version": "latest-but-disabled"},
							{"version": "latest-but-also-disabled"},
						},
						versionsToDisable: []int{2, 3},
					},
				},
			},
			volumeData: []db.Volume{
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-1",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-2",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest-enabled-version"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-3",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest-but-disabled"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-4",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest-but-also-disabled"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
			},
			expectedTTLs: map[string]time.Duration{
				"some-volume-handle-1": expectedOldResourceGracePeriod,
				"some-volume-handle-2": expectedLatestVersionTTL,
				"some-volume-handle-3": expectedOldResourceGracePeriod,
				"some-volume-handle-4": expectedOldResourceGracePeriod,
			},
		}),
		Entry("it doesn't update the TTL if it's already correct", baggageCollectionExample{
			pipelineData: map[string][]resourceConfigAndVersions{
				"pipeline-a": []resourceConfigAndVersions{
					{
						config: atc.ResourceConfig{
							Name: "resource-a",
							Type: "some-a-type",
							Source: atc.Source{
								"some": "a-source",
							},
						},
						versions: []atc.Version{
							{"version": "oldest"},
							{"version": "even-older-and-disabled"},
							{"version": "older"},
							{"version": "latest"},
						},
						versionsToDisable: []int{1},
					},
				},
			},
			volumeData: []db.Volume{
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-1",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "oldest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-2",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "even-older-and-disabled"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-3",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-4",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
			},
			expectedTTLs: map[string]time.Duration{},
		}),
		Entry("it expires resource versions that are no longer mentioned in the pipeline", baggageCollectionExample{
			pipelineData: map[string][]resourceConfigAndVersions{
				"pipeline-a": []resourceConfigAndVersions{
					{
						config: atc.ResourceConfig{
							Name: "resource-a",
							Type: "some-a-type",
							Source: atc.Source{
								"some": "a-source",
							},
						},
						versions: []atc.Version{
							{"version": "oldest"},
							{"version": "older"},
							{"version": "latest"},
						},
					},
				},
			},
			volumeData: []db.Volume{
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-1",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "oldest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-3",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-4",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-5",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "not-in-pipeline-anymore"},
						ResourceHash:    `some-b-type{"some":"b-source"}`,
					},
				},
			},
			expectedTTLs: map[string]time.Duration{
				"some-volume-handle-5": expectedOldResourceGracePeriod,
			},
		}),
		Entry("it expires volumes even if a resource has no versions", baggageCollectionExample{
			pipelineData: map[string][]resourceConfigAndVersions{
				"pipeline-a": []resourceConfigAndVersions{
					{
						config: atc.ResourceConfig{
							Name: "resource-a",
							Type: "some-a-type",
							Source: atc.Source{
								"some": "a-source",
							},
						},
						versions: []atc.Version{
							{"version": "oldest"},
							{"version": "older"},
							{"version": "latest"},
						},
					},
					{
						config: atc.ResourceConfig{
							Name: "resource-no-versions",
							Type: "some-type",
							Source: atc.Source{
								"some": "some-source",
							},
						},
						versions: []atc.Version{},
					},
				},
			},
			volumeData: []db.Volume{
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-1",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "oldest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedOldResourceGracePeriod,
					Handle:     "some-volume-handle-3",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				{
					WorkerName: "some-worker",
					TTL:        expectedLatestVersionTTL,
					Handle:     "some-volume-handle-4",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
			},
			expectedTTLs: map[string]time.Duration{},
		}),
	)
})
