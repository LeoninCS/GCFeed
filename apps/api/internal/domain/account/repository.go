package domainaccount

import "context"

// Repository 定义账号领域需要的持久化能力，应用层只依赖这个接口。
type Repository interface {
	// Save 保存新用户，账号重复时返回 ErrAccountAlreadyExists。
	Save(ctx context.Context, user *User) error
	// FindByAccount 用于登录流程通过账号查找用户。
	FindByAccount(ctx context.Context, account string) (*User, error)
	// FindByID 用于根据登录态读取当前用户。
	FindByID(ctx context.Context, id int64) (*User, error)
	// UpdateProfile 只更新用户展示资料字段。
	UpdateProfile(ctx context.Context, user *User) error
}
