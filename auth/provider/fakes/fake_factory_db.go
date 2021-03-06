// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/db"
)

type FakeFactoryDB struct {
	GetTeamByNameStub        func(teamName string) (db.SavedTeam, bool, error)
	getTeamByNameMutex       sync.RWMutex
	getTeamByNameArgsForCall []struct {
		teamName string
	}
	getTeamByNameReturns struct {
		result1 db.SavedTeam
		result2 bool
		result3 error
	}
}

func (fake *FakeFactoryDB) GetTeamByName(teamName string) (db.SavedTeam, bool, error) {
	fake.getTeamByNameMutex.Lock()
	fake.getTeamByNameArgsForCall = append(fake.getTeamByNameArgsForCall, struct {
		teamName string
	}{teamName})
	fake.getTeamByNameMutex.Unlock()
	if fake.GetTeamByNameStub != nil {
		return fake.GetTeamByNameStub(teamName)
	} else {
		return fake.getTeamByNameReturns.result1, fake.getTeamByNameReturns.result2, fake.getTeamByNameReturns.result3
	}
}

func (fake *FakeFactoryDB) GetTeamByNameCallCount() int {
	fake.getTeamByNameMutex.RLock()
	defer fake.getTeamByNameMutex.RUnlock()
	return len(fake.getTeamByNameArgsForCall)
}

func (fake *FakeFactoryDB) GetTeamByNameArgsForCall(i int) string {
	fake.getTeamByNameMutex.RLock()
	defer fake.getTeamByNameMutex.RUnlock()
	return fake.getTeamByNameArgsForCall[i].teamName
}

func (fake *FakeFactoryDB) GetTeamByNameReturns(result1 db.SavedTeam, result2 bool, result3 error) {
	fake.GetTeamByNameStub = nil
	fake.getTeamByNameReturns = struct {
		result1 db.SavedTeam
		result2 bool
		result3 error
	}{result1, result2, result3}
}

var _ provider.FactoryDB = new(FakeFactoryDB)
