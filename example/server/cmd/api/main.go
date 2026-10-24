// Package main は API サーバーのエントリポイント。
//
// このファイルが唯一の DI ワイヤリング地点:
//
//	Repository(生成) → Service(手書き) → Usecase(手書き) → Handlers(生成) → ServeMux
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"example.com/blog/server/internal/di"
	"example.com/blog/server/internal/domain/service"
	infrapostgres "example.com/blog/server/internal/infra/postgres"
	infrarepo "example.com/blog/server/internal/infra/repository"
	"example.com/blog/server/internal/usecase"
	"example.com/blog/server/pkg/util/env"
	"example.com/blog/server/pkg/util/errs"
	"example.com/blog/server/pkg/util/logger"
	"example.com/blog/server/pkg/util/tx"
)

func main() {
	logger.Init()

	db, err := openDB()
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	tx.Init(db)

	// スキーマ適用は migrations/*.up.sql（make migration-gen で生成）を
	// golang-migrate で流す一本道にする。起動時に AutoMigrate は呼ばない
	// （GORM が期待する制約名と生成 SQL の制約名が食い違い、既存スキーマを壊すため）。
	// 生成済みの AutoMigrate<Entity> ヘルパーは integration test 用。

	handlers := buildHandlers()

	mux := http.NewServeMux()
	handlers.Register(mux)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	addr := env.Port()
	slog.Info("listening", "addr", addr, "env", env.AppEnv())
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func openDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		env.DBHost(), env.DBPort(), env.DBUser(), env.DBPassword(), env.DBName(),
	)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

// buildHandlers は Repository → Service → Usecase → Handlers の配線を行う。
// エンティティを増やしたときに触るのはこの関数だけ。
func buildHandlers() *di.Handlers {
	errWriter := errs.NewErrorWriter()
	rdbWrapper := errs.NewRDBErrorWrapper(infrapostgres.NewPostgresErrorChecker())

	userRepo := infrarepo.NewPostgresUserRepository(rdbWrapper)
	articleRepo := infrarepo.NewPostgresArticleRepository(rdbWrapper)

	userService := service.NewUserService(userRepo, nil, nil)
	articleService := service.NewArticleService(articleRepo, userRepo, nil, nil)

	userUsecase := usecase.NewUserUsecase(userService)
	articleUsecase := usecase.NewArticleUsecase(articleService)

	// 引数順は di.NewHandlers の生成順（service 名の昇順）に従う。
	return di.NewHandlers(errWriter, articleUsecase, userUsecase)
}
