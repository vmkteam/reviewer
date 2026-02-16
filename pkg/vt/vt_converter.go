package vt

import (
	"time"

	"reviewsrv/pkg/db"
)

// mapp converts slice of type T to slice of type M with given converter with pointers.
func mapp[T, M any](a []T, f func(*T) *M) []M {
	n := make([]M, len(a))
	for i := range a {
		n[i] = *f(&a[i])
	}
	return n
}

func fmtDate(t time.Time) string {
	return t.Format(time.DateOnly)
}

func fmtDatePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

func NewUser(in *db.User) *User {
	if in == nil {
		return nil
	}

	user := &User{
		ID:             in.ID,
		CreatedAt:      fmtDate(in.CreatedAt),
		Login:          in.Login,
		LastActivityAt: fmtDatePtr(in.LastActivityAt),
		StatusID:       in.StatusID,
		Status:         NewStatus(in.StatusID),
	}

	return user
}

func NewUserSummary(in *db.User) *UserSummary {
	if in == nil {
		return nil
	}

	return &UserSummary{
		ID:             in.ID,
		CreatedAt:      fmtDate(in.CreatedAt),
		Login:          in.Login,
		LastActivityAt: fmtDatePtr(in.LastActivityAt),
		Status:         NewStatus(in.StatusID),
	}
}

func NewUserProfile(in *db.User) *UserProfile {
	if in == nil {
		return nil
	}

	return &UserProfile{
		ID:             in.ID,
		CreatedAt:      fmtDate(in.CreatedAt),
		Login:          in.Login,
		LastActivityAt: fmtDatePtr(in.LastActivityAt),
		StatusID:       in.StatusID,
	}
}
