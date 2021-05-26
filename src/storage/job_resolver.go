package storage

import (
	"github.com/elliotchance/orderedmap"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

func newJobResolver() *JobResolver {
	return &JobResolver{
		appIdToSignJobMap:   orderedmap.NewOrderedMap(),
		idToReturnJobMap:    map[string]*ReturnJob{},
		appIdToReturnJobMap: map[string]*ReturnJob{},
	}
}

type JobResolver struct {
	mu                  sync.Mutex
	appIdToSignJobMap   *orderedmap.OrderedMap
	idToReturnJobMap    map[string]*ReturnJob
	appIdToReturnJobMap map[string]*ReturnJob
}

// User bundle ID is unused if the profile is not an account.
func (r *JobResolver) MakeSignJob(appId string, profileId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appIdToSignJobMap.Set(appId, signJob{
		ts:        time.Now(),
		appId:     appId,
		profileId: profileId,
	})
}

var ErrNotFound = errors.New("not found")

func (r *JobResolver) TakeLastJob(writer io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.appIdToSignJobMap.Len() < 1 {
		return errors.WithMessage(ErrNotFound, "sign job")
	}

	elem := r.appIdToSignJobMap.Back()
	r.appIdToSignJobMap.Delete(elem.Key)
	job := elem.Value.(signJob)

	returnJobId, err := job.writeArchive(writer)
	if err != nil {
		return errors.WithMessage(err, "write archive")
	}
	returnJob := ReturnJob{Id: returnJobId, Ts: time.Now(), AppId: job.appId}
	r.idToReturnJobMap[returnJobId] = &returnJob
	r.appIdToReturnJobMap[job.appId] = &returnJob
	return nil
}

func (r *JobResolver) GetAll() []*ReturnJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	var jobs []*ReturnJob
	for _, job := range r.idToReturnJobMap {
		jobs = append(jobs, job)
	}
	return jobs
}

func (r *JobResolver) IsPendingForAppId(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.appIdToSignJobMap.Get(id)
	return ok
}

func (r *JobResolver) GetById(id string) (*ReturnJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.idToReturnJobMap[id]
	return job, ok
}

func (r *JobResolver) GetByAppId(id string) (*ReturnJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.appIdToReturnJobMap[id]
	return job, ok
}

func (r *JobResolver) DeleteById(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.idToReturnJobMap[id]
	if !ok {
		return false
	}
	delete(r.appIdToReturnJobMap, job.AppId)
	delete(r.idToReturnJobMap, id)
	return true
}
