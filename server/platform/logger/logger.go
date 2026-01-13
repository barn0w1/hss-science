package logger

import (
	"log/slog"
	"os"
)

type Config struct {
	IsDev bool
}

// Setup はグローバルロガーを設定します
// アプリ起動時に1回だけ呼び出してください
func Setup(cfg Config) {
	var handler slog.Handler

	if cfg.IsDev {
		// 開発時: 人間が読みやすいテキスト形式 & DEBUGレベルまで出す
		// 例: time=... level=INFO msg="Hello"
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	} else {
		// 本番時: 機械が読みやすいJSON形式 & INFOレベル以上
		// 例: {"time":"...", "level":"INFO", "msg":"Hello"}
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	// グローバルのデフォルトロガーを上書きする
	// これにより、slog.Info() と書くだけでこの設定が適用される
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
