package repository

import (
	"context"

	"ego/services/users/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTx(tx *gorm.DB) *Repository {
	return &Repository{db: tx}
}

func (r *Repository) UpsertUser(ctx context.Context, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoNothing: true,
	}).Create(user).Error; err != nil {
		return nil, err
	}

	var dbUser *model.User
	if err := r.db.WithContext(ctx).Where("email = ?", user.Email).First(&dbUser).Error; err != nil {
		return nil, err
	}

	return dbUser, nil
}

func (r *Repository) GetByID(ctx context.Context, userID string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) UpdateMe(ctx context.Context, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Clauses(clause.Returning{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"name":   user.Name,
			"avatar": user.Avatar,
		}).Scan(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *Repository) GetList(ctx context.Context) ([]*model.User, error) {
	var users []model.User
	if err := r.db.WithContext(ctx).Find(&users).Error; err != nil {
		return nil, err
	}

	userPtrs := make([]*model.User, len(users))
	for i := range users {
		userPtrs[i] = &users[i]
	}

	return userPtrs, nil
}
