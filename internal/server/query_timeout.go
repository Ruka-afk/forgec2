package server

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// installQueryTimeout registers GORM callbacks that inject a deadline into any
// query whose context does not already have one. This guarantees every DB read
// (list endpoints, stats, etc.) is bounded even if the calling handler forgot
// to pass a context. The deadline is cancelled in the After callback to avoid
// leaking timers.
func installQueryTimeout(db *gorm.DB, d time.Duration) {
	const key = "forgec2:query_cancel"

	db.Callback().Query().Before("gorm:query").Register("forgec2:before_query_timeout", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Context.Deadline(); ok {
			return
		}
		ctx, cancel := context.WithTimeout(tx.Statement.Context, d)
		tx.Statement.Context = ctx
		tx.InstanceSet(key, cancel)
	})

	db.Callback().Query().After("gorm:query").Register("forgec2:after_query_timeout", func(tx *gorm.DB) {
		if v, ok := tx.InstanceGet(key); ok {
			if cancel, ok := v.(context.CancelFunc); ok {
				cancel()
			}
		}
	})
}
