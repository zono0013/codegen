package service_test

import (
	"context"
	"testing"
	"time"

	"example.com/blog/server/internal/domain/entity"
	"example.com/blog/server/internal/domain/repository/mock"
	"example.com/blog/server/internal/domain/service"
	"example.com/blog/server/pkg/util/errs"
)

func newUserService(userRepo *mock.MockUserRepository, now time.Time) *service.UserService {
	return service.NewUserService(userRepo, func() time.Time { return now }, passThrough)
}

func TestUserServiceCreate(t *testing.T) {
	var inserted *entity.User
	userRepo := &mock.MockUserRepository{
		InsertFunc: func(_ context.Context, u *entity.User) error {
			inserted = u
			return nil
		},
	}

	got, err := newUserService(userRepo, fixedNow).Create(testCtx, "writer@example.com", "writer")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.Email != "writer@example.com" || got.Name != "writer" {
		t.Errorf("Create() = %+v", got)
	}
	if !got.CreatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, fixedNow)
	}
	if inserted == nil || inserted.ID == "" {
		t.Error("Insert に ID 付きの entity が渡されていない")
	}
}

func TestUserServiceCreateRejectsDuplicateEmail(t *testing.T) {
	userRepo := &mock.MockUserRepository{
		SelectByEmailFunc: func(_ context.Context, email string) (*entity.User, error) {
			return &entity.User{ID: "u-other", Email: email}, nil
		},
		InsertFunc: func(context.Context, *entity.User) error {
			t.Error("email 重複時に Insert してはいけない")
			return nil
		},
	}

	_, err := newUserService(userRepo, fixedNow).Create(testCtx, "taken@example.com", "writer")

	assertErrorType(t, err, errs.ErrorTypeDuplicate)
}

func TestUserServiceCreateRejectsInvalidEmail(t *testing.T) {
	_, err := newUserService(&mock.MockUserRepository{}, fixedNow).Create(testCtx, "not-an-email", "writer")

	assertErrorType(t, err, errs.ErrorTypeValidation)
}

func TestUserServiceUpdateAllowsKeepingOwnEmail(t *testing.T) {
	current := &entity.User{ID: "u1", Email: "me@example.com", Name: "old", CreatedAt: fixedNow}
	userRepo := &mock.MockUserRepository{
		SelectByPKFunc: func(context.Context, string) (*entity.User, error) { return current, nil },
		SelectByEmailFunc: func(context.Context, string) (*entity.User, error) {
			return current, nil // 同じ email は自分自身なので衝突扱いしない
		},
		UpdateFunc: func(context.Context, *entity.User) error { return nil },
	}

	got, err := newUserService(userRepo, laterNow).Update(testCtx, "u1", "me@example.com", "new name")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got.Name != "new name" {
		t.Errorf("Name = %q, want %q", got.Name, "new name")
	}
	if !got.CreatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt = %v, want %v（更新では変えない）", got.CreatedAt, fixedNow)
	}
}

func TestUserServiceUpdateRejectsOthersEmail(t *testing.T) {
	current := &entity.User{ID: "u1", Email: "me@example.com", Name: "me", CreatedAt: fixedNow}
	userRepo := &mock.MockUserRepository{
		SelectByPKFunc: func(context.Context, string) (*entity.User, error) { return current, nil },
		SelectByEmailFunc: func(context.Context, string) (*entity.User, error) {
			return &entity.User{ID: "u2", Email: "taken@example.com"}, nil
		},
	}

	_, err := newUserService(userRepo, laterNow).Update(testCtx, "u1", "taken@example.com", "me")

	assertErrorType(t, err, errs.ErrorTypeDuplicate)
}

func TestUserServiceUpdateNotFound(t *testing.T) {
	userRepo := &mock.MockUserRepository{
		SelectByPKFunc: func(context.Context, string) (*entity.User, error) { return nil, nil },
	}

	_, err := newUserService(userRepo, laterNow).Update(testCtx, "missing", "a@example.com", "name")

	assertErrorType(t, err, errs.ErrorTypeNotFound)
}
