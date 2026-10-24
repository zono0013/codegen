package usecase

import (
	"context"

	"example.com/blog/server/internal/domain/service"
	"example.com/blog/server/internal/dto"
)

// ArticleUsecaseImpl は生成された ArticleUsecase interface の実装。
// Service を呼び、結果を DTO に変換して返すだけの薄い層に保つ。
type ArticleUsecaseImpl struct {
	articleService *service.ArticleService
}

var _ ArticleUsecase = (*ArticleUsecaseImpl)(nil)

func NewArticleUsecase(articleService *service.ArticleService) *ArticleUsecaseImpl {
	return &ArticleUsecaseImpl{articleService: articleService}
}

func (u *ArticleUsecaseImpl) ListArticles(ctx context.Context, _ ListArticlesInput) ([]*dto.ArticleDTO, error) {
	articles, err := u.articleService.List(ctx)
	if err != nil {
		return nil, err
	}
	return dto.FromArticles(articles), nil
}

func (u *ArticleUsecaseImpl) ListArticlesByCursor(ctx context.Context, input ListArticlesByCursorInput) ([]*dto.ArticleDTO, error) {
	articles, err := u.articleService.ListByCursor(ctx, int(input.Limit), input.AfterID)
	if err != nil {
		return nil, err
	}
	return dto.FromArticles(articles), nil
}

func (u *ArticleUsecaseImpl) GetArticle(ctx context.Context, input GetArticleInput) (*dto.ArticleDTO, error) {
	article, err := u.articleService.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	return dto.FromArticle(article), nil
}

func (u *ArticleUsecaseImpl) GetArticleBySlug(ctx context.Context, input GetArticleBySlugInput) (*dto.ArticleDTO, error) {
	article, err := u.articleService.GetBySlug(ctx, input.Slug)
	if err != nil {
		return nil, err
	}
	return dto.FromArticle(article), nil
}

func (u *ArticleUsecaseImpl) CreateArticle(ctx context.Context, input CreateArticleInput) (*dto.ArticleDTO, error) {
	article, err := u.articleService.Create(ctx, input.Slug, input.Title, input.Body, input.AuthorID)
	if err != nil {
		return nil, err
	}
	return dto.FromArticle(article), nil
}

func (u *ArticleUsecaseImpl) UpdateArticle(ctx context.Context, input UpdateArticleInput) (*dto.ArticleDTO, error) {
	article, err := u.articleService.Update(ctx, input.ID, input.Slug, input.Title, input.Body)
	if err != nil {
		return nil, err
	}
	return dto.FromArticle(article), nil
}

func (u *ArticleUsecaseImpl) PublishArticle(ctx context.Context, input PublishArticleInput) (*dto.ArticleDTO, error) {
	article, err := u.articleService.SetPublished(ctx, input.ID, true)
	if err != nil {
		return nil, err
	}
	return dto.FromArticle(article), nil
}

func (u *ArticleUsecaseImpl) UnpublishArticle(ctx context.Context, input UnpublishArticleInput) (*dto.ArticleDTO, error) {
	article, err := u.articleService.SetPublished(ctx, input.ID, false)
	if err != nil {
		return nil, err
	}
	return dto.FromArticle(article), nil
}

func (u *ArticleUsecaseImpl) DeleteArticle(ctx context.Context, input DeleteArticleInput) error {
	return u.articleService.Delete(ctx, input.ID)
}
