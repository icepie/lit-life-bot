package model

import (
	"github.com/jinzhu/gorm"
)

// User 用户模型
type User struct {
	gorm.Model
	StuID  string `gorm:"unique"`
	QQ     uint   `gorm:"unique"`
	Gender uint
	Class  string
}
