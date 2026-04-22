package infraaccount

import "time"

type UserModel struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Account   string    `gorm:"column:account;size:64;not null;uniqueIndex"`
	Password  string    `gorm:"column:password;size:255;not null"`
	Nickname  string    `gorm:"column:nickname;size:128;not null"`
	Role      string    `gorm:"column:role;size:32;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserModel) TableName() string {
	return "accounts"
}
