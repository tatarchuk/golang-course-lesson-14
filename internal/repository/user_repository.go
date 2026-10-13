package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"homework/internal/model"
)

// UserRepository is the storage contract for accounts.
type UserRepository interface {
	// Create stores a new member account; a duplicate e-mail is ErrConflict.
	Create(ctx context.Context, email, passwordHash string) (model.User, error)
	// Get returns the account with the id, or ErrNotFound.
	Get(ctx context.Context, id int64) (model.User, error)
	// GetByEmail returns the account with the (normalized) e-mail, or ErrNotFound.
	GetByEmail(ctx context.Context, email string) (model.User, error)
	// List returns every account ordered by id; no accounts is an empty slice, not nil.
	List(ctx context.Context) ([]model.User, error)
	// UpdateRole changes the role of an existing account: a missing account is
	// ErrNotFound, a role outside the CHECK list is ErrInvalidInput.
	UpdateRole(ctx context.Context, id int64, role string) (model.User, error)
	// UpsertAdmin creates the account with role admin or, when the e-mail exists, sets
	// its password hash and promotes it. Only cmd/bootstrap uses it.
	UpsertAdmin(ctx context.Context, email, passwordHash string) (model.User, error)
}

type gormUserRepository struct {
	db *gorm.DB
}

var _ UserRepository = (*gormUserRepository)(nil)

func (r *gormUserRepository) Create(ctx context.Context, email, passwordHash string) (model.User, error) {
	user := model.User{Email: email, Role: model.UserRoleMember, PasswordHash: passwordHash}
	if err := r.db.WithContext(ctx).Create(&user).Error; err != nil {
		return model.User{}, translate(err) // uq_users_email: ErrConflict
	}
	return user, nil
}

func (r *gormUserRepository) Get(ctx context.Context, id int64) (model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return model.User{}, translate(err)
	}
	return user, nil
}

func (r *gormUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	// Always "email = ?" with an argument: a bare string here would be raw SQL.
	if err := r.db.WithContext(ctx).First(&user, "email = ?", email).Error; err != nil {
		return model.User{}, translate(err)
	}
	return user, nil
}

func (r *gormUserRepository) List(ctx context.Context) ([]model.User, error) {
	users := make([]model.User, 0)
	if err := r.db.WithContext(ctx).Order("id").Find(&users).Error; err != nil {
		return nil, translate(err)
	}
	return users, nil
}

func (r *gormUserRepository) UpdateRole(ctx context.Context, id int64, role string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&user, id).Error; err != nil {
			return err // a missing row becomes ErrNotFound below
		}
		if err := tx.Model(&user).Update("role", role).Error; err != nil {
			return err // chk_users_role: ErrInvalidInput
		}
		// Re-read the row so the caller sees exactly what was stored (role, updated_at).
		return tx.First(&user, id).Error
	})
	if err != nil {
		return model.User{}, translate(err)
	}
	return user, nil
}

func (r *gormUserRepository) UpsertAdmin(ctx context.Context, email, passwordHash string) (model.User, error) {
	user := model.User{Email: email, Role: model.UserRoleAdmin, PasswordHash: passwordHash}
	onConflict := clause.OnConflict{
		Columns: []clause.Column{{Name: "email"}},
		DoUpdates: clause.Assignments(map[string]any{
			"password_hash": passwordHash,
			"role":          model.UserRoleAdmin,
			"updated_at":    gorm.Expr("now()"),
		}),
	}
	if err := r.db.WithContext(ctx).Clauses(onConflict).Create(&user).Error; err != nil {
		return model.User{}, translate(err)
	}
	// On the conflict path the in-memory struct is stale (id, created_at): re-read it.
	return r.GetByEmail(ctx, email)
}
