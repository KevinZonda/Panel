package dao

import (
	"errors"

	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/gorm"
)

func CountUsers(db *gorm.DB) (int64, error) {
	var count int64
	err := db.Model(&dbmod.User{}).Count(&count).Error
	return count, err
}

func CountActiveAdmins(db *gorm.DB) (int64, error) {
	var count int64
	err := db.Model(&dbmod.User{}).
		Where("role = ? AND status = ?", "admin", "active").
		Count(&count).Error
	return count, err
}

func FindUserByUsername(db *gorm.DB, username string) (*dbmod.User, error) {
	var user dbmod.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func FindUserByID(db *gorm.DB, id uint) (*dbmod.User, error) {
	var user dbmod.User
	if err := db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func CreateUser(db *gorm.DB, user *dbmod.User) error {
	return db.Create(user).Error
}

func UpdateUser(db *gorm.DB, user *dbmod.User) error {
	return db.Save(user).Error
}

func DeleteUser(db *gorm.DB, id uint) error {
	result := db.Delete(&dbmod.User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ListUsers(db *gorm.DB, page, pageSize int, keyword string) ([]dbmod.User, int64, error) {
	query := db.Model(&dbmod.User{})
	if keyword != "" {
		query = query.Where("username LIKE ?", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []dbmod.User
	err := query.Order("id ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, err
	}
	return users, total, nil
}
