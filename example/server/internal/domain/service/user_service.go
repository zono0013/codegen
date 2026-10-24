package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"example.com/blog/server/internal/domain/entity"
	"example.com/blog/server/internal/domain/repository"
	"example.com/blog/server/pkg/util/errs"
)

// UserService は投稿者のドメインロジックを持つ。
type UserService struct {
	userRepo repository.UserRepository
	clock    func() time.Time
	runTx    TxRunner
}

// NewUserService は clock / runTx に nil を渡すと time.Now / tx.Run を使う。
func NewUserService(userRepo repository.UserRepository, clock func() time.Time, runTx TxRunner) *UserService {
	return &UserService{
		userRepo: userRepo,
		clock:    orDefaultClock(clock),
		runTx:    orDefaultTxRunner(runTx),
	}
}

func (s *UserService) List(ctx context.Context) ([]*entity.User, error) {
	return s.userRepo.SelectAll(ctx)
}

func (s *UserService) ListByCursor(ctx context.Context, limit int, afterID string) ([]*entity.User, error) {
	var after *string
	if afterID != "" {
		after = &afterID
	}
	return s.userRepo.SelectByCursor(ctx, normalizeLimit(limit), after)
}

// GetByID は未存在なら (nil, nil)。404 への変換は handler が行う。
func (s *UserService) GetByID(ctx context.Context, id string) (*entity.User, error) {
	return s.userRepo.SelectByPK(ctx, id)
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	return s.userRepo.SelectByEmail(ctx, email)
}

func (s *UserService) Create(ctx context.Context, email, name string) (*entity.User, error) {
	if err := s.ensureEmailAvailable(ctx, email, ""); err != nil {
		return nil, err
	}

	user, err := entity.NewUser(uuid.NewString(), email, name, s.clock())
	if err != nil {
		return nil, errs.NewValidationError("user", err.Error())
	}
	if err := s.userRepo.Insert(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Update は email / name を差し替える。created_at は保持する。
func (s *UserService) Update(ctx context.Context, id, email, name string) (*entity.User, error) {
	var updated *entity.User
	err := s.runTx(ctx, func(ctx context.Context) error {
		current, err := s.userRepo.SelectByPK(ctx, id)
		if err != nil {
			return err
		}
		if current == nil {
			return errs.NewNotFoundError("user", id)
		}
		if err := s.ensureEmailAvailable(ctx, email, id); err != nil {
			return err
		}

		next, err := entity.NewUser(current.ID, email, name, current.CreatedAt)
		if err != nil {
			return errs.NewValidationError("user", err.Error())
		}
		if err := s.userRepo.Update(ctx, next); err != nil {
			return err
		}
		updated = next
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	return s.userRepo.Delete(ctx, id)
}

// ensureEmailAvailable は email が未使用か確認する。selfID を渡すと自分自身との衝突は無視する。
func (s *UserService) ensureEmailAvailable(ctx context.Context, email, selfID string) error {
	existing, err := s.userRepo.SelectByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != selfID {
		return errs.NewDuplicateError("user", "email")
	}
	return nil
}
