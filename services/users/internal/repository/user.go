package repository

import (
	"context"

	"ego/services/users/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) WithTx(tx *gorm.DB) *UserRepository {
	return &UserRepository{db: tx}
}

func (r *UserRepository) UpsertUser(ctx context.Context, user *model.User) (*model.User, error) {
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

func (r *UserRepository) GetByID(ctx context.Context, userID string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateMe(ctx context.Context, user *model.User) (*model.User, error) {
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"name":   user.Name,
			"avatar": user.Avatar,
		}).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetList(ctx context.Context) ([]*model.User, error) {
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
