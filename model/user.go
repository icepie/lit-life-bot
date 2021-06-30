package model

import (
	"github.com/jinzhu/gorm"
)

var (
	AdminIdentity = "admin"
	UserIdentity  = "user"
)

// User 用户模型
type User struct {
	gorm.Model
	StuID          string `gorm:"unique"`
	QQ             uint   `gorm:"unique"`
	Identity       string
	SecPassword    string
	JWPassword     string
	HealthPassword string
	Gender         uint
	Class          string
}
