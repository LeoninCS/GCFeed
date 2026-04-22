package domainaccount

import "context"

type Repository interface {
	Save(ctx context.Context, user *User) error
	FindByAccount(ctx context.Context, account string) (*User, error)
}
