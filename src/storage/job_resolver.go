package storage

import (
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

func newJobResolver() *JobResolver {
	return &JobResolver{idToReturnJob: map[string]returnJob{}}
}

type JobResolver struct {
	mu            sync.Mutex
	signJobs      []signJob
	idToReturnJob map[string]returnJob
}

func (r *JobResolver) MakeSignJob(appId string, profileId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.signJobs = append(r.signJobs, signJob{
		ts:        time.Now(),
		appId:     appId,
		profileId: profileId,
	})
}

var ErrNotFound = errors.New("not found")

func (r *JobResolver) WriteLastJob(writer io.Writer) error {
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
	r.idToReturnJob[returnJobId] = returnJob{time.Now(), job.appId}
	return nil
}

func (r *JobResolver) ResolveReturnJob(id string) (string, bool) {
	job, ok := r.idToReturnJob[id]
	if ok {
		delete(r.idToReturnJob, id)
	}
	return job.appId, ok
}
