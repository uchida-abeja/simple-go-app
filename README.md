# simple-go-app

MinIO（S3互換API）のバケットとオブジェクトをHTTPで確認する、小さなGoアプリケーションです。
Argo CDとGitHub Actionsを組み合わせるブログ記事のサンプルとして、GitHub Actionsから[learn-k8s](https://github.com/uchida-abeja/learn-k8s)のマニフェスト更新PRを作ります。

## API

| メソッドとパス | 内容 |
|---|---|
| `GET /healthz` | プロセスのヘルスチェック。MinIOには接続しません |
| `GET /buckets` | バケット名の一覧 |
| `GET /buckets/:name/objects` | 指定したバケット内のオブジェクト名一覧 |

## ローカルで動かす

Go 1.25以降と、接続先のMinIOが必要です。まず`.env.example`を参考に環境変数を設定します。`MINIO_ENDPOINT`は`localhost:9000`のような`host:port`と、`http://`または`https://`で始まるURLの両方を指定できます。

```bash
export MINIO_ENDPOINT=localhost:9000
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY=minioadmin
export AWS_REGION=us-east-1

go run .
```

別のターミナルから確認します。

```bash
curl --fail http://localhost:8080/healthz
curl --fail http://localhost:8080/buckets
curl --fail http://localhost:8080/buckets/raw-data/objects
```

`MINIO_ACCESS_KEY`と`MINIO_SECRET_KEY`の実値を`.env.example`へ書かないでください。ローカル用の`.env`は`.gitignore`と`.dockerignore`の対象です。

## テスト

ユニットテストはMinIOを起動せずに実行できます。

```bash
go test -race ./...
go vet ./...
```

## コンテナイメージ

Dockerfileは、Goのクロスコンパイルを使って`linux/amd64`と`linux/arm64`を作れる構成です。ビルド用・実行用イメージはtagとdigestを固定し、実行時は非rootユーザー（UID/GID `65532`）を使います。

ローカルアーキテクチャ向けの確認例です。

```bash
docker build --tag simple-go-app:local .
docker run --rm \
  --publish 8080:8080 \
  --env MINIO_ENDPOINT=host.docker.internal:9000 \
  --env MINIO_ACCESS_KEY=minioadmin \
  --env MINIO_SECRET_KEY=minioadmin \
  simple-go-app:local
```

## GitHub ActionsとGitOps

`.github/workflows/deploy.yml`は`main`へのpushで次を実行します。

1. ユニットテストを実行する
2. amd64／arm64イメージ、SBOM、provenanceを作り、GHCRへpushする
3. commitのフルSHAをイメージタグとして、`learn-k8s`の`deploy/simple-go-app`ブランチを更新する
4. `learn-k8s`側のworkflowが、そのブランチからデプロイPRを作る

`latest`タグも目視確認用に作りますが、Kubernetesのマニフェストにはcommitを一意に表すフルSHAタグを書き込みます。マニフェストリポジトリの`main`を直接更新しないため、デプロイ前にPRの差分をレビューできます。

### 必要なSecret: `GITOPS_DEPLOY_KEY`

アプリケーションリポジトリからマニフェストリポジトリのbot専用ブランチへpushするため、SSH deploy keyを1組用意します。従来の`PAT_FOR_GITOPS`は使いません。

秘密鍵を誤ってリポジトリへ追加しないよう、鍵はリポジトリの外にある一時ディレクトリで生成します。

```bash
KEY_DIR="$(mktemp -d "${TMPDIR:-/tmp}/simple-go-app-gitops.XXXXXX")"
KEY_PATH="${KEY_DIR}/deploy-key"

cleanup_key() {
  rm -rf -- "${KEY_DIR}"
}
trap cleanup_key EXIT

ssh-keygen -q -t ed25519 -N "" \
  -C "GitHub Actions manifest update" \
  -f "${KEY_PATH}"
```

1. `${KEY_PATH}.pub`を`learn-k8s`の`Settings → Deploy keys`へ登録し、`Allow write access`を有効にする
2. 秘密鍵`${KEY_PATH}`を、このリポジトリのActions secret `GITOPS_DEPLOY_KEY`へ登録する
3. 登録後に`cleanup_key`を実行し、`trap - EXIT`で終了時処理を解除する

GitHub CLIで2を行う場合は、リポジトリを明示します。

```bash
gh secret set GITOPS_DEPLOY_KEY \
  --repo uchida-abeja/simple-go-app \
  < "${KEY_PATH}"

cleanup_key
trap - EXIT
```

`GITHUB_TOKEN`はGHCRへのpushにだけ使い、ジョブ単位で`packages: write`を付与しています。Actionsとベースイメージはcommit SHAまたはdigest、KustomizeはバージョンとSHA-256を固定しています。更新時はDependabot等で新しい値を確認してから変更してください。

### 自分のリポジトリで試す場合

フォークまたはコピーして使う場合は、ワークフローとマニフェストに残るサンプル所有者名を置き換えます。

- このリポジトリの`.github/workflows/deploy.yml`: `IMAGE_NAME`と`MANIFEST_REPO`
- マニフェストリポジトリの`apps/simple-go-app/overlays/dev/kustomization.yaml`: `images[].newName`
- `GITOPS_DEPLOY_KEY`: 自分のマニフェストリポジトリへ登録したdeploy keyの秘密鍵

GHCRのイメージ名は小文字にします。置換後は、誤って別の所有者のパッケージやリポジトリを操作しないよう、push前に差分を確認してください。

## デプロイ先

Kubernetesマニフェストは`uchida-abeja/learn-k8s`で管理しています。

- base: `apps/simple-go-app/base/`
- dev overlay: `apps/simple-go-app/overlays/dev/`
- Argo CD Application: `infrastructure/argocd/simple-go-app-application.yaml`

GHCRパッケージがprivateの場合、クラスタには別途`read:packages`権限のPATから作った`ImagePullSecret`が必要です。これはGitを更新する`GITOPS_DEPLOY_KEY`とは別の認証情報です。
