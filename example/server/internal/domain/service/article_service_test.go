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

var (
	fixedNow    = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	laterNow    = fixedNow.Add(24 * time.Hour)
	testCtx     = context.Background()
	passThrough = func(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }
)

// newArticleService はテスト用の ArticleService を組み立てる。
// clock は固定時刻、トランザクションは素通しにして DB なしで検証できるようにする。
func newArticleService(
	articleRepo *mock.MockArticleRepository,
	userRepo *mock.MockUserRepository,
	now time.Time,
) *service.ArticleService {
	return service.NewArticleService(articleRepo, userRepo, func() time.Time { return now }, passThrough)
}

func existingAuthor(t *testing.T) *mock.MockUserRepository {
	t.Helper()
	return &mock.MockUserRepository{
		SelectByPKFunc: func(_ context.Context, id string) (*entity.User, error) {
			return &entity.User{ID: id, Email: "author@example.com", Name: "author", CreatedAt: fixedNow}, nil
		},
	}
}

func TestArticleServiceCreateDraft(t *testing.T) {
	var inserted *entity.Article
	articleRepo := &mock.MockArticleRepository{
		InsertFunc: func(_ context.Context, a *entity.Article) error {
			inserted = a
			return nil
		},
	}

	got, err := newArticleService(articleRepo, existingAuthor(t), fixedNow).
		Create(testCtx, "hello-world", "Hello", "本文", "author-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.Published {
		t.Errorf("Published = true, 新規作成は下書きであるべき")
	}
	if got.Slug != "hello-world" || got.Title != "Hello" || got.AuthorID != "author-1" {
		t.Errorf("Create() = %+v", got)
	}
	if !got.CreatedAt.Equal(fixedNow) || !got.UpdatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt = %v, UpdatedAt = %v, want %v", got.CreatedAt, got.UpdatedAt, fixedNow)
	}
	if inserted == nil {
		t.Fatal("Insert が呼ばれていない")
	}
}

func TestArticleServiceCreateRejectsUnknownAuthor(t *testing.T) {
	userRepo := &mock.MockUserRepository{
		SelectByPKFunc: func(context.Context, string) (*entity.User, error) { return nil, nil },
	}

	_, err := newArticleService(&mock.MockArticleRepository{}, userRepo, fixedNow).
		Create(testCtx, "hello-world", "Hello", "本文", "missing")

	assertErrorType(t, err, errs.ErrorTypeValidation)
}

func TestArticleServiceCreateRejectsDuplicateSlug(t *testing.T) {
	articleRepo := &mock.MockArticleRepository{
		SelectBySlugFunc: func(_ context.Context, slug string) (*entity.Article, error) {
			return &entity.Article{ID: "other", Slug: slug}, nil
		},
		InsertFunc: func(context.Context, *entity.Article) error {
			t.Error("slug 重複時に Insert してはいけない")
			return nil
		},
	}

	_, err := newArticleService(articleRepo, existingAuthor(t), fixedNow).
		Create(testCtx, "taken", "Hello", "本文", "author-1")

	assertErrorType(t, err, errs.ErrorTypeDuplicate)
}

func TestArticleServiceUpdateKeepsPublishedAndCreatedAt(t *testing.T) {
	current := &entity.Article{
		ID: "a1", Slug: "old", Title: "Old", Body: "old body",
		AuthorID: "author-1", Published: true,
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
	var saved *entity.Article
	articleRepo := &mock.MockArticleRepository{
		SelectByPKFunc: func(context.Context, string) (*entity.Article, error) { return current, nil },
		UpdateFunc: func(_ context.Context, a *entity.Article) error {
			saved = a
			return nil
		},
	}

	got, err := newArticleService(articleRepo, existingAuthor(t), laterNow).
		Update(testCtx, "a1", "new", "New", "new body")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !got.Published {
		t.Error("Published が更新で落ちている")
	}
	if !got.CreatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt = %v, want %v（更新では変えない）", got.CreatedAt, fixedNow)
	}
	if !got.UpdatedAt.Equal(laterNow) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, laterNow)
	}
	if saved == nil || saved.Slug != "new" {
		t.Errorf("Update に渡された entity = %+v", saved)
	}
}

func TestArticleServiceUpdateNotFound(t *testing.T) {
	articleRepo := &mock.MockArticleRepository{
		SelectByPKFunc: func(context.Context, string) (*entity.Article, error) { return nil, nil },
	}

	_, err := newArticleService(articleRepo, existingAuthor(t), fixedNow).
		Update(testCtx, "missing", "slug", "title", "body")

	assertErrorType(t, err, errs.ErrorTypeNotFound)
}

func TestArticleServiceSetPublished(t *testing.T) {
	tests := []struct {
		name          string
		currentPublic bool
		want          bool
		wantUpdate    bool
	}{
		{name: "下書きを公開する", currentPublic: false, want: true, wantUpdate: true},
		{name: "公開を取り下げる", currentPublic: true, want: false, wantUpdate: true},
		{name: "同じ状態なら書き込まない", currentPublic: true, want: true, wantUpdate: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &entity.Article{
				ID: "a1", Slug: "s", Title: "T", Body: "B",
				AuthorID: "author-1", Published: tt.currentPublic,
				CreatedAt: fixedNow, UpdatedAt: fixedNow,
			}
			updateCalled := false
			articleRepo := &mock.MockArticleRepository{
				SelectByPKFunc: func(context.Context, string) (*entity.Article, error) { return current, nil },
				UpdateFunc: func(_ context.Context, a *entity.Article) error {
					updateCalled = true
					if a.Published != tt.want {
						t.Errorf("Update に渡された Published = %v, want %v", a.Published, tt.want)
					}
					return nil
				},
			}

			got, err := newArticleService(articleRepo, existingAuthor(t), laterNow).
				SetPublished(testCtx, "a1", tt.want)
			if err != nil {
				t.Fatalf("SetPublished() error = %v", err)
			}
			if got.Published != tt.want {
				t.Errorf("Published = %v, want %v", got.Published, tt.want)
			}
			if updateCalled != tt.wantUpdate {
				t.Errorf("Update 呼び出し = %v, want %v", updateCalled, tt.wantUpdate)
			}
		})
	}
}

func TestArticleServiceListByCursorNormalizesLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "0 は既定値", limit: 0, want: 20},
		{name: "負数は既定値", limit: -1, want: 20},
		{name: "上限超えは丸める", limit: 1000, want: 100},
		{name: "範囲内はそのまま", limit: 42, want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotLimit int
			var gotAfter *string
			articleRepo := &mock.MockArticleRepository{
				SelectByCursorFunc: func(_ context.Context, limit int, after *string) ([]*entity.Article, error) {
					gotLimit, gotAfter = limit, after
					return nil, nil
				},
			}

			if _, err := newArticleService(articleRepo, existingAuthor(t), fixedNow).
				ListByCursor(testCtx, tt.limit, ""); err != nil {
				t.Fatalf("ListByCursor() error = %v", err)
			}
			if gotLimit != tt.want {
				t.Errorf("limit = %d, want %d", gotLimit, tt.want)
			}
			if gotAfter != nil {
				t.Errorf("after = %v, want nil（先頭ページ）", *gotAfter)
			}
		})
	}
}

func assertErrorType(t *testing.T, err error, want errs.ErrorType) {
	t.Helper()
	if err == nil {
		t.Fatalf("エラーが返っていない (want %s)", want)
	}
	de, ok := errs.As(err)
	if !ok {
		t.Fatalf("DomainError ではない: %v", err)
	}
	if de.Type != want {
		t.Errorf("ErrorType = %s, want %s", de.Type, want)
	}
}
