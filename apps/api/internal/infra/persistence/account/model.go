package infraaccount

import "time"

// UserModel 映射 account 表，保存用户登录凭证和展示资料。
type UserModel struct {
	ID      int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Account string `gorm:"column:account;size:64;not null;uniqueIndex"`
	// Password 保存 bcrypt 哈希值，登录时用于校验用户输入密码。
	Password  string    `gorm:"column:password;size:255;not null"`
	Nickname  string    `gorm:"column:nickname;size:128;not null"`
	AvatarURL string    `gorm:"column:avatar_url;size:512"`
	Bio       string    `gorm:"column:bio;size:255"`
	Status    int       `gorm:"column:status;not null;default:1"`
	Role      string    `gorm:"column:role;size:32;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定数据库表名，避免 GORM 使用默认复数表名。
func (UserModel) TableName() string {
	return "account"
}
