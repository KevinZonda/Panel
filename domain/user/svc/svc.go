package svc

import (
	"errors"
	"strings"

	authdao "github.com/KevinZonda/Panel/domain/auth/dao"
	userdao "github.com/KevinZonda/Panel/domain/user/dao"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/lib/validatex"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListUsers(page, pageSize int, keyword string) (*apimod.UserList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	keyword = strings.TrimSpace(keyword)

	users, total, err := userdao.ListUsers(s.db, page, pageSize, keyword)
	if err != nil {
		return nil, err
	}
	items := make([]apimod.User, 0, len(users))
	for i := range users {
		items = append(items, apimod.UserResponse(&users[i]))
	}
	return &apimod.UserList{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *Service) CreateUser(req apimod.CreateUserRequest) (*dbmod.User, error) {
	req.Username = strings.TrimSpace(req.Username)
	if !validatex.ValidUsername(req.Username) {
		return nil, apperr.Validationf("username is invalid")
	}
	if !validatex.ValidRole(req.Role) {
		return nil, apperr.Validationf("role is invalid")
	}
	if !validatex.ValidUserStatus(req.Status) {
		return nil, apperr.Validationf("status is invalid")
	}

	_, err := userdao.FindUserByUsername(s.db, req.Username)
	if err == nil {
		return nil, apperr.Conflictf("username already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash, err := passwordx.Hash(req.Password)
	if err != nil {
		return nil, err
	}
	user := &dbmod.User{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         req.Role,
		Status:       req.Status,
	}
	if err := userdao.CreateUser(s.db, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) GetUser(id uint) (*dbmod.User, error) {
	user, err := userdao.FindUserByID(s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFoundf("user not found")
		}
		return nil, err
	}
	return user, nil
}

func (s *Service) UpdateUser(actorID, targetID uint, req apimod.UpdateUserRequest) (*dbmod.User, error) {
	var updated *dbmod.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		target, err := userdao.FindUserByID(tx, targetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFoundf("user not found")
			}
			return err
		}

		wasActiveAdmin := target.Role == models.RoleAdmin && target.Status == models.UserStatusActive

		if req.Username != nil {
			username := strings.TrimSpace(*req.Username)
			if !validatex.ValidUsername(username) {
				return apperr.Validationf("username is invalid")
			}
			if username != target.Username {
				_, err := userdao.FindUserByUsername(tx, username)
				if err == nil {
					return apperr.Conflictf("username already exists")
				}
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				target.Username = username
			}
		}
		if req.Role != nil {
			if !validatex.ValidRole(*req.Role) {
				return apperr.Validationf("role is invalid")
			}
			target.Role = *req.Role
		}
		if req.Status != nil {
			if !validatex.ValidUserStatus(*req.Status) {
				return apperr.Validationf("status is invalid")
			}
			target.Status = *req.Status
		}

		isActiveAdmin := target.Role == models.RoleAdmin && target.Status == models.UserStatusActive
		if actorID == targetID && wasActiveAdmin && !isActiveAdmin {
			return apperr.Conflictf("cannot disable or demote yourself")
		}
		if wasActiveAdmin && !isActiveAdmin {
			count, err := userdao.CountActiveAdmins(tx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return apperr.Conflictf("cannot remove the last active admin")
			}
		}

		if err := userdao.UpdateUser(tx, target); err != nil {
			return err
		}
		if target.Status == models.UserStatusDisabled {
			if err := authdao.RevokeUserSessions(tx, target.ID, 0, target.UpdatedAt); err != nil {
				return err
			}
		}
		updated = target
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) ResetPassword(targetID uint, newPassword string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		target, err := userdao.FindUserByID(tx, targetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFoundf("user not found")
			}
			return err
		}
		hash, err := passwordx.Hash(newPassword)
		if err != nil {
			return err
		}
		target.PasswordHash = hash
		if err := userdao.UpdateUser(tx, target); err != nil {
			return err
		}
		return authdao.RevokeUserSessions(tx, targetID, 0, target.UpdatedAt)
	})
}

func (s *Service) DeleteUser(actorID, targetID uint) error {
	if actorID == targetID {
		return apperr.Conflictf("cannot delete yourself")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		target, err := userdao.FindUserByID(tx, targetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFoundf("user not found")
			}
			return err
		}
		if target.Role == models.RoleAdmin && target.Status == models.UserStatusActive {
			count, err := userdao.CountActiveAdmins(tx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return apperr.Conflictf("cannot delete the last active admin")
			}
		}
		if err := authdao.RevokeUserSessions(tx, targetID, 0, target.UpdatedAt); err != nil {
			return err
		}
		return userdao.DeleteUser(tx, targetID)
	})
}
