// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/pivotal-golang/lager"
)

type FakeEngine struct {
	NameStub        func() string
	nameMutex       sync.RWMutex
	nameArgsForCall []struct{}
	nameReturns     struct {
		result1 string
	}
	CreateBuildStub        func(lager.Logger, db.Build, atc.Plan) (engine.Build, error)
	createBuildMutex       sync.RWMutex
	createBuildArgsForCall []struct {
		arg1 lager.Logger
		arg2 db.Build
		arg3 atc.Plan
	}
	createBuildReturns struct {
		result1 engine.Build
		result2 error
	}
	LookupBuildStub        func(lager.Logger, db.Build) (engine.Build, error)
	lookupBuildMutex       sync.RWMutex
	lookupBuildArgsForCall []struct {
		arg1 lager.Logger
		arg2 db.Build
	}
	lookupBuildReturns struct {
		result1 engine.Build
		result2 error
	}
}

func (fake *FakeEngine) Name() string {
	fake.nameMutex.Lock()
	fake.nameArgsForCall = append(fake.nameArgsForCall, struct{}{})
	fake.nameMutex.Unlock()
	if fake.NameStub != nil {
		return fake.NameStub()
	} else {
		return fake.nameReturns.result1
	}
}

func (fake *FakeEngine) NameCallCount() int {
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	return len(fake.nameArgsForCall)
}

func (fake *FakeEngine) NameReturns(result1 string) {
	fake.NameStub = nil
	fake.nameReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeEngine) CreateBuild(arg1 lager.Logger, arg2 db.Build, arg3 atc.Plan) (engine.Build, error) {
	fake.createBuildMutex.Lock()
	fake.createBuildArgsForCall = append(fake.createBuildArgsForCall, struct {
		arg1 lager.Logger
		arg2 db.Build
		arg3 atc.Plan
	}{arg1, arg2, arg3})
	fake.createBuildMutex.Unlock()
	if fake.CreateBuildStub != nil {
		return fake.CreateBuildStub(arg1, arg2, arg3)
	} else {
		return fake.createBuildReturns.result1, fake.createBuildReturns.result2
	}
}

func (fake *FakeEngine) CreateBuildCallCount() int {
	fake.createBuildMutex.RLock()
	defer fake.createBuildMutex.RUnlock()
	return len(fake.createBuildArgsForCall)
}

func (fake *FakeEngine) CreateBuildArgsForCall(i int) (lager.Logger, db.Build, atc.Plan) {
	fake.createBuildMutex.RLock()
	defer fake.createBuildMutex.RUnlock()
	return fake.createBuildArgsForCall[i].arg1, fake.createBuildArgsForCall[i].arg2, fake.createBuildArgsForCall[i].arg3
}

func (fake *FakeEngine) CreateBuildReturns(result1 engine.Build, result2 error) {
	fake.CreateBuildStub = nil
	fake.createBuildReturns = struct {
		result1 engine.Build
		result2 error
	}{result1, result2}
}

func (fake *FakeEngine) LookupBuild(arg1 lager.Logger, arg2 db.Build) (engine.Build, error) {
	fake.lookupBuildMutex.Lock()
	fake.lookupBuildArgsForCall = append(fake.lookupBuildArgsForCall, struct {
		arg1 lager.Logger
		arg2 db.Build
	}{arg1, arg2})
	fake.lookupBuildMutex.Unlock()
	if fake.LookupBuildStub != nil {
		return fake.LookupBuildStub(arg1, arg2)
	} else {
		return fake.lookupBuildReturns.result1, fake.lookupBuildReturns.result2
	}
}

func (fake *FakeEngine) LookupBuildCallCount() int {
	fake.lookupBuildMutex.RLock()
	defer fake.lookupBuildMutex.RUnlock()
	return len(fake.lookupBuildArgsForCall)
}

func (fake *FakeEngine) LookupBuildArgsForCall(i int) (lager.Logger, db.Build) {
	fake.lookupBuildMutex.RLock()
	defer fake.lookupBuildMutex.RUnlock()
	return fake.lookupBuildArgsForCall[i].arg1, fake.lookupBuildArgsForCall[i].arg2
}

func (fake *FakeEngine) LookupBuildReturns(result1 engine.Build, result2 error) {
	fake.LookupBuildStub = nil
	fake.lookupBuildReturns = struct {
		result1 engine.Build
		result2 error
	}{result1, result2}
}

var _ engine.Engine = new(FakeEngine)
