package storage

import (
	"SignTools/src/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type uploadResolver struct {
	idToUploadMap map[string]Upload
	mu            sync.Mutex
}

func newUploadResolver() *uploadResolver {
	return &uploadResolver{idToUploadMap: map[string]Upload{}}
}

func (r *uploadResolver) refresh() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	files, err := os.ReadDir(uploadsPath)
	if err != nil {
		return errors.WithMessage(err, "read uploads dir")
	}
	files = util.RemoveHiddenDirs(files)
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".info" {
			continue
		}
		id := file.Name()
		r.idToUploadMap[id] = loadUpload(id)
	}
	return nil
}

func loadUpload(id string) Upload {
	return newUpload(id)
}

func (r *uploadResolver) Add(id string) Upload {
	r.mu.Lock()
	defer r.mu.Unlock()
	upload := newUpload(id)
	r.idToUploadMap[id] = upload
	return upload
}

func (r *uploadResolver) Get(id string) (Upload, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	upload, ok := r.idToUploadMap[id]
	if !ok {
		return nil, false
	}
	return upload, true
}

func (r *uploadResolver) Cleanup(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	var deleteList []Upload
	for id, upload := range r.idToUploadMap {
		modTime, err := upload.GetModTime()
		if err != nil {
			log.Err(err).Str("id", id).Msg("upload cleanup")
		}
		if now.After(modTime.Add(timeout)) {
			deleteList = append(deleteList, upload)
		}
	}
	for _, upload := range deleteList {
		id := upload.GetId()
		if err := upload.delete(); err != nil {
			log.Err(err).Str("id", id).Msg("upload cleanup")
		}
		delete(r.idToUploadMap, id)
	}
}

func (r *uploadResolver) Delete(id string) error {
	r.mu.Lock()
	upload, ok := r.idToUploadMap[id]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	uploadId := upload.GetId()
	delete(r.idToUploadMap, uploadId)
	r.mu.Unlock()
	if err := upload.delete(); err != nil {
		return errors.WithMessagef(err, "delete upload id=%s", uploadId)
	}
	return nil
}
