package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"example.com/blog/server/internal/domain/entity"
	"example.com/blog/server/internal/domain/repository"
	"example.com/blog/server/pkg/util/errs"
)

// ArticleService は記事のドメインロジックを持つ。
type ArticleService struct {
	articleRepo repository.ArticleRepository
	userRepo    repository.UserRepository
	clock       func() time.Time
	runTx       TxRunner
}

// NewArticleService は clock / runTx に nil を渡すと time.Now / tx.Run を使う。
func NewArticleService(
	articleRepo repository.ArticleRepository,
	userRepo repository.UserRepository,
	clock func() time.Time,
	runTx TxRunner,
) *ArticleService {
	return &ArticleService{
		articleRepo: articleRepo,
		userRepo:    userRepo,
		clock:       orDefaultClock(clock),
		runTx:       orDefaultTxRunner(runTx),
	}
}

func (s *ArticleService) List(ctx context.Context) ([]*entity.Article, error) {
	return s.articleRepo.SelectAll(ctx)
}

// ListByCursor は id 昇順の cursor pagination。afterID が空なら先頭ページ。
func (s *ArticleService) ListByCursor(ctx context.Context, limit int, afterID string) ([]*entity.Article, error) {
	var after *string
	if afterID != "" {
		after = &afterID
	}
	return s.articleRepo.SelectByCursor(ctx, normalizeLimit(limit), after)
}

// GetByID は未存在なら (nil, nil)。404 への変換は handler が行う。
func (s *ArticleService) GetByID(ctx context.Context, id string) (*entity.Article, error) {
	return s.articleRepo.SelectByPK(ctx, id)
}

func (s *ArticleService) GetBySlug(ctx context.Context, slug string) (*entity.Article, error) {
	return s.articleRepo.SelectBySlug(ctx, slug)
}

// Create は下書き（published=false）として記事を作る。
// 投稿者の存在と slug の一意性をドメインルールとして検証する。
func (s *ArticleService) Create(ctx context.Context, slug, title, body, authorID string) (*entity.Article, error) {
	if err := s.ensureAuthorExists(ctx, authorID); err != nil {
		return nil, err
	}
	if err := s.ensureSlugAvailable(ctx, slug, ""); err != nil {
		return nil, err
	}

	now := s.clock()
	article, err := entity.NewArticle(uuid.NewString(), slug, title, body, authorID, false, now, now)
	if err != nil {
		return nil, errs.NewValidationError("article", err.Error())
	}
	if err := s.articleRepo.Insert(ctx, article); err != nil {
		return nil, err
	}
	return article, nil
}

// Update は slug / title / body を差し替える。published / author_id / created_at は保持する。
func (s *ArticleService) Update(ctx context.Context, id, slug, title, body string) (*entity.Article, error) {
	var updated *entity.Article
	err := s.runTx(ctx, func(ctx context.Context) error {
		current, err := s.articleRepo.SelectByPK(ctx, id)
		if err != nil {
			return err
		}
		if current == nil {
			return errs.NewNotFoundError("article", id)
		}
		if err := s.ensureSlugAvailable(ctx, slug, id); err != nil {
			return err
		}

		next, err := entity.NewArticle(
			current.ID, slug, title, body,
			current.AuthorID, current.Published,
			current.CreatedAt, s.clock(),
		)
		if err != nil {
			return errs.NewValidationError("article", err.Error())
		}
		if err := s.articleRepo.Update(ctx, next); err != nil {
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

// SetPublished は公開状態だけを切り替える。冪等（同じ状態なら書き込まない）。
func (s *ArticleService) SetPublished(ctx context.Context, id string, published bool) (*entity.Article, error) {
	var updated *entity.Article
	err := s.runTx(ctx, func(ctx context.Context) error {
		current, err := s.articleRepo.SelectByPK(ctx, id)
		if err != nil {
			return err
		}
		if current == nil {
			return errs.NewNotFoundError("article", id)
		}
		if current.Published == published {
			updated = current
			return nil
		}

		next, err := entity.NewArticle(
			current.ID, current.Slug, current.Title, current.Body,
			current.AuthorID, published,
			current.CreatedAt, s.clock(),
		)
		if err != nil {
			return errs.NewValidationError("article", err.Error())
		}
		if err := s.articleRepo.Update(ctx, next); err != nil {
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

func (s *ArticleService) Delete(ctx context.Context, id string) error {
	return s.articleRepo.Delete(ctx, id)
}

// ensureAuthorExists は author_id が実在する user かを確認する。
// FK 制約は張っていないので（proto に FK マーカーがない）、ここで担保する。
func (s *ArticleService) ensureAuthorExists(ctx context.Context, authorID string) error {
	author, err := s.userRepo.SelectByPK(ctx, authorID)
	if err != nil {
		return err
	}
	if author == nil {
		return errs.NewValidationError("author_id", "author not found")
	}
	return nil
}

// ensureSlugAvailable は slug が未使用か確認する。selfID を渡すと自分自身との衝突は無視する。
func (s *ArticleService) ensureSlugAvailable(ctx context.Context, slug, selfID string) error {
	existing, err := s.articleRepo.SelectBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != selfID {
		return errs.NewDuplicateError("article", "slug")
	}
	return nil
}
