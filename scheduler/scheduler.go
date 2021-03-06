package scheduler

import (
	"sync"
	"time"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/engine"
)

//go:generate counterfeiter . PipelineDB

type PipelineDB interface {
	JobServiceDB
	CreateJobBuild(job string) (db.Build, error)
	CreateJobBuildForCandidateInputs(job string) (db.Build, bool, error)
	UpdateBuildToScheduled(buildID int) (bool, error)

	GetJobBuildForInputs(job string, inputs []db.BuildInput) (db.Build, bool, error)
	GetNextPendingBuild(job string) (db.Build, bool, error)

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
}

//go:generate counterfeiter . BuildsDB

type BuildsDB interface {
	LeaseBuildScheduling(buildID int, interval time.Duration) (db.Lease, bool, error)
	ErrorBuild(buildID int, err error) error
	FinishBuild(int, db.Status) error

	GetBuildPreparation(buildID int) (db.BuildPreparation, bool, error)
}

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.ResourceTypes, []db.BuildInput) (atc.Plan, error)
}

type Waiter interface {
	Wait()
}

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger, string) error
}

type Scheduler struct {
	PipelineDB PipelineDB
	BuildsDB   BuildsDB
	Factory    BuildFactory
	Engine     engine.Engine
	Scanner    Scanner
}

func (s *Scheduler) BuildLatestInputs(logger lager.Logger, versions *algorithm.VersionsDB, job atc.JobConfig, resources atc.ResourceConfigs, resourceTypes atc.ResourceTypes) error {
	logger = logger.Session("build-latest")

	inputs := config.JobInputs(job)

	if len(inputs) == 0 {
		// no inputs; no-op
		return nil
	}

	latestInputs, found, err := s.PipelineDB.GetLatestInputVersions(versions, job.Name, inputs)
	if err != nil {
		logger.Error("failed-to-get-latest-input-versions", err)
		return err
	}

	if !found {
		logger.Debug("no-input-versions-available")
		return nil
	}

	checkInputs := []db.BuildInput{}
	for _, input := range latestInputs {
		for _, ji := range inputs {
			if ji.Name == input.Name {
				if ji.Trigger {
					checkInputs = append(checkInputs, input)
				}

				break
			}
		}
	}

	if len(checkInputs) == 0 {
		logger.Debug("no-triggered-input-versions")
		return nil
	}

	existingBuild, found, err := s.PipelineDB.GetJobBuildForInputs(job.Name, checkInputs)
	if err != nil {
		logger.Error("could-not-determine-if-inputs-are-already-used", err)
		return err
	}

	if found {
		logger.Debug("build-already-exists-for-inputs", lager.Data{
			"existing-build": existingBuild.ID,
		})

		return nil
	}

	build, created, err := s.PipelineDB.CreateJobBuildForCandidateInputs(job.Name)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return err
	}

	if !created {
		logger.Debug("waiting-for-existing-build-to-determine-inputs", lager.Data{
			"existing-build": build.ID,
		})
		return nil
	}

	logger.Debug("created-build", lager.Data{"build": build.ID})

	jobService, err := NewJobService(job, s.PipelineDB, s.Scanner)
	if err != nil {
		logger.Error("failed-to-get-job-service", err)
		return nil
	}

	// NOTE: this is intentionally serial within a scheduler tick, so that
	// multiple ATCs don't do redundant work to determine a build's inputs.

	s.ScheduleAndResumePendingBuild(logger, versions, build, job, resources, resourceTypes, jobService)

	return nil
}

func (s *Scheduler) TryNextPendingBuild(logger lager.Logger, versions *algorithm.VersionsDB, job atc.JobConfig, resources atc.ResourceConfigs, resourceTypes atc.ResourceTypes) Waiter {
	logger = logger.Session("try-next-pending")

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()

		build, found, err := s.PipelineDB.GetNextPendingBuild(job.Name)
		if err != nil {
			logger.Error("failed-to-get-next-pending-build", err)
			return
		}

		if !found {
			return
		}

		jobService, err := NewJobService(job, s.PipelineDB, s.Scanner)
		if err != nil {
			logger.Error("failed-to-get-job-service", err)
			return
		}

		s.ScheduleAndResumePendingBuild(logger, versions, build, job, resources, resourceTypes, jobService)
	}()

	return wg
}

func (s *Scheduler) TriggerImmediately(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs, resourceTypes atc.ResourceTypes) (db.Build, Waiter, error) {
	logger = logger.Session("trigger-immediately")

	build, err := s.PipelineDB.CreateJobBuild(job.Name)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return db.Build{}, nil, err
	}

	jobService, err := NewJobService(job, s.PipelineDB, s.Scanner)
	if err != nil {
		return db.Build{}, nil, err
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)

	// do not block request on scanning input versions
	go func() {
		defer wg.Done()
		s.ScheduleAndResumePendingBuild(logger, nil, build, job, resources, resourceTypes, jobService)
	}()

	return build, wg, nil
}

func (s *Scheduler) updateBuildToScheduled(logger lager.Logger, canBuildBeScheduled bool, buildID int, reason string) bool {
	if canBuildBeScheduled {
		updated, err := s.PipelineDB.UpdateBuildToScheduled(buildID)
		if err != nil {
			logger.Error("failed-to-update-build-to-scheduled", err, lager.Data{"build-id": buildID})
			return false
		}

		if !updated {
			logger.Debug("unable-to-update-build-to-scheduled", lager.Data{"build-id": buildID})
			return false
		}

		return true
	}
	logger.Debug("build-could-not-be-scheduled", lager.Data{
		"build-id": buildID,
		"reason":   reason,
	})
	return false
}

func (s *Scheduler) ScheduleAndResumePendingBuild(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	build db.Build,
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
	jobService JobService,
) engine.Build {
	lease, acquired, err := s.BuildsDB.LeaseBuildScheduling(build.ID, 10*time.Second)
	if err != nil {
		logger.Error("failed-to-get-lease", err)
		return nil
	}

	if !acquired {
		return nil
	}

	defer lease.Break()

	logger = logger.WithData(lager.Data{"build": build.ID})

	buildPrep, found, err := s.BuildsDB.GetBuildPreparation(build.ID)
	if err != nil {
		logger.Error("failed-to-get-build-prep", err, lager.Data{"build-id": build.ID})
		return nil
	}

	if !found {
		logger.Debug("failed-to-find-build-prep", lager.Data{"build-id": build.ID})
		return nil
	}

	inputs, canBuildBeScheduled, reason, err := jobService.CanBuildBeScheduled(logger, build, buildPrep, versions)
	if err != nil {
		logger.Error("failed-to-schedule-build", err, lager.Data{
			"build-id": build.ID,
			"reason":   reason,
		})

		if reason == "failed-to-scan" {
			err = s.BuildsDB.ErrorBuild(build.ID, err)
			if err != nil {
				logger.Error("failed-to-mark-build-as-errored", err)
			}
		}
		return nil
	}

	if !s.updateBuildToScheduled(logger, canBuildBeScheduled, build.ID, reason) {
		return nil
	}

	plan, err := s.Factory.Create(job, resources, resourceTypes, inputs)
	if err != nil {
		// Don't use ErrorBuild because it logs a build event, and this build hasn't started
		err := s.BuildsDB.FinishBuild(build.ID, db.StatusErrored)
		if err != nil {
			logger.Error("failed-to-mark-build-as-errored", err)
		}
		return nil
	}

	createdBuild, err := s.Engine.CreateBuild(logger, build, plan)
	if err != nil {
		logger.Error("failed-to-create-build", err)
		return nil
	}

	if createdBuild != nil {
		logger.Info("building")
		go createdBuild.Resume(logger)
	}

	return createdBuild
}
