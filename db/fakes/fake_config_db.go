// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

type FakeConfigDB struct {
	GetConfigStub        func(teamName, pipelineName string) (atc.Config, db.ConfigVersion, error)
	getConfigMutex       sync.RWMutex
	getConfigArgsForCall []struct {
		teamName     string
		pipelineName string
	}
	getConfigReturns struct {
		result1 atc.Config
		result2 db.ConfigVersion
		result3 error
	}
	SaveConfigStub        func(string, string, atc.Config, db.ConfigVersion, db.PipelinePausedState) (db.SavedPipeline, bool, error)
	saveConfigMutex       sync.RWMutex
	saveConfigArgsForCall []struct {
		arg1 string
		arg2 string
		arg3 atc.Config
		arg4 db.ConfigVersion
		arg5 db.PipelinePausedState
	}
	saveConfigReturns struct {
		result1 db.SavedPipeline
		result2 bool
		result3 error
	}
}

func (fake *FakeConfigDB) GetConfig(teamName string, pipelineName string) (atc.Config, db.ConfigVersion, error) {
	fake.getConfigMutex.Lock()
	fake.getConfigArgsForCall = append(fake.getConfigArgsForCall, struct {
		teamName     string
		pipelineName string
	}{teamName, pipelineName})
	fake.getConfigMutex.Unlock()
	if fake.GetConfigStub != nil {
		return fake.GetConfigStub(teamName, pipelineName)
	} else {
		return fake.getConfigReturns.result1, fake.getConfigReturns.result2, fake.getConfigReturns.result3
	}
}

func (fake *FakeConfigDB) GetConfigCallCount() int {
	fake.getConfigMutex.RLock()
	defer fake.getConfigMutex.RUnlock()
	return len(fake.getConfigArgsForCall)
}

func (fake *FakeConfigDB) GetConfigArgsForCall(i int) (string, string) {
	fake.getConfigMutex.RLock()
	defer fake.getConfigMutex.RUnlock()
	return fake.getConfigArgsForCall[i].teamName, fake.getConfigArgsForCall[i].pipelineName
}

func (fake *FakeConfigDB) GetConfigReturns(result1 atc.Config, result2 db.ConfigVersion, result3 error) {
	fake.GetConfigStub = nil
	fake.getConfigReturns = struct {
		result1 atc.Config
		result2 db.ConfigVersion
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeConfigDB) SaveConfig(arg1 string, arg2 string, arg3 atc.Config, arg4 db.ConfigVersion, arg5 db.PipelinePausedState) (db.SavedPipeline, bool, error) {
	fake.saveConfigMutex.Lock()
	fake.saveConfigArgsForCall = append(fake.saveConfigArgsForCall, struct {
		arg1 string
		arg2 string
		arg3 atc.Config
		arg4 db.ConfigVersion
		arg5 db.PipelinePausedState
	}{arg1, arg2, arg3, arg4, arg5})
	fake.saveConfigMutex.Unlock()
	if fake.SaveConfigStub != nil {
		return fake.SaveConfigStub(arg1, arg2, arg3, arg4, arg5)
	} else {
		return fake.saveConfigReturns.result1, fake.saveConfigReturns.result2, fake.saveConfigReturns.result3
	}
}

func (fake *FakeConfigDB) SaveConfigCallCount() int {
	fake.saveConfigMutex.RLock()
	defer fake.saveConfigMutex.RUnlock()
	return len(fake.saveConfigArgsForCall)
}

func (fake *FakeConfigDB) SaveConfigArgsForCall(i int) (string, string, atc.Config, db.ConfigVersion, db.PipelinePausedState) {
	fake.saveConfigMutex.RLock()
	defer fake.saveConfigMutex.RUnlock()
	return fake.saveConfigArgsForCall[i].arg1, fake.saveConfigArgsForCall[i].arg2, fake.saveConfigArgsForCall[i].arg3, fake.saveConfigArgsForCall[i].arg4, fake.saveConfigArgsForCall[i].arg5
}

func (fake *FakeConfigDB) SaveConfigReturns(result1 db.SavedPipeline, result2 bool, result3 error) {
	fake.SaveConfigStub = nil
	fake.saveConfigReturns = struct {
		result1 db.SavedPipeline
		result2 bool
		result3 error
	}{result1, result2, result3}
}

var _ db.ConfigDB = new(FakeConfigDB)
