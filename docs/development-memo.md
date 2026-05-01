# GitReal Development Memo

最終更新: 2026-05-01

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

現状の実装は `cmd/git-real/main.go` から `internal/cli` を呼び、Git 実行は `internal/git`、通知は `internal/notify` に分離している。`config` と `daemon` は将来拡張用の想定で、MVP ではまだ導入していない。

## ユーザー体験

```bash
# インストール
brew install your/tap/git-real

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
- `git real start`: フォアグラウンドで常駐し、毎時ランダムな時刻に通知
- `git real daemon`: `launchd` / `systemd` / Windows Task Scheduler から起動される本番用
- `git real init`: repo local config に `gitreal.enabled=true` を書く
- `git real arm`: `gitreal.armed=true` を書く
- `git real rescue`: `refs/gitreal/backups/...` に退避した `HEAD` を一覧・復旧する

MVP では hook は使わない。後から追加するなら `post-commit` で「未 push commit ができたので GitReal 対象になった」と通知する程度に留める。

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

## MVP コマンド

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

## 初期プロトタイプ

初期の単一ファイル案は以下。現在の repository 実装はこの責務を `internal/cli`、`internal/git`、`internal/notify` に分割している。

```go
package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type GitRealError struct {
	Message string
}

func (e GitRealError) Error() string {
	return e.Message
}

type Git struct {
	Repo string
}

func main() {
	exitCode := run(os.Args)
	os.Exit(exitCode)
}

func run(args []string) int {
	if len(args) < 2 {
		printHelp()
		return 0
	}

	command := args[1]

	switch command {
	case "init":
		return commandInit(args[2:])
	case "status":
		return commandStatus(args[2:])
	case "once":
		return commandOnce(args[2:])
	case "start":
		return commandStart(args[2:])
	case "arm":
		return commandArm(args[2:])
	case "disarm":
		return commandDisarm(args[2:])
	case "rescue":
		return commandRescue(args[2:])
	case "help", "-h", "--help":
		printHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "git-real: unknown command: %s\n", command)
		printHelp()
		return 2
	}
}

func printHelp() {
	fmt.Println(`git-real - BeReal-inspired punishment CLI for Git

Usage:
  git real init
  git real status
  git real once [--grace-seconds=120]
  git real start [--grace-seconds=120]
  git real arm
  git real disarm
  git real rescue list
  git real rescue restore <backup-ref>

Git invokes this binary as "git real" when the executable is named "git-real" and is on PATH.`)
}

func commandInit(args []string) int {
	repo, err := discoverRepo(".")
	if err != nil {
		return fail(err)
	}

	g := Git{Repo: repo}

	if err := g.Run("config", "--local", "gitreal.enabled", "true"); err != nil {
		return fail(err)
	}
	if err := g.Run("config", "--local", "gitreal.armed", "false"); err != nil {
		return fail(err)
	}
	if err := g.Run("config", "--local", "gitreal.graceSeconds", "120"); err != nil {
		return fail(err)
	}

	fmt.Println("GitReal initialized for:")
	fmt.Println(repo)
	fmt.Println("Mode: dry-run")
	fmt.Println("Run: git real once")
	return 0
}

func commandStatus(args []string) int {
	repo, err := discoverRepo(".")
	if err != nil {
		return fail(err)
	}

	g := Git{Repo: repo}

	branch, err := g.CurrentBranch()
	if err != nil {
		return fail(err)
	}

	upstream, err := g.Upstream()
	if err != nil {
		return fail(err)
	}

	_ = g.FetchQuiet()

	ahead, err := g.AheadCount()
	if err != nil {
		return fail(err)
	}

	armed := g.ConfigBool("gitreal.armed", false)

	fmt.Printf("repo:     %s\n", repo)
	fmt.Printf("branch:   %s\n", branch)
	fmt.Printf("upstream: %s\n", upstream)
	fmt.Printf("ahead:    %d\n", ahead)
	fmt.Printf("armed:    %t\n", armed)

	return 0
}

func commandOnce(args []string) int {
	fs := flag.NewFlagSet("once", flag.ContinueOnError)
	graceSeconds := fs.Int("grace-seconds", 120, "seconds before punishment")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	repo, err := discoverRepo(".")
	if err != nil {
		return fail(err)
	}

	g := Git{Repo: repo}
	armed := g.ConfigBool("gitreal.armed", false)

	if err := runChallenge(g, *graceSeconds, armed); err != nil {
		return fail(err)
	}

	return 0
}

func commandStart(args []string) int {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	graceSeconds := fs.Int("grace-seconds", 120, "seconds before punishment")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	repo, err := discoverRepo(".")
	if err != nil {
		return fail(err)
	}

	g := Git{Repo: repo}
	armed := g.ConfigBool("gitreal.armed", false)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	next := nextRandomSlot(time.Now(), rng)

	fmt.Printf("GitReal started for %s\n", repo)
	fmt.Printf("next challenge: %s\n", next.Format(time.RFC3339))

	for {
		sleepUntil(next)

		if err := runChallenge(g, *graceSeconds, armed); err != nil {
			fmt.Fprintf(os.Stderr, "git-real: %s\n", err.Error())
		}

		next = nextRandomSlot(time.Now().Add(time.Hour), rng)
		fmt.Printf("next challenge: %s\n", next.Format(time.RFC3339))
	}
}

func commandArm(args []string) int {
	repo, err := discoverRepo(".")
	if err != nil {
		return fail(err)
	}

	g := Git{Repo: repo}

	if err := g.Run("config", "--local", "gitreal.armed", "true"); err != nil {
		return fail(err)
	}

	fmt.Println("GitReal is now armed for this repository.")
	return 0
}

func commandDisarm(args []string) int {
	repo, err := discoverRepo(".")
	if err != nil {
		return fail(err)
	}

	g := Git{Repo: repo}

	if err := g.Run("config", "--local", "gitreal.armed", "false"); err != nil {
		return fail(err)
	}

	fmt.Println("GitReal is now in dry-run mode for this repository.")
	return 0
}

func commandRescue(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: git real rescue list | git real rescue restore <backup-ref>")
		return 2
	}

	repo, err := discoverRepo(".")
	if err != nil {
		return fail(err)
	}

	g := Git{Repo: repo}

	switch args[0] {
	case "list":
		out, err := g.Output("for-each-ref", "refs/gitreal/backups", "--format=%(refname)")
		if err != nil {
			return fail(err)
		}
		text := strings.TrimSpace(out)
		if text == "" {
			fmt.Println("No GitReal backup refs found.")
			return 0
		}
		fmt.Println(text)
		return 0

	case "restore":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: git real rescue restore <backup-ref>")
			return 2
		}
		backupRef := args[1]
		if !strings.HasPrefix(backupRef, "refs/gitreal/backups/") {
			fmt.Fprintln(os.Stderr, "ref must start with refs/gitreal/backups/")
			return 2
		}
		if err := g.Run("reset", "--hard", backupRef); err != nil {
			return fail(err)
		}
		fmt.Printf("Restored: %s\n", backupRef)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "unknown rescue command: %s\n", args[0])
		return 2
	}
}

func runChallenge(g Git, graceSeconds int, armed bool) error {
	branch, err := g.CurrentBranch()
	if err != nil {
		return err
	}

	upstream, err := g.Upstream()
	if err != nil {
		return err
	}

	_ = g.FetchQuiet()

	ahead, err := g.AheadCount()
	if err != nil {
		return err
	}

	deadline := time.Now().Add(time.Duration(graceSeconds) * time.Second)

	message := fmt.Sprintf("%s has %d unpushed commit(s). Push before %s.", branch, ahead, deadline.Format("15:04:05"))
	notify("GitReal", message)

	fmt.Printf("repo:     %s\n", g.Repo)
	fmt.Printf("branch:   %s\n", branch)
	fmt.Printf("upstream: %s\n", upstream)
	fmt.Printf("ahead:    %d\n", ahead)
	fmt.Printf("deadline: %s\n", deadline.Format(time.RFC3339))

	sleepUntil(deadline)

	if err := g.FetchQuiet(); err != nil {
		notify("GitReal", "fetch failed; punishment skipped for safety.")
		return nil
	}

	aheadAfter, err := g.AheadCount()
	if err != nil {
		return err
	}

	if aheadAfter == 0 {
		notify("GitReal", "Push confirmed. You are GitReal.")
		return nil
	}

	if !armed {
		notify("GitReal dry-run", fmt.Sprintf("%d commit(s) would be reset.", aheadAfter))
		fmt.Printf("dry-run: would reset %d commit(s) to @{u}\n", aheadAfter)
		return nil
	}

	backupRef, err := g.BackupHead(branch)
	if err != nil {
		return err
	}

	stashed, err := g.StashDirtyWorktree(backupRef)
	if err != nil {
		return err
	}

	if err := g.Run("reset", "--hard", "@{u}"); err != nil {
		return err
	}

	if stashed {
		if err := g.Run("stash", "pop"); err != nil {
			fmt.Println("stash pop failed; your stash remains available via git stash list")
		}
	}

	notify("GitReal", fmt.Sprintf("Local commits made unreal. Backup: %s", backupRef))
	fmt.Printf("backup ref: %s\n", backupRef)
	fmt.Printf("restore: git real rescue restore %s\n", backupRef)

	return nil
}

func discoverRepo(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", GitRealError{Message: "not inside a Git repository"}
	}
	return strings.TrimSpace(string(out)), nil
}

func (g Git) Run(args ...string) error {
	_, err := g.Output(args...)
	return err
}

func (g Git) Output(args ...string) (string, error) {
	fullArgs := append([]string{"-C", g.Repo}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", GitRealError{
			Message: fmt.Sprintf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out))),
		}
	}
	return string(out), nil
}

func (g Git) CurrentBranch() (string, error) {
	out, err := g.Output("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", GitRealError{Message: "detached HEAD is not supported"}
	}
	return strings.TrimSpace(out), nil
}

func (g Git) Upstream() (string, error) {
	out, err := g.Output("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", GitRealError{Message: "no upstream configured; run: git push -u origin HEAD"}
	}
	return strings.TrimSpace(out), nil
}

func (g Git) FetchQuiet() error {
	return g.Run("fetch", "--quiet", "--prune")
}

func (g Git) AheadCount() (int, error) {
	out, err := g.Output("rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(out)
	if value == "" {
		return 0, nil
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (g Git) BackupHead(branch string) (string, error) {
	safeBranch := strings.ReplaceAll(branch, string(filepath.Separator), "-")
	safeBranch = strings.ReplaceAll(safeBranch, "/", "-")

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	ref := fmt.Sprintf("refs/gitreal/backups/%s/%s", safeBranch, timestamp)

	if err := g.Run("update-ref", ref, "HEAD"); err != nil {
		return "", err
	}

	return ref, nil
}

func (g Git) StashDirtyWorktree(backupRef string) (bool, error) {
	out, err := g.Output("status", "--porcelain=v1", "-z")
	if err != nil {
		return false, err
	}

	if out == "" {
		return false, nil
	}

	message := fmt.Sprintf("gitreal preserve worktree before penalty %s", backupRef)

	if err := g.Run("stash", "push", "--include-untracked", "--message", message); err != nil {
		return false, err
	}

	return true, nil
}

func (g Git) ConfigBool(key string, fallback bool) bool {
	out, err := g.Output("config", "--bool", "--get", key)
	if err != nil {
		return fallback
	}

	value := strings.ToLower(strings.TrimSpace(out))

	switch value {
	case "true", "yes", "on", "1":
		return true
	case "false", "no", "off", "0":
		return false
	default:
		return fallback
	}
}

func nextRandomSlot(base time.Time, rng *rand.Rand) time.Time {
	hourStart := base.Truncate(time.Hour)
	offset := time.Duration(rng.Intn(3600)) * time.Second
	candidate := hourStart.Add(offset)

	if !candidate.After(time.Now()) {
		candidate = hourStart.Add(time.Hour).Add(time.Duration(rng.Intn(3600)) * time.Second)
	}

	return candidate
}

func sleepUntil(target time.Time) {
	for {
		remaining := time.Until(target)
		if remaining <= 0 {
			return
		}
		if remaining > 30*time.Second {
			time.Sleep(30 * time.Second)
		} else {
			time.Sleep(remaining)
		}
	}
}

func notify(title string, message string) {
	fmt.Printf("\a%s: %s\n", title, message)

	switch runtime.GOOS {
	case "darwin":
		escapedTitle := strings.ReplaceAll(title, `"`, `\"`)
		escapedMessage := strings.ReplaceAll(message, `"`, `\"`)
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, escapedMessage, escapedTitle)
		_ = exec.Command("osascript", "-e", script).Run()

	case "linux":
		if commandExists("notify-send") {
			_ = exec.Command("notify-send", title, message).Run()
		}

	case "windows":
		if commandExists("powershell") {
			_ = exec.Command("powershell", "-NoProfile", "-Command", "[console]::beep(880,300)").Run()
		}
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func fail(err error) int {
	if err == nil {
		return 0
	}

	var gitRealError GitRealError
	if errors.As(err, &gitRealError) {
		fmt.Fprintf(os.Stderr, "git-real: %s\n", gitRealError.Message)
		return 1
	}

	fmt.Fprintf(os.Stderr, "git-real: %s\n", err.Error())
	return 1
}
```

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
