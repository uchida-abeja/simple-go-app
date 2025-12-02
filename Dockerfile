# 🏗️ ステージ1: ビルド環境 (Builder)
# Goのコンパイラが含まれる公式イメージを使います
FROM golang:1.23-alpine AS builder

# ワーキングディレクトリを設定
WORKDIR /app

# 依存関係のファイルを先にコピー（キャッシュ効率のため）
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピーしてビルド
COPY . .
# CGO_ENABLED=0: 依存ライブラリを含まない完全な静的バイナリを作る設定
# -o main: 出力ファイル名を main にする
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# 🚀 ステージ2: 実行環境 (Runner)
# 軽量な Alpine Linux を使います（Distrolessなども一般的です）
FROM alpine:latest

# セキュリティのためにルート以外のユーザーを作成・使用（推奨）
WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# ステージ1で作ったバイナリだけをコピーしてくる
COPY --from=builder /app/main .

# コンテナ起動時に実行するコマンド
CMD ["./main"]