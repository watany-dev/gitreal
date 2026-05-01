# GitReal Development Memo

## 結論

- 実行ファイル名は `git-real` にする。
- ユーザー向けコマンドは `git real`。
- 実装言語は Go を採用する。

Git は `git <command>` が core Git command でない場合、`PATH` 上の `git-<command>` を探して実行し、後続引数をそのまま渡す。したがって `git-real` という実行ファイルを配布すれば、ユーザーは以下のように実行できる。

```bash
git real
git real start
git real once
git real status
git real rescue
```

## Go を選ぶ理由

GitReal は以下が中心になる。

- Git plumbing を呼ぶ CLI
- 通知
- タイマー常駐
- クロスプラットフォーム配布

この用途では Go の単一バイナリ配布が強い。Python はユーザー環境の Python バージョンや依存関係が面倒になりやすい。Rust も有力だが、この用途では Go の方が実装速度と配布のバランスがよい。Node は Git ユーザー向け CLI としてはランタイム依存がやや重い。

## 想定ディレクトリ構成

```text
git-real/
  go.mod
  cmd/
    git-real/
      main.go
  internal/
    cli/
      app.go
    git/
      git.go
    challenge/
      grace.go
    notify/
      notify.go
  README.md
```

現状の実装は `cmd/git-real/main.go` から `internal/cli` を呼び、Git 実行は `internal/git`、通知は `internal/notify` に分離している。`config` と `daemon` は将来拡張用として未実装。

## ユーザー体験

```bash
# インストール
go install github.com/watany-dev/gitreal/cmd/git-real@latest

# 対象 repo で初期化
git real init

# dry-run で開始
git real start

# 本当に reset する危険モードを有効化
git real arm

# 今すぐ1回だけ発火
git real once

# 退避された commit を確認
git real rescue list

# 復旧
git real rescue restore <backup-ref>
```

`git real once`、`git real start`、`git real arm`、`git real disarm` は `git real init` 後にのみ動かす。未初期化でも `git real status` と `git real rescue ...` は使える。

現状の制約も明示しておく。

- challenge 実行には upstream branch が必要
- detached HEAD は非対応
- 通知は best-effort で、使えない環境では標準出力へのフォールバックになる
- `git real daemon` はまだ未実装

## 設定方針

設定は Git config に入れる。

```bash
git config --local gitreal.enabled true
git config --local gitreal.armed false
git config --local gitreal.graceSeconds 120
```

`gitreal.*` のような独自 namespace を使う。Git config は repository local の `.git/config`、global の `~/.gitconfig`、system-wide の config を扱えるため、この方針が自然。

## Hook 方針

Hook だけで作るのは不向き。Git hooks は `commit`、`push`、`merge` など Git 実行タイミングに反応する仕組みであり、ランダム時刻に通知する scheduler ではない。

- `git real once`: その場でチャレンジを 1 回実行
- `git real start`: Public Beta 時点の常駐入口。フォアグラウンドで常駐し、毎時ランダムな時刻に通知
- `git real daemon`: `launchd` / `systemd` / Windows Task Scheduler から起動される本番用
- `git real init`: repo local config に `gitreal.enabled=true` を書く
- `git real arm`: `gitreal.armed=true` を書く
- `git real rescue`: `refs/gitreal/backups/...` に退避した `HEAD` を一覧・復旧する

hook は当面使わない。将来追加するとしても `post-commit` で「未 push commit ができたので GitReal 対象になった」と通知する程度に留める。

## Git 状態判定

Go から `.git` ディレクトリを直接読むより、Git の状態判定はすべて `git` コマンド経由にする。

```bash
git rev-parse --show-toplevel
git symbolic-ref --quiet --short HEAD
git rev-parse --abbrev-ref --symbolic-full-name @{u}
git fetch --quiet --prune
git rev-list --count @{u}..HEAD
```

`@{u}..HEAD` が `0` なら push 済み、`1` 以上なら upstream より ahead。締切後に `fetch` してから判定すれば、通常の `git push` による remote 反映を確認できる。

## 処罰処理

```bash
git update-ref refs/gitreal/backups/<branch>/<timestamp> HEAD
git stash push --include-untracked --message "gitreal preserve worktree ..."
git reset --hard @{u}
git stash pop
```

この順序にする理由は、`reset --hard @{u}` でブランチ先端を upstream に戻す前に、元の `HEAD` を `refs/gitreal/backups/...` に退避するため。これにより「ローカルコミットがなかったことになる」体験を出しつつ、事故時は `git real rescue restore` で戻せる。

## Rescue restore 処理

`git real rescue restore <ref>` も破壊的操作なので、restore 前に現在の `HEAD` を `refs/gitreal/backups/...` に退避する。worktree が dirty な場合は stash してから restore し、restore 後に stash pop を試みる。

## コマンド一覧

```text
git real init
git real status
git real once
git real start
git real arm
git real disarm
git real rescue list
git real rescue restore <ref>
```

危険モードは必ず `git real arm` を明示的に要求する。デフォルトは dry-run。

## Public Beta 配布方針

- 正式なベータ導線は `go install` と GitHub Releases に絞る
- Release artifact は macOS / Linux / Windows 向けに出す
- Release には `SHA256SUMS` を含める
- `git real daemon` と Homebrew tap は次フェーズの課題として残す

## 初期ビルド

```bash
go mod init github.com/yourname/git-real
go build -o git-real ./cmd/git-real
```

## 動作確認

```bash
mkdir -p ~/bin
cp git-real ~/bin/git-real
export PATH="$HOME/bin:$PATH"

cd /path/to/repo
git real init
git real status
git real once --grace-seconds=10
```

`git real` が動くこと自体が、Git サブコマンドとして配布できている証拠になる。

## 配布方針

段階的に進める。

1. GitHub Releases に OS 別バイナリを置く
2. Homebrew tap を出す
3. `go install` を整備する

アーカイブ名の例:

```text
git-real_Darwin_arm64.tar.gz
git-real_Darwin_x86_64.tar.gz
git-real_Linux_x86_64.tar.gz
git-real_Windows_x86_64.zip
```

想定コマンド:

```bash
brew install yourname/tap/git-real
go install github.com/yourname/git-real/cmd/git-real@latest
```

将来的には Scoop、Winget、AUR を追加候補とするが、初期は Homebrew + GitHub Releases + `go install` で十分。

## README に必ず書くこと

- タイトルは `GitReal`
- タグラインは次を使う

```text
BeReal, but for Git.
When the notification hits, you have 2 minutes to push.
Miss it, and your local commits become unreal.
```

- デフォルトは dry-run
- 危険モードは `git real arm` が必要
- コミットは `refs/gitreal/backups/...` に退避される
- 復旧方法を明記する
