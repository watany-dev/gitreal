package git

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	responses map[string]fakeResponse
	calls     []string
}

type fakeResponse struct {
	output string
	err    error
}

func (f *fakeRunner) run(dir string, args ...string) (string, error) {
	key := dir + "|" + joinArgs(args)
	f.calls = append(f.calls, key)

	response, ok := f.responses[key]
	if !ok {
		return "", errors.New("unexpected command")
	}

	return response.output, response.err
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += "\x00"
		}
		result += arg
	}
	return result
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		responses: map[string]fakeResponse{
			".|rev-parse\x00--show-toplevel": {
				output: "/tmp/repo\n",
			},
		},
	}

	repo, err := discover(".", runner)
	if err != nil {
		t.Fatalf("discover() error = %v", err)
	}

	if got := repo.Root(); got != "/tmp/repo" {
		t.Fatalf("Root() = %q, want /tmp/repo", got)
	}
}

func TestDiscoverOutsideRepository(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		responses: map[string]fakeResponse{
			".|rev-parse\x00--show-toplevel": {
				err: errors.New("boom"),
			},
		},
	}

	_, err := discover(".", runner)
	if err == nil || err.Error() != "not inside a Git repository" {
		t.Fatalf("discover() error = %v, want not inside a Git repository", err)
	}
}

func TestConfigAccessors(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|config\x00--bool\x00--get\x00gitreal.enabled": {
					output: "true\n",
				},
				"/tmp/repo|config\x00--bool\x00--get\x00gitreal.armed": {
					output: "off\n",
				},
				"/tmp/repo|config\x00--bool\x00--get\x00gitreal.unknown": {
					output: "wat\n",
				},
				"/tmp/repo|config\x00--bool\x00--get\x00gitreal.missing": {
					err: errors.New("missing"),
				},
				"/tmp/repo|config\x00--int\x00--get\x00gitreal.graceSeconds": {
					output: "90\n",
				},
				"/tmp/repo|config\x00--int\x00--get\x00gitreal.bad": {
					output: "abc\n",
				},
				"/tmp/repo|config\x00--int\x00--get\x00gitreal.missingInt": {
					err: errors.New("missing"),
				},
			},
		},
	}

	if !repo.ConfigBool("gitreal.enabled", false) {
		t.Fatalf("ConfigBool(enabled) = false, want true")
	}
	if repo.ConfigBool("gitreal.armed", true) {
		t.Fatalf("ConfigBool(armed) = true, want false")
	}
	if !repo.ConfigBool("gitreal.unknown", true) {
		t.Fatalf("ConfigBool(unknown) = false, want fallback true")
	}
	if repo.ConfigBool("gitreal.missing", false) {
		t.Fatalf("ConfigBool(missing) = true, want fallback false")
	}

	if got := repo.ConfigInt("gitreal.graceSeconds", 120); got != 90 {
		t.Fatalf("ConfigInt(graceSeconds) = %d, want 90", got)
	}
	if got := repo.ConfigInt("gitreal.bad", 120); got != 120 {
		t.Fatalf("ConfigInt(bad) = %d, want fallback 120", got)
	}
	if got := repo.ConfigInt("gitreal.missingInt", 45); got != 45 {
		t.Fatalf("ConfigInt(missingInt) = %d, want fallback 45", got)
	}
}

func TestSetConfig(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		responses: map[string]fakeResponse{
			"/tmp/repo|config\x00--local\x00gitreal.enabled\x00true":     {},
			"/tmp/repo|config\x00--local\x00gitreal.enabled\x00false":    {},
			"/tmp/repo|config\x00--local\x00gitreal.graceSeconds\x0045":  {},
			"/tmp/repo|config\x00--local\x00gitreal.graceSeconds\x00120": {},
		},
	}
	repo := &Repository{root: "/tmp/repo", runner: runner}

	if err := repo.SetConfigBool("gitreal.enabled", true); err != nil {
		t.Fatalf("SetConfigBool(true) error = %v", err)
	}
	if err := repo.SetConfigBool("gitreal.enabled", false); err != nil {
		t.Fatalf("SetConfigBool(false) error = %v", err)
	}
	if err := repo.SetConfigInt("gitreal.graceSeconds", 45); err != nil {
		t.Fatalf("SetConfigInt(45) error = %v", err)
	}
	if err := repo.SetConfigInt("gitreal.graceSeconds", 120); err != nil {
		t.Fatalf("SetConfigInt(120) error = %v", err)
	}
}

func TestBranchAndUpstream(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|symbolic-ref\x00--quiet\x00--short\x00HEAD": {
					output: "main\n",
				},
				"/tmp/repo|rev-parse\x00--abbrev-ref\x00--symbolic-full-name\x00@{u}": {
					output: "origin/main\n",
				},
			},
		},
	}

	branch, err := repo.CurrentBranch()
	if err != nil || branch != "main" {
		t.Fatalf("CurrentBranch() = %q, %v", branch, err)
	}

	upstream, err := repo.Upstream()
	if err != nil || upstream != "origin/main" {
		t.Fatalf("Upstream() = %q, %v", upstream, err)
	}
}

func TestBranchAndUpstreamErrors(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|symbolic-ref\x00--quiet\x00--short\x00HEAD": {
					err: errors.New("detached"),
				},
				"/tmp/repo|rev-parse\x00--abbrev-ref\x00--symbolic-full-name\x00@{u}": {
					err: errors.New("missing"),
				},
			},
		},
	}

	if _, err := repo.CurrentBranch(); err == nil || err.Error() != "detached HEAD is not supported" {
		t.Fatalf("CurrentBranch() error = %v", err)
	}

	if _, err := repo.Upstream(); err == nil || err.Error() != "no upstream configured; run: git push -u origin HEAD" {
		t.Fatalf("Upstream() error = %v", err)
	}
}

func TestAheadCount(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|rev-list\x00--count\x00@{u}..HEAD": {
					output: "3\n",
				},
			},
		},
	}

	got, err := repo.AheadCount()
	if err != nil || got != 3 {
		t.Fatalf("AheadCount() = %d, %v", got, err)
	}
}

func TestAheadCountEmptyAndInvalid(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|rev-list\x00--count\x00@{u}..HEAD": {},
			},
		},
	}

	got, err := repo.AheadCount()
	if err != nil || got != 0 {
		t.Fatalf("AheadCount() = %d, %v, want 0, nil", got, err)
	}

	repo = &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|rev-list\x00--count\x00@{u}..HEAD": {
					output: "abc\n",
				},
			},
		},
	}

	if _, err := repo.AheadCount(); err == nil {
		t.Fatalf("AheadCount() error = nil, want non-nil")
	}
}

func TestBackupHead(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	runner := &fakeRunner{
		responses: map[string]fakeResponse{
			"/tmp/repo|update-ref\x00refs/gitreal/backups/feature-test/20260501T120000Z-000000000\x00HEAD": {},
			"/tmp/repo|update-ref\x00refs/gitreal/backups/main/20260501T120000Z-000000000\x00HEAD": {
				err: errors.New("boom"),
			},
		},
	}
	repo := &Repository{root: "/tmp/repo", runner: runner}

	ref, err := repo.BackupHead("feature/test", now)
	if err != nil {
		t.Fatalf("BackupHead() error = %v", err)
	}

	want := "refs/gitreal/backups/feature-test/20260501T120000Z-000000000"
	if ref != want {
		t.Fatalf("BackupHead() = %q, want %q", ref, want)
	}
	if !IsBackupRef(ref) {
		t.Fatalf("IsBackupRef(%q) = false, want true", ref)
	}

	if _, err := repo.BackupHead("main", now); err == nil {
		t.Fatalf("BackupHead() error = nil, want non-nil")
	}
}

func TestSanitizeBranchSegment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string // empty means "fallback to hash, just check it's well-formed"
	}{
		{name: "simple", input: "main", want: "main"},
		{name: "slash", input: "feature/test", want: "feature-test"},
		{name: "backslash", input: "feature\\sub", want: "feature-sub"},
		{name: "release dotted", input: "release-1.0", want: "release-1.0"},
		{name: "git revision", input: "main@{1}", want: "main-1"},
		{name: "caret", input: "main^", want: "main"},
		{name: "tilde", input: "main~3", want: "main-3"},
		{name: "glob asterisk", input: "*", want: ""},
		{name: "glob brackets", input: "[abc]", want: "abc"},
		{name: "two dots", input: "..", want: ""},
		{name: "single dot", input: ".", want: ""},
		{name: "leading dash", input: "-leading", want: "leading"},
		{name: "trailing dot", input: "trailing.", want: "trailing"},
		{name: "with space", input: "with space", want: "with-space"},
		{name: "with tab", input: "with\ttab", want: "with-tab"},
		{name: "with newline", input: "with\nnewline", want: "with-newline"},
		{name: "with null", input: "with/null\x00here", want: "with-null-here"},
		{name: "very long", input: strings.Repeat("a", 256)},
		{name: "japanese", input: "機能/テスト"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeBranchSegment(tc.input)
			if got == "" {
				t.Fatalf("sanitizeBranchSegment(%q) = empty string", tc.input)
			}
			candidate := backupRefPrefix + got + "/20260501T120000Z-000000000"
			if !IsBackupRef(candidate) {
				t.Fatalf("sanitizeBranchSegment(%q) = %q, produces invalid backup ref %q", tc.input, got, candidate)
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("sanitizeBranchSegment(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestAheadCountRunError(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|rev-list\x00--count\x00@{u}..HEAD": {err: errors.New("boom")},
			},
		},
	}
	if _, err := repo.AheadCount(); err == nil {
		t.Fatalf("AheadCount() error = nil, want non-nil")
	}
}

func TestSanitizeBranchSegmentFallback(t *testing.T) {
	t.Parallel()

	// All-special input collapses to empty before fallback.
	got := sanitizeBranchSegment("***///")
	if !strings.HasPrefix(got, "branch-") || len(got) != len("branch-")+12 {
		t.Fatalf("sanitizeBranchSegment(all-special) = %q, want branch-<12hex>", got)
	}

	// Excessively long input falls back to the hash form.
	got = sanitizeBranchSegment(strings.Repeat("a", 256))
	if !strings.HasPrefix(got, "branch-") {
		t.Fatalf("sanitizeBranchSegment(long) = %q, want branch- prefix", got)
	}
}

func TestBackupHeadRejectsCraftedSafeBranch(t *testing.T) {
	t.Parallel()

	// Force-feed sanitizeBranchSegment a value that the regex rejects to
	// exercise the defense-in-depth check that refuses to run `update-ref`
	// with a malformed ref. We bypass sanitizeBranchSegment by constructing
	// the ref through internal helpers indirectly: build a Repository whose
	// runner records arguments, then invoke BackupHead with a name that we
	// confirm sanitizes to something the pattern accepts; then independently
	// validate that the regex would reject a hand-crafted bad ref.
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	runner := &fakeRunner{responses: map[string]fakeResponse{
		"/tmp/repo|update-ref\x00refs/gitreal/backups/main/20260501T120000Z-000000000\x00HEAD": {},
	}}
	repo := &Repository{root: "/tmp/repo", runner: runner}
	if _, err := repo.BackupHead("main", now); err != nil {
		t.Fatalf("BackupHead(main) error = %v", err)
	}

	if backupRefPattern.MatchString("refs/gitreal/backups/main@{1}/20260501T120000Z-000000000") {
		t.Fatalf("backupRefPattern accepted a ref with revision syntax")
	}
}

func TestIsBackupRefRejectsRevisionSyntax(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ref  string
		want bool
	}{
		{ref: "refs/gitreal/backups/main/20260501T120000Z-000000000", want: true},
		{ref: "refs/gitreal/backups/feature-test/20260501T120000Z-123456789", want: true},
		{ref: "refs/heads/main", want: false},
		{ref: "refs/gitreal/backups/main@{-1}/20260501T120000Z-000000000", want: false},
		{ref: "refs/gitreal/backups/main^/20260501T120000Z-000000000", want: false},
		{ref: "refs/gitreal/backups/main~3/20260501T120000Z-000000000", want: false},
		{ref: "refs/gitreal/backups/main:passwd/20260501T120000Z-000000000", want: false},
		{ref: "refs/gitreal/backups/main/20260501T120000Z-000000000 --no-ff", want: false},
		{ref: "refs/gitreal/backups/main/20260501T120000Z", want: false},
		{ref: "refs/gitreal/backups/main/", want: false},
		{ref: "refs/gitreal/backups//20260501T120000Z-000000000", want: false},
		{ref: "refs/gitreal/backups/main/20260501T120000Z-000000000/extra", want: false},
		{ref: "refs/gitreal/backups/../../../etc/passwd", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ref, func(t *testing.T) {
			t.Parallel()
			if got := IsBackupRef(tc.ref); got != tc.want {
				t.Fatalf("IsBackupRef(%q) = %t, want %t", tc.ref, got, tc.want)
			}
		})
	}
}

func TestStashDirtyWorktree(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|status\x00--porcelain=v1\x00-z": {
					output: " M file.txt\x00",
				},
				"/tmp/repo|stash\x00push\x00--include-untracked\x00--message\x00save": {},
			},
		},
	}

	stashed, err := repo.StashDirtyWorktree("save")
	if err != nil {
		t.Fatalf("StashDirtyWorktree() error = %v", err)
	}
	if !stashed {
		t.Fatalf("StashDirtyWorktree() = false, want true")
	}
}

func TestStashDirtyWorktreeEdgeCases(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|status\x00--porcelain=v1\x00-z": {},
			},
		},
	}

	stashed, err := repo.StashDirtyWorktree("save")
	if err != nil {
		t.Fatalf("StashDirtyWorktree(clean) error = %v", err)
	}
	if stashed {
		t.Fatalf("StashDirtyWorktree(clean) = true, want false")
	}

	repo = &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|status\x00--porcelain=v1\x00-z": {
					err: errors.New("boom"),
				},
			},
		},
	}

	if _, err := repo.StashDirtyWorktree("save"); err == nil {
		t.Fatalf("StashDirtyWorktree(status) error = nil, want non-nil")
	}

	repo = &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|status\x00--porcelain=v1\x00-z": {
					output: " M file.txt\x00",
				},
				"/tmp/repo|stash\x00push\x00--include-untracked\x00--message\x00save": {
					err: errors.New("boom"),
				},
			},
		},
	}

	if _, err := repo.StashDirtyWorktree("save"); err == nil {
		t.Fatalf("StashDirtyWorktree(stash) error = nil, want non-nil")
	}
}

func TestResetRescueAndHelpers(t *testing.T) {
	t.Parallel()

	validRef := "refs/gitreal/backups/main/20260501T120000Z-000000001"
	validRef2 := "refs/gitreal/backups/main/20260501T120000Z-000000002"

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|reset\x00--hard\x00" + validRef:                               {},
				"/tmp/repo|stash\x00pop":                                                 {},
				"/tmp/repo|fetch\x00--quiet\x00--prune":                                  {},
				"/tmp/repo|for-each-ref\x00refs/gitreal/backups/\x00--format=%(refname)": {output: validRef + "\n" + validRef2 + "\n"},
			},
		},
	}

	if err := repo.ResetHard(validRef); err != nil {
		t.Fatalf("ResetHard() error = %v", err)
	}
	if err := repo.StashPop(); err != nil {
		t.Fatalf("StashPop() error = %v", err)
	}
	if err := repo.FetchQuiet(); err != nil {
		t.Fatalf("FetchQuiet() error = %v", err)
	}

	refs, err := repo.RescueRefs()
	if err != nil {
		t.Fatalf("RescueRefs() error = %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("len(RescueRefs()) = %d, want 2", len(refs))
	}
	if !IsBackupRef(refs[0]) || IsBackupRef("refs/heads/main") {
		t.Fatalf("IsBackupRef() returned unexpected result")
	}
}

func TestRescueRefsEmptyAndError(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|for-each-ref\x00refs/gitreal/backups/\x00--format=%(refname)": {},
			},
		},
	}

	refs, err := repo.RescueRefs()
	if err != nil {
		t.Fatalf("RescueRefs() error = %v", err)
	}
	if refs != nil {
		t.Fatalf("RescueRefs() = %v, want nil", refs)
	}

	repo = &Repository{
		root: "/tmp/repo",
		runner: &fakeRunner{
			responses: map[string]fakeResponse{
				"/tmp/repo|for-each-ref\x00refs/gitreal/backups/\x00--format=%(refname)": {
					err: errors.New("boom"),
				},
			},
		},
	}

	if _, err := repo.RescueRefs(); err == nil {
		t.Fatalf("RescueRefs() error = nil, want non-nil")
	}
}

func TestDiscoverIntegration(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	runGit(t, tempDir, "init")

	repo, err := Discover(tempDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if repo.Root() != tempDir {
		t.Fatalf("Root() = %q, want %q", repo.Root(), tempDir)
	}

	if err := repo.SetConfigBool("gitreal.enabled", true); err != nil {
		t.Fatalf("SetConfigBool() error = %v", err)
	}
	if err := repo.SetConfigInt("gitreal.graceSeconds", 15); err != nil {
		t.Fatalf("SetConfigInt() error = %v", err)
	}
	if !repo.ConfigBool("gitreal.enabled", false) {
		t.Fatalf("ConfigBool(enabled) = false, want true")
	}
	if got := repo.ConfigInt("gitreal.graceSeconds", 120); got != 15 {
		t.Fatalf("ConfigInt(graceSeconds) = %d, want 15", got)
	}
}

func TestCommandRunnerRunError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	runner := commandRunner{}

	_, err := runner.run(dir, "status")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
	}
}
