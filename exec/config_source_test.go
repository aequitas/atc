package exec_test

import (
	"errors"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ConfigSource", func() {
	var (
		taskConfig atc.TaskConfig
		taskPlan   atc.TaskPlan
		repo       *SourceRepository
	)

	BeforeEach(func() {
		repo = NewSourceRepository()
		taskConfig = atc.TaskConfig{
			Platform: "some-platform",
			Image:    "some-image",
			ImageResource: &atc.TaskImageConfig{
				Type:   "docker",
				Source: atc.Source{"a": "b"},
			},
			Params: atc.Params{
				"task-config-param-key": "task-config-param-val-1",
				"common-key":            "task-config-param-val-2",
			},
			Run: atc.TaskRunConfig{
				Path: "ls",
				Args: []string{"-al"},
			},
			Inputs: []atc.TaskInputConfig{
				{Name: "some-input", Path: "some-path"},
			},
		}

		taskPlan = atc.TaskPlan{
			Params: atc.Params{
				"task-plan-param-key": "task-plan-param-val-1",
				"common-key":          "task-plan-param-val-2",
			},
			Config: &taskConfig,
		}
	})

	Describe("DeprecationConfigSource", func() {
		var (
			configSource TaskConfigSource
			stderrBuf    *gbytes.Buffer
		)

		JustBeforeEach(func() {
			delegate := StaticConfigSource{Plan: taskPlan}
			stderrBuf = gbytes.NewBuffer()
			configSource = DeprecationConfigSource{
				Delegate: &delegate,
				Stderr:   stderrBuf,
			}
		})

		It("merges task params prefering params in task plan", func() {
			fetchedConfig, err := configSource.FetchConfig(repo)
			Expect(err).ToNot(HaveOccurred())
			Expect(fetchedConfig.Params).To(Equal(map[string]string{
				"task-plan-param-key":   "task-plan-param-val-1",
				"task-config-param-key": "task-config-param-val-1",
				"common-key":            "task-plan-param-val-2",
			}))
		})

		Context("when task config params are not set", func() {
			BeforeEach(func() {
				taskConfig = atc.TaskConfig{}
			})

			It("uses params from task plan", func() {
				fetchedConfig, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(fetchedConfig.Params).To(Equal(map[string]string{
					"task-plan-param-key": "task-plan-param-val-1",
					"common-key":          "task-plan-param-val-2",
				}))
			})
		})

		Context("when task plan params are not set", func() {
			BeforeEach(func() {
				taskPlan = atc.TaskPlan{
					Config: &taskConfig,
				}
			})

			It("uses params from task config", func() {
				fetchedConfig, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(fetchedConfig.Params).To(Equal(map[string]string{
					"task-config-param-key": "task-config-param-val-1",
					"common-key":            "task-config-param-val-2",
				}))
			})
		})

		Context("when task plan config and task config file are set", func() {
			BeforeEach(func() {
				taskPlan.ConfigPath = "task-config-path"
			})

			It("writes warning to stderr", func() {
				_, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(stderrBuf).To(gbytes.Say("DEPRECATION WARNING: Specifying both `file` and `config.params` in a task step is deprecated, use params on task step directly"))
			})
		})
	})

	Describe("StaticConfigSource", func() {
		var (
			configSource TaskConfigSource
		)

		JustBeforeEach(func() {
			configSource = StaticConfigSource{Plan: taskPlan}
		})

		It("merges task params prefering params in task plan", func() {
			fetchedConfig, err := configSource.FetchConfig(repo)
			Expect(err).ToNot(HaveOccurred())
			Expect(fetchedConfig.Params).To(Equal(map[string]string{
				"task-plan-param-key":   "task-plan-param-val-1",
				"task-config-param-key": "task-config-param-val-1",
				"common-key":            "task-plan-param-val-2",
			}))
		})

		Context("when task config params are not set", func() {
			BeforeEach(func() {
				taskConfig = atc.TaskConfig{}
			})

			It("uses params from task plan", func() {
				fetchedConfig, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(fetchedConfig.Params).To(Equal(map[string]string{
					"task-plan-param-key": "task-plan-param-val-1",
					"common-key":          "task-plan-param-val-2",
				}))
			})
		})

		Context("when task plan params are not set", func() {
			BeforeEach(func() {
				taskPlan = atc.TaskPlan{
					Config: &taskConfig,
				}
			})

			It("uses params from task config", func() {
				fetchedConfig, err := configSource.FetchConfig(repo)
				Expect(err).ToNot(HaveOccurred())
				Expect(fetchedConfig.Params).To(Equal(map[string]string{
					"task-config-param-key": "task-config-param-val-1",
					"common-key":            "task-config-param-val-2",
				}))
			})
		})
	})

	Describe("FileConfigSource", func() {
		var (
			configSource FileConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			configSource = FileConfigSource{Path: "some/build.yml"}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		Context("when the path does not indicate an artifact source", func() {
			BeforeEach(func() {
				configSource.Path = "foo-bar.yml"
			})

			It("returns an error", func() {
				Expect(fetchErr).To(Equal(UnspecifiedArtifactSourceError{"foo-bar.yml"}))
			})
		})

		Context("when the file's artifact source can be found in the repository", func() {
			var fakeArtifactSource *fakes.FakeArtifactSource

			BeforeEach(func() {
				fakeArtifactSource = new(fakes.FakeArtifactSource)
				repo.RegisterSource("some", fakeArtifactSource)
			})

			Context("when the artifact source provides a proper file", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					marshalled, err := yaml.Marshal(taskConfig)
					Expect(err).NotTo(HaveOccurred())

					streamedOut = gbytes.BufferWithBytes(marshalled)
					fakeArtifactSource.StreamFileReturns(streamedOut, nil)
				})

				It("fetches the file via the correct path", func() {
					Expect(fakeArtifactSource.StreamFileArgsForCall(0)).To(Equal("build.yml"))
				})

				It("succeeds", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("returns the unmarshalled config", func() {
					Expect(fetchedConfig).To(Equal(taskConfig))
				})

				It("closes the stream", func() {
					Expect(streamedOut.Closed()).To(BeTrue())
				})
			})

			Context("when the artifact source provides an invalid configuration", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					invalidConfig := taskConfig
					invalidConfig.Platform = ""
					invalidConfig.Run = atc.TaskRunConfig{}

					marshalled, err := yaml.Marshal(invalidConfig)
					Expect(err).NotTo(HaveOccurred())

					streamedOut = gbytes.BufferWithBytes(marshalled)
					fakeArtifactSource.StreamFileReturns(streamedOut, nil)
				})

				It("returns an error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})
			})

			Context("when the artifact source provides a malformed file", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					streamedOut = gbytes.BufferWithBytes([]byte("bogus"))
					fakeArtifactSource.StreamFileReturns(streamedOut, nil)
				})

				It("fails", func() {
					Expect(fetchErr).To(HaveOccurred())
				})

				It("closes the stream", func() {
					Expect(streamedOut.Closed()).To(BeTrue())
				})
			})

			Context("when the artifact source provides a valid file with invalid keys", func() {
				var streamedOut *gbytes.Buffer

				BeforeEach(func() {
					streamedOut = gbytes.BufferWithBytes([]byte(`
platform: beos

intputs: []

run: {path: a/file}
`))
					fakeArtifactSource.StreamFileReturns(streamedOut, nil)
				})

				It("fails", func() {
					Expect(fetchErr).To(HaveOccurred())
				})

				It("closes the stream", func() {
					Expect(streamedOut.Closed()).To(BeTrue())
				})
			})

			Context("when streaming the file out fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeArtifactSource.StreamFileReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})
			})
		})

		Context("when the file's artifact source cannot be found in the repository", func() {
			It("returns an UnknownArtifactSourceError", func() {
				Expect(fetchErr).To(Equal(UnknownArtifactSourceError{"some"}))
			})
		})
	})

	Describe("MergedConfigSource", func() {
		var (
			fakeConfigSourceA *fakes.FakeTaskConfigSource
			fakeConfigSourceB *fakes.FakeTaskConfigSource

			configSource TaskConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error

			configA atc.TaskConfig
			configB atc.TaskConfig
		)

		BeforeEach(func() {
			fakeConfigSourceA = new(fakes.FakeTaskConfigSource)
			fakeConfigSourceB = new(fakes.FakeTaskConfigSource)

			configSource = MergedConfigSource{
				A: fakeConfigSourceA,
				B: fakeConfigSourceB,
			}

			configA = atc.TaskConfig{
				Platform: "some-platform",
				Image:    "some-image",
				Params:   map[string]string{"PARAM": "A"},
				Run: atc.TaskRunConfig{
					Path: "echo",
					Args: []string{"bananapants"},
				},
			}
			configB = atc.TaskConfig{
				Params: map[string]string{"PARAM": "B"},
			}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		Context("when fetching via A succeeds", func() {
			BeforeEach(func() {
				fakeConfigSourceA.FetchConfigReturns(configA, nil)
			})

			Context("and fetching via B succeeds", func() {
				BeforeEach(func() {
					fakeConfigSourceB.FetchConfigReturns(configB, nil)
				})

				It("fetches via the input source", func() {
					Expect(fakeConfigSourceA.FetchConfigArgsForCall(0)).To(Equal(repo))
					Expect(fakeConfigSourceB.FetchConfigArgsForCall(0)).To(Equal(repo))
				})

				It("succeeds", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("returns the merged config", func() {
					Expect(fetchedConfig).To(Equal(atc.TaskConfig{
						Platform: "some-platform",
						Image:    "some-image",
						Params:   map[string]string{"PARAM": "B"},
						Run: atc.TaskRunConfig{
							Path: "echo",
							Args: []string{"bananapants"},
						},
					}))
				})

			})

			Context("and fetching via B fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeConfigSourceB.FetchConfigReturns(atc.TaskConfig{}, disaster)
				})

				It("returns the error", func() {
					Expect(fetchErr).To(Equal(disaster))
				})
			})
		})

		Context("when fetching via A fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeConfigSourceA.FetchConfigReturns(atc.TaskConfig{}, disaster)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})

			It("does not fetch via B", func() {
				Expect(fakeConfigSourceB.FetchConfigCallCount()).To(Equal(0))
			})
		})
	})

	Describe("ValidatingConfigSource", func() {
		var (
			fakeConfigSource *fakes.FakeTaskConfigSource

			configSource TaskConfigSource

			fetchedConfig atc.TaskConfig
			fetchErr      error
		)

		BeforeEach(func() {
			fakeConfigSource = new(fakes.FakeTaskConfigSource)

			configSource = ValidatingConfigSource{fakeConfigSource}
		})

		JustBeforeEach(func() {
			fetchedConfig, fetchErr = configSource.FetchConfig(repo)
		})

		Context("when the config is valid", func() {
			config := atc.TaskConfig{
				Platform: "some-platform",
				Image:    "some-image",
				Params:   map[string]string{"PARAM": "A"},
				Run: atc.TaskRunConfig{
					Path: "echo",
					Args: []string{"bananapants"},
				},
			}

			BeforeEach(func() {
				fakeConfigSource.FetchConfigReturns(config, nil)
			})

			It("returns the config and no error", func() {
				Expect(fetchErr).ToNot(HaveOccurred())
				Expect(fetchedConfig).To(Equal(config))
			})
		})

		Context("when the config is invalid", func() {
			BeforeEach(func() {
				fakeConfigSource.FetchConfigReturns(atc.TaskConfig{
					Image:  "some-image",
					Params: map[string]string{"PARAM": "A"},
					Run: atc.TaskRunConfig{
						Args: []string{"bananapants"},
					},
				}, nil)
			})

			It("returns the validation error", func() {
				Expect(fetchErr).To(HaveOccurred())
			})
		})

		Context("when fetching the config fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeConfigSource.FetchConfigReturns(atc.TaskConfig{}, disaster)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})
		})
	})
})
