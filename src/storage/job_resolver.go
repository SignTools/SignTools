package storage

import (
	"github.com/elliotchance/orderedmap"
	"github.com/google/uuid"
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
	r.appIdToSignJobMap.Set(appId, &signJob{
		ts:        time.Now(),
		appId:     appId,
		profileId: profileId,
	})
}

var ErrNotFound = errors.New("not found")

func (r *JobResolver) TakeLastJob(writer io.Writer) error {
	r.mu.Lock()
	if r.appIdToSignJobMap.Len() < 1 {
		r.mu.Unlock()
		return errors.WithMessage(ErrNotFound, "sign job")
	}

	elem := r.appIdToSignJobMap.Back()
	r.appIdToSignJobMap.Delete(elem.Key)
	job := elem.Value.(*signJob)
	returnJobId := uuid.NewString()
	returnJob := ReturnJob{Id: returnJobId, Ts: time.Now(), AppId: job.appId}
	r.idToReturnJobMap[returnJobId] = &returnJob
	r.appIdToReturnJobMap[job.appId] = &returnJob
	r.mu.Unlock()

	if err := job.writeArchive(returnJobId, writer); err != nil {
		r.mu.Lock()
		delete(r.idToReturnJobMap, returnJobId)
		delete(r.appIdToReturnJobMap, job.appId)
		r.mu.Unlock()
		return errors.WithMessage(err, "write archive")
	}
	return nil
}

func (r *JobResolver) Cleanup(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	var deleteList []any
	for el := r.appIdToSignJobMap.Front(); el != nil; el = el.Next() {
		job := el.Value.(*signJob)
		if now.After(job.ts.Add(timeout)) {
			deleteList = append(deleteList, el.Key)
		}
	}
	for _, key := range deleteList {
		r.appIdToSignJobMap.Delete(key)
	}
	var deleteList2 []string
	for id, job := range r.idToReturnJobMap {
		if now.After(job.Ts.Add(timeout)) {
			deleteList2 = append(deleteList2, id)
		}
	}
	for _, id := range deleteList2 {
		r.deleteById(id)
	}
}

func (r *JobResolver) GetStatusByAppId(id string) (bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, jobPending := r.appIdToSignJobMap.Get(id)
	_, jobExists := r.appIdToReturnJobMap[id]
	return jobPending, jobExists
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
	return r.deleteById(id)
}

func (r *JobResolver) deleteById(id string) bool {
	job, ok := r.idToReturnJobMap[id]
	if !ok {
		return false
	}
	delete(r.appIdToReturnJobMap, job.AppId)
	delete(r.idToReturnJobMap, id)
	return true
}
