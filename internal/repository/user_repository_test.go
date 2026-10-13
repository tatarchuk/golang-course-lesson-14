package repository_test

import (
	"context"
	"errors"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
	"homework/internal/testdb"
)

func TestUserRepositoryCreateGetAndDuplicate(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()

	u, err := repos.Users.Create(ctx, "reader@albums.local", "$2a$10$hash")
	if err != nil || u.ID == 0 || u.Role != model.UserRoleMember || u.Email != "reader@albums.local" {
		t.Fatalf("create: %+v %v", u, err)
	}
	if u.CreatedAt.IsZero() || u.UpdatedAt.IsZero() {
		t.Errorf("timestamps must be set: %+v", u)
	}
	got, err := repos.Users.GetByEmail(ctx, "reader@albums.local")
	if err != nil || got.ID != u.ID || got.PasswordHash != "$2a$10$hash" {
		t.Errorf("get by email: %+v %v", got, err)
	}
	if got, err := repos.Users.Get(ctx, u.ID); err != nil || got.Email != "reader@albums.local" {
		t.Errorf("get: %+v %v", got, err)
	}
	if _, err := repos.Users.Create(ctx, "reader@albums.local", "x"); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("duplicate e-mail: expected ErrConflict, got %v", err)
	}
	if _, err := repos.Users.GetByEmail(ctx, "nobody@albums.local"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown e-mail: expected ErrNotFound, got %v", err)
	}
	if _, err := repos.Users.Get(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown id: expected ErrNotFound, got %v", err)
	}
}

func TestUserRepositoryListAndUpdateRole(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()

	empty, err := repos.Users.List(ctx)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("an empty list must be an empty non-nil slice: %#v %v", empty, err)
	}
	first, _ := repos.Users.Create(ctx, "first@albums.local", "h1")
	second, _ := repos.Users.Create(ctx, "second@albums.local", "h2")
	users, err := repos.Users.List(ctx)
	if err != nil || len(users) != 2 || users[0].ID != first.ID || users[1].ID != second.ID {
		t.Fatalf("list ordered by id: %+v %v", users, err)
	}

	updated, err := repos.Users.UpdateRole(ctx, first.ID, model.UserRoleStaff)
	if err != nil || updated.Role != model.UserRoleStaff || updated.ID != first.ID || updated.Email != first.Email {
		t.Fatalf("update role: %+v %v", updated, err)
	}
	if updated.UpdatedAt.Before(first.UpdatedAt) {
		t.Errorf("updated_at must not go backwards: %s < %s", updated.UpdatedAt, first.UpdatedAt)
	}
	if again, _ := repos.Users.Get(ctx, first.ID); again.Role != model.UserRoleStaff {
		t.Errorf("the new role must be stored: %+v", again)
	}
	if _, err := repos.Users.UpdateRole(ctx, 999999, model.UserRoleStaff); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown id: expected ErrNotFound, got %v", err)
	}
	if _, err := repos.Users.UpdateRole(ctx, second.ID, "boss"); !errors.Is(err, repository.ErrInvalidInput) {
		t.Errorf("role outside the CHECK list: expected ErrInvalidInput, got %v", err)
	}
	if again, _ := repos.Users.Get(ctx, second.ID); again.Role != model.UserRoleMember {
		t.Errorf("a rejected update must change nothing: %+v", again)
	}
}

func TestUserRepositoryUpsertAdmin(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()

	created, err := repos.Users.UpsertAdmin(ctx, "boss@albums.local", "h-admin")
	if err != nil || created.ID == 0 || created.Role != model.UserRoleAdmin || created.PasswordHash != "h-admin" {
		t.Fatalf("fresh admin: %+v %v", created, err)
	}
	member, _ := repos.Users.Create(ctx, "promoted@albums.local", "h-old")
	promoted, err := repos.Users.UpsertAdmin(ctx, "promoted@albums.local", "h-new")
	if err != nil || promoted.ID != member.ID || promoted.Role != model.UserRoleAdmin || promoted.PasswordHash != "h-new" {
		t.Fatalf("existing member: %+v %v", promoted, err)
	}
	if users, _ := repos.Users.List(ctx); len(users) != 2 {
		t.Errorf("an upsert must not add a row for an existing e-mail: %d users", len(users))
	}
}

func TestMigrationAddsPasswordHashColumn(t *testing.T) {
	var col struct {
		IsNullable    string
		ColumnDefault string
	}
	err := testdb.DB(t).Raw("SELECT is_nullable, column_default FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'password_hash'").Scan(&col).Error
	if err != nil || col.IsNullable != "NO" || col.ColumnDefault != "''::text" {
		t.Fatalf("password_hash column: %+v %v", col, err)
	}
}
