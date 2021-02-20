package storage

import (
	"github.com/google/uuid"
)

func newOneTimeResolver() *OneTimeResolver {
	return &OneTimeResolver{publicToPrivateId: map[string]string{}}
}

type OneTimeResolver struct {
	publicToPrivateId map[string]string
}

func (r *OneTimeResolver) MakeId(privateId string) string {
	publicId := uuid.NewString()
	r.publicToPrivateId[publicId] = privateId
	return publicId
}

func (r *OneTimeResolver) ResolveId(publicId string) (string, bool) {
	privateId, ok := r.publicToPrivateId[publicId]
	if ok {
		delete(r.publicToPrivateId, publicId)
	}
	return privateId, ok
}
