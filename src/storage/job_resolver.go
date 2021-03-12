package storage

import (
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

func newJobResolver() *JobResolver {
	return &JobResolver{
		idToReturnJobMap:    map[string]*ReturnJob{},
		appIdToReturnJobMap: map[string]*ReturnJob{},
	}
}

type JobResolver struct {
	mu                  sync.Mutex
	signJobs            []signJob
	idToReturnJobMap    map[string]*ReturnJob
	appIdToReturnJobMap map[string]*ReturnJob
}

// User bundle ID is unused if the profile is not an account.
func (r *JobResolver) MakeSignJob(appId string, userBundleId string, profileId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.signJobs = append(r.signJobs, signJob{
		ts:           time.Now(),
		appId:        appId,
		userBundleId: userBundleId,
		profileId:    profileId,
	})
}

var ErrNotFound = errors.New("not found")

func (r *JobResolver) TakeLastJob(writer io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.signJobs) < 1 {
		return errors.WithMessage(ErrNotFound, "sign job")
	}

	l := len(r.signJobs)
	job := r.signJobs[l-1]
	r.signJobs = r.signJobs[:l-1]

	returnJobId, err := job.writeArchive(writer)
	if err != nil {
		return errors.WithMessage(err, "write archive")
	}
	returnJob := ReturnJob{Id: returnJobId, Ts: time.Now(), AppId: job.appId}
	r.idToReturnJobMap[returnJobId] = &returnJob
	r.appIdToReturnJobMap[job.appId] = &returnJob
	return nil
}

func (r *JobResolver) GetById(id string) (*ReturnJob, bool) {
	job, ok := r.idToReturnJobMap[id]
	return job, ok
}

func (r *JobResolver) GetByAppId(id string) (*ReturnJob, bool) {
	job, ok := r.appIdToReturnJobMap[id]
	return job, ok
}

func (r *JobResolver) DeleteById(id string) bool {
	job, ok := r.idToReturnJobMap[id]
	if !ok {
		return false
	}
	delete(r.appIdToReturnJobMap, job.AppId)
	delete(r.idToReturnJobMap, id)
	return true
}
