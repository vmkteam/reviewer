//nolint:dupl,funlen
package test

import (
	"context"
	"testing"
	"time"

	"reviewsrv/pkg/db"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-pg/pg/v10/orm"
)

type UserOpFunc func(t *testing.T, dbo orm.DB, in *db.User) Cleaner

func User(t *testing.T, dbo orm.DB, in *db.User, ops ...UserOpFunc) (*db.User, Cleaner) {
	repo := db.NewCommonRepo(dbo)
	var cleaners []Cleaner

	// Fill the incoming entity
	if in == nil {
		in = &db.User{}
	}

	// Check if PKs are provided
	if in.ID != 0 {
		// Fetch the entity by PK
		user, err := repo.UserByID(t.Context(), in.ID, repo.FullUser())
		if err != nil {
			t.Fatal(err)
		}

		// We must find the entity by PK
		if user == nil {
			t.Fatalf("the entity User is not found by provided PKs ID=%v", in.ID)
		}

		// Return if found without real cleanup
		return user, emptyClean
	}

	for _, op := range ops {
		if cl := op(t, dbo, in); cl != nil {
			cleaners = append(cleaners, cl)
		}
	}

	// Create the main entity
	user, err := repo.AddUser(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}

	return user, func() {
		if _, err := dbo.ModelContext(context.Background(), &db.User{ID: user.ID}).WherePK().Delete(); err != nil {
			t.Fatal(err)
		}
		// Clean up related entities from the last to the first
		for i := len(cleaners) - 1; i >= 0; i-- {
			cleaners[i]()
		}
	}
}

func WithFakeUser(t *testing.T, dbo orm.DB, in *db.User) Cleaner {
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}

	if in.Login == "" {
		in.Login = cutS(gofakeit.Word(), 64)
	}

	if in.Password == "" {
		in.Password = cutS(gofakeit.Password(true, true, true, false, false, 12), 64)
	}

	if in.AuthKey == "" {
		in.AuthKey = cutS(gofakeit.Sentence(3), 32)
	}

	if in.StatusID == 0 {
		in.StatusID = 1
	}

	return emptyClean
}
