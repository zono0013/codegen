// Package service はドメインロジックを持つ手書き層。
// 生成される Repository interface にのみ依存し、HTTP / proto / DTO は知らない。
package service

import (
	"context"
	"time"

	"example.com/blog/server/pkg/util/tx"
)

const (
	// defaultPageLimit は cursor pagination の limit 未指定時の既定値。
	defaultPageLimit = 20
	// maxPageLimit は 1 回のリクエストで返す上限。
	maxPageLimit = 100
)

// TxRunner はトランザクション境界を張る関数。本番は tx.Run、テストは素通し関数を注入する。
// tx.Run はプロセス全体で 1 つの *gorm.DB に依存するため、差し替え可能にしておく。
type TxRunner func(ctx context.Context, fn func(ctx context.Context) error) error

func orDefaultTxRunner(runner TxRunner) TxRunner {
	if runner == nil {
		return tx.Run
	}
	return runner
}

func orDefaultClock(clock func() time.Time) func() time.Time {
	if clock == nil {
		return time.Now
	}
	return clock
}

// normalizeLimit は cursor pagination の limit を既定値・上限に丸める。
func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultPageLimit
	case limit > maxPageLimit:
		return maxPageLimit
	default:
		return limit
	}
}
