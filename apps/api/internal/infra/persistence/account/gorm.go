package infraaccount

import (
	domainaccount "GCFeed/internal/domain/account"
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
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
		Account:   user.Account,
		Password:  user.Password,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
		Bio:       user.Bio,
		Status:    user.Status,
		Role:      user.Role,
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicateKeyError(err) {
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

	return restoreUser(user), nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*domainaccount.User, error) {
	var user UserModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainaccount.ErrUserNotFound
		}
		return nil, err
	}

	return restoreUser(user), nil
}

func (r *Repository) UpdateProfile(ctx context.Context, user *domainaccount.User) error {
	result := r.db.WithContext(ctx).
		Model(&UserModel{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"nickname":   user.Nickname,
			"avatar_url": user.AvatarURL,
			"bio":        user.Bio,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainaccount.ErrUserNotFound
	}
	return nil
}

func restoreUser(user UserModel) *domainaccount.User {
	return domainaccount.RestoreUser(
		user.ID,
		user.Account,
		user.Password,
		user.Nickname,
		user.AvatarURL,
		user.Bio,
		user.Status,
		user.Role,
	)
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
