package infraaccount

import (
	"context"
	"errors"
	domainaccount "feedsystem_video_hard/internal/domain/account"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(ctx context.Context, user *domainaccount.User) error {
	model := UserModel{
		Account:  user.Account,
		Password: user.Password,
		Nickname: user.Nickname,
		Role:     user.Role,
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domainaccount.ErrAccountAlreadyExists
		}
		return err
	}
	user.ID = model.ID
	return nil
}

func (r *Repository) FindByAccount(ctx context.Context, account string) (*domainaccount.User, error) {
	var user UserModel
	err := r.db.WithContext(ctx).Where("account = ?", account).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainaccount.ErrUserNotFound
		}
		return nil, err
	}

	return domainaccount.RestoreUser(
		user.ID,
		user.Account,
		user.Password,
		user.Nickname,
		user.Role,
	), nil
}
