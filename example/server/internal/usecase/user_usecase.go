package usecase

import (
	"context"

	"example.com/blog/server/internal/domain/service"
	"example.com/blog/server/internal/dto"
)

// UserUsecaseImpl は生成された UserUsecase interface の実装。
type UserUsecaseImpl struct {
	userService *service.UserService
}

var _ UserUsecase = (*UserUsecaseImpl)(nil)

func NewUserUsecase(userService *service.UserService) *UserUsecaseImpl {
	return &UserUsecaseImpl{userService: userService}
}

func (u *UserUsecaseImpl) ListUsers(ctx context.Context, _ ListUsersInput) ([]*dto.UserDTO, error) {
	users, err := u.userService.List(ctx)
	if err != nil {
		return nil, err
	}
	return dto.FromUsers(users), nil
}

func (u *UserUsecaseImpl) ListUsersByCursor(ctx context.Context, input ListUsersByCursorInput) ([]*dto.UserDTO, error) {
	users, err := u.userService.ListByCursor(ctx, int(input.Limit), input.AfterID)
	if err != nil {
		return nil, err
	}
	return dto.FromUsers(users), nil
}

func (u *UserUsecaseImpl) GetUser(ctx context.Context, input GetUserInput) (*dto.UserDTO, error) {
	user, err := u.userService.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	return dto.FromUser(user), nil
}

func (u *UserUsecaseImpl) GetUserByEmail(ctx context.Context, input GetUserByEmailInput) (*dto.UserDTO, error) {
	user, err := u.userService.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	return dto.FromUser(user), nil
}

func (u *UserUsecaseImpl) CreateUser(ctx context.Context, input CreateUserInput) (*dto.UserDTO, error) {
	user, err := u.userService.Create(ctx, input.Email, input.Name)
	if err != nil {
		return nil, err
	}
	return dto.FromUser(user), nil
}

func (u *UserUsecaseImpl) UpdateUser(ctx context.Context, input UpdateUserInput) (*dto.UserDTO, error) {
	user, err := u.userService.Update(ctx, input.ID, input.Email, input.Name)
	if err != nil {
		return nil, err
	}
	return dto.FromUser(user), nil
}

func (u *UserUsecaseImpl) DeleteUser(ctx context.Context, input DeleteUserInput) error {
	return u.userService.Delete(ctx, input.ID)
}
