package cli

import (
	"bytes"
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	root             string
	configBools      map[string]bool
	configInts       map[string]int
	currentBranch    string
	currentBranchErr error
	upstream         string
	upstreamErr      error
	aheadCounts      []int
	aheadErr         error
	fetchErrors      []error
	backupRef        string
	backupErr        error
	stashDirty       bool
	stashErr         error
	stashPopErr      error
	rescueRefs       []string
	rescueErr        error
	resetErr         error
	setBoolErr       error
	setIntErr        error

	setBoolCalls  map[string]bool
	setIntCalls   map[string]int
	resetCalls    []string
	fetchCalls    int
	stashMessages []string
}

func (f *fakeRepo) Root() string { return f.root }

func (f *fakeRepo) SetConfigBool(key string, value bool) error {
	if f.setBoolErr != nil {
		return f.setBoolErr
	}
	if f.setBoolCalls == nil {
		f.setBoolCalls = map[string]bool{}
	}
	f.setBoolCalls[key] = value
	if f.configBools == nil {
		f.configBools = map[string]bool{}
	}
	f.configBools[key] = value
	return nil
}

func (f *fakeRepo) SetConfigInt(key string, value int) error {
	if f.setIntErr != nil {
		return f.setIntErr
	}
	if f.setIntCalls == nil {
		f.setIntCalls = map[string]int{}
	}
	f.setIntCalls[key] = value
	if f.configInts == nil {
		f.configInts = map[string]int{}
	}
	f.configInts[key] = value
	return nil
}

func (f *fakeRepo) ConfigBool(key string, fallback bool) bool {
	if value, ok := f.configBools[key]; ok {
		return value
	}
	return fallback
}

func (f *fakeRepo) ConfigInt(key string, fallback int) int {
	if value, ok := f.configInts[key]; ok {
		return value
	}
	return fallback
}

func (f *fakeRepo) CurrentBranch() (string, error) {
	if f.currentBranchErr != nil {
		return "", f.currentBranchErr
	}
	return f.currentBranch, nil
}

func (f *fakeRepo) Upstream() (string, error) {
	if f.upstreamErr != nil {
		return "", f.upstreamErr
	}
	return f.upstream, nil
}

func (f *fakeRepo) FetchQuiet() error {
	index := f.fetchCalls
	f.fetchCalls++
	if index < len(f.fetchErrors) {
		return f.fetchErrors[index]
	}
	return nil
}

func (f *fakeRepo) AheadCount() (int, error) {
	if f.aheadErr != nil {
		return 0, f.aheadErr
	}
	if len(f.aheadCounts) == 0 {
		return 0, nil
	}
	value := f.aheadCounts[0]
	if len(f.aheadCounts) > 1 {
		f.aheadCounts = f.aheadCounts[1:]
	}
	return value, nil
}

func (f *fakeRepo) BackupHead(branch string, now time.Time) (string, error) {
	if f.backupErr != nil {
		return "", f.backupErr
	}
	if f.backupRef != "" {
		return f.backupRef, nil
	}
	return "refs/gitreal/backups/" + branch + "/" + now.UTC().Format("20060102T150405Z"), nil
}

func (f *fakeRepo) StashDirtyWorktree(message string) (bool, error) {
	f.stashMessages = append(f.stashMessages, message)
	if f.stashErr != nil {
		return false, f.stashErr
	}
	return f.stashDirty, nil
}

func (f *fakeRepo) StashPop() error {
	return f.stashPopErr
}

func (f *fakeRepo) ResetHard(ref string) error {
	f.resetCalls = append(f.resetCalls, ref)
	return f.resetErr
}

func (f *fakeRepo) RescueRefs() ([]string, error) {
	if f.rescueErr != nil {
		return nil, f.rescueErr
	}
	return f.rescueRefs, nil
}

type fakeClock struct {
	current time.Time
}

func (f *fakeClock) now() time.Time {
	return f.current
}

func (f *fakeClock) sleep(duration time.Duration) {
	f.current = f.current.Add(duration)
}

func newTestApp(repo repository) (*app, *bytes.Buffer, *bytes.Buffer, *fakeClock, *[]string) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	clock := &fakeClock{
		current: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}
	notifications := []string{}

	testApp := &app{
		discoverRepo: func(path string) (repository, error) {
			return repo, nil
		},
		now:   clock.now,
		sleep: clock.sleep,
		sendNotification: func(title, message string) error {
			notifications = append(notifications, title+": "+message)
			return nil
		},
		rng:    rand.New(rand.NewSource(1)),
		stdout: stdout,
		stderr: stderr,
	}

	return testApp, stdout, stderr, clock, &notifications
}

func TestTopLevelRunAndNewApp(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if got := Run([]string{"help"}, stdout, stderr); got != 0 {
		t.Fatalf("Run(help) = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "git real once") {
		t.Fatalf("stdout = %q, want help output", stdout.String())
	}

	app := newApp(stdout, stderr)
	if app == nil || app.rng == nil || app.now == nil || app.sleep == nil {
		t.Fatalf("newApp() returned incomplete app: %#v", app)
	}

	if _, err := app.discoverRepo("."); err != nil {
		t.Fatalf("discoverRepo(.) error = %v", err)
	}
}

func TestRunHelpAndUnknown(t *testing.T) {
	t.Parallel()

	app, stdout, stderr, _, _ := newTestApp(&fakeRepo{})

	if got := app.run(nil); got != 0 {
		t.Fatalf("run(nil) = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want help output", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if got := app.run([]string{"wat"}); got != 2 {
		t.Fatalf("run(unknown) = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "unknown command: wat") {
		t.Fatalf("stderr = %q, want unknown command", stderr.String())
	}
}

func TestInitArmDisarmAndStatus(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		configBools:   map[string]bool{},
		configInts:    map[string]int{},
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{2},
	}
	app, stdout, _, _, _ := newTestApp(repo)

	if got := app.run([]string{"init"}); got != 0 {
		t.Fatalf("init exit code = %d, want 0", got)
	}
	if !repo.setBoolCalls["gitreal.enabled"] || repo.setBoolCalls["gitreal.armed"] {
		t.Fatalf("init config writes = %#v", repo.setBoolCalls)
	}
	if repo.setIntCalls["gitreal.graceSeconds"] != 120 {
		t.Fatalf("graceSeconds = %d, want 120", repo.setIntCalls["gitreal.graceSeconds"])
	}

	stdout.Reset()
	if got := app.run([]string{"arm"}); got != 0 {
		t.Fatalf("arm exit code = %d, want 0", got)
	}
	if !repo.configBools["gitreal.armed"] {
		t.Fatalf("armed config = false, want true")
	}

	stdout.Reset()
	if got := app.run([]string{"disarm"}); got != 0 {
		t.Fatalf("disarm exit code = %d, want 0", got)
	}
	if repo.configBools["gitreal.armed"] {
		t.Fatalf("armed config = true, want false")
	}

	stdout.Reset()
	if got := app.run([]string{"status"}); got != 0 {
		t.Fatalf("status exit code = %d, want 0", got)
	}

	statusOutput := stdout.String()
	for _, want := range []string{
		"repo: /tmp/repo",
		"enabled: true",
		"armed: false",
		"grace-seconds: 120",
		"branch: main",
		"upstream: origin/main",
		"ahead: 2",
	} {
		if !strings.Contains(statusOutput, want) {
			t.Fatalf("status output = %q, want substring %q", statusOutput, want)
		}
	}
}

func TestStatusWithoutUpstream(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		configBools:   map[string]bool{"gitreal.enabled": false, "gitreal.armed": false},
		configInts:    map[string]int{"gitreal.graceSeconds": 90},
		currentBranch: "main",
		upstreamErr:   errors.New("missing"),
	}
	app, stdout, _, _, _ := newTestApp(repo)

	if got := app.run([]string{"status"}); got != 0 {
		t.Fatalf("status exit code = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "upstream: <none>") || !strings.Contains(stdout.String(), "ahead: unknown") {
		t.Fatalf("stdout = %q, want missing upstream markers", stdout.String())
	}
}

func TestResolveGraceSeconds(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		configInts: map[string]int{"gitreal.graceSeconds": 30},
	}
	stderr := new(bytes.Buffer)

	got, err := resolveGraceSeconds(nil, repo, stderr)
	if err != nil || got != 30 {
		t.Fatalf("resolveGraceSeconds(nil) = %d, %v, want 30, nil", got, err)
	}

	got, err = resolveGraceSeconds([]string{"--grace-seconds=7200"}, repo, stderr)
	if err != nil || got != 3600 {
		t.Fatalf("resolveGraceSeconds(explicit) = %d, %v, want 3600, nil", got, err)
	}

	if _, err := resolveGraceSeconds([]string{"--grace-seconds=nope"}, repo, stderr); err == nil {
		t.Fatalf("resolveGraceSeconds(invalid) error = nil, want non-nil")
	}
	if _, err := resolveGraceSeconds([]string{"extra"}, repo, stderr); err == nil {
		t.Fatalf("resolveGraceSeconds(extra) error = nil, want non-nil")
	}
}

func TestCommandOnceViaRun(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		configBools:   map[string]bool{"gitreal.armed": false},
		configInts:    map[string]int{"gitreal.graceSeconds": 15},
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{0},
	}
	app, stdout, _, _, _ := newTestApp(repo)

	if got := app.run([]string{"once"}); got != 0 {
		t.Fatalf("once exit code = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "nothing to do") {
		t.Fatalf("stdout = %q, want once output", stdout.String())
	}
}

func TestCommandParseFailures(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		configBools:   map[string]bool{"gitreal.armed": false},
		configInts:    map[string]int{"gitreal.graceSeconds": 15},
		currentBranch: "main",
		upstream:      "origin/main",
	}
	app, _, stderr, _, _ := newTestApp(repo)

	if got := app.run([]string{"once", "--grace-seconds=nope"}); got != 2 {
		t.Fatalf("once invalid exit code = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "invalid value") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}

	stderr.Reset()
	if got := app.run([]string{"start", "extra"}); got != 2 {
		t.Fatalf("start invalid exit code = %d, want 2", got)
	}
}

func TestRunChallengeNoUnpushedCommits(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{0},
	}
	app, stdout, _, _, notifications := newTestApp(repo)

	if err := app.runChallenge(repo, 120, false); err != nil {
		t.Fatalf("runChallenge() error = %v", err)
	}
	if repo.fetchCalls != 1 {
		t.Fatalf("fetchCalls = %d, want 1", repo.fetchCalls)
	}
	if !strings.Contains(stdout.String(), "nothing to do: no unpushed commits") {
		t.Fatalf("stdout = %q, want nothing to do", stdout.String())
	}
	if len(*notifications) != 1 || !strings.Contains((*notifications)[0], "No unpushed commits") {
		t.Fatalf("notifications = %v, want no-unpushed message", *notifications)
	}
}

func TestRunChallengeDryRun(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{2, 2},
	}
	app, stdout, _, clock, notifications := newTestApp(repo)

	if err := app.runChallenge(repo, 120, false); err != nil {
		t.Fatalf("runChallenge() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "dry-run: would reset 2 commit(s) to @{u}") {
		t.Fatalf("stdout = %q, want dry-run message", stdout.String())
	}
	if clock.current != time.Date(2026, 5, 1, 12, 2, 0, 0, time.UTC) {
		t.Fatalf("clock.current = %s, want 2 minutes later", clock.current)
	}
	if len(*notifications) != 2 {
		t.Fatalf("notifications = %v, want 2 notifications", *notifications)
	}
}

func TestRunChallengePushConfirmed(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{1, 0},
	}
	app, stdout, _, _, notifications := newTestApp(repo)

	if err := app.runChallenge(repo, 30, false); err != nil {
		t.Fatalf("runChallenge() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "push confirmed") {
		t.Fatalf("stdout = %q, want push confirmed", stdout.String())
	}
	if len(*notifications) != 2 || !strings.Contains((*notifications)[1], "Push confirmed") {
		t.Fatalf("notifications = %v, want push confirmed", *notifications)
	}
}

func TestRunChallengeFetchFailureSkipsPunishment(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{1},
		fetchErrors:   []error{nil, errors.New("fetch failed")},
	}
	app, stdout, _, _, _ := newTestApp(repo)

	if err := app.runChallenge(repo, 30, true); err != nil {
		t.Fatalf("runChallenge() error = %v", err)
	}
	if len(repo.resetCalls) != 0 {
		t.Fatalf("resetCalls = %v, want none", repo.resetCalls)
	}
	if !strings.Contains(stdout.String(), "punishment skipped for safety") {
		t.Fatalf("stdout = %q, want skip message", stdout.String())
	}
}

func TestRunChallengeArmed(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{2, 2},
		backupRef:     "refs/gitreal/backups/main/20260501T120200Z",
		stashDirty:    true,
	}
	app, stdout, _, _, notifications := newTestApp(repo)

	if err := app.runChallenge(repo, 120, true); err != nil {
		t.Fatalf("runChallenge() error = %v", err)
	}
	if len(repo.resetCalls) != 1 || repo.resetCalls[0] != "@{u}" {
		t.Fatalf("resetCalls = %v, want [@{u}]", repo.resetCalls)
	}
	if len(repo.stashMessages) != 1 || !strings.Contains(repo.stashMessages[0], repo.backupRef) {
		t.Fatalf("stashMessages = %v, want backup ref", repo.stashMessages)
	}
	if !strings.Contains(stdout.String(), "restore: git real rescue restore "+repo.backupRef) {
		t.Fatalf("stdout = %q, want restore message", stdout.String())
	}
	if len(*notifications) != 2 || !strings.Contains((*notifications)[1], "Local commits made unreal") {
		t.Fatalf("notifications = %v, want punishment message", *notifications)
	}
}

func TestRunChallengeStashPopFailure(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{1, 1},
		backupRef:     "refs/gitreal/backups/main/1",
		stashDirty:    true,
		stashPopErr:   errors.New("conflict"),
	}
	app, stdout, _, _, _ := newTestApp(repo)

	if err := app.runChallenge(repo, 1, true); err != nil {
		t.Fatalf("runChallenge() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "stash pop failed") {
		t.Fatalf("stdout = %q, want stash pop warning", stdout.String())
	}
}

func TestRunChallengeErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		repo *fakeRepo
	}{
		{
			name: "branch error",
			repo: &fakeRepo{currentBranchErr: errors.New("detached")},
		},
		{
			name: "upstream error",
			repo: &fakeRepo{currentBranch: "main", upstreamErr: errors.New("no upstream")},
		},
		{
			name: "ahead error",
			repo: &fakeRepo{currentBranch: "main", upstream: "origin/main", aheadErr: errors.New("boom")},
		},
		{
			name: "backup error",
			repo: &fakeRepo{currentBranch: "main", upstream: "origin/main", aheadCounts: []int{1, 1}, backupErr: errors.New("boom")},
		},
		{
			name: "stash error",
			repo: &fakeRepo{currentBranch: "main", upstream: "origin/main", aheadCounts: []int{1, 1}, backupRef: "refs/gitreal/backups/main/1", stashErr: errors.New("boom")},
		},
		{
			name: "reset error",
			repo: &fakeRepo{currentBranch: "main", upstream: "origin/main", aheadCounts: []int{1, 1}, backupRef: "refs/gitreal/backups/main/1", resetErr: errors.New("boom")},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, _, _, _, _ := newTestApp(tc.repo)
			if err := app.runChallenge(tc.repo, 1, true); err == nil {
				t.Fatalf("runChallenge() error = nil, want non-nil")
			}
		})
	}
}

func TestNotifyFallback(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	app, stdout, _, _, _ := newTestApp(repo)
	app.sendNotification = func(title, message string) error {
		return errors.New("unsupported")
	}

	app.notify("GitReal", "hello")
	if !strings.Contains(stdout.String(), "notification: GitReal: hello") {
		t.Fatalf("stdout = %q, want notification fallback", stdout.String())
	}
}

func TestRescueCommands(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		rescueRefs: []string{
			"refs/gitreal/backups/main/1",
			"refs/gitreal/backups/main/2",
		},
	}
	app, stdout, stderr, _, _ := newTestApp(repo)

	if got := app.run([]string{"rescue", "list"}); got != 0 {
		t.Fatalf("rescue list exit code = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "refs/gitreal/backups/main/1") {
		t.Fatalf("stdout = %q, want rescue refs", stdout.String())
	}

	stdout.Reset()
	if got := app.run([]string{"rescue", "restore", "refs/gitreal/backups/main/1"}); got != 0 {
		t.Fatalf("rescue restore exit code = %d, want 0", got)
	}
	if len(repo.resetCalls) != 1 || repo.resetCalls[0] != "refs/gitreal/backups/main/1" {
		t.Fatalf("resetCalls = %v, want restore ref", repo.resetCalls)
	}

	stdout.Reset()
	stderr.Reset()
	if got := app.run([]string{"rescue", "restore", "refs/heads/main"}); got != 2 {
		t.Fatalf("rescue restore invalid exit code = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "ref must start with refs/gitreal/backups/") {
		t.Fatalf("stderr = %q, want backup prefix error", stderr.String())
	}
}

func TestRescueCommandErrors(t *testing.T) {
	t.Parallel()

	app, _, stderr, _, _ := newTestApp(&fakeRepo{})
	if got := app.run([]string{"rescue"}); got != 2 {
		t.Fatalf("rescue exit code = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "expected subcommand list or restore <ref>") {
		t.Fatalf("stderr = %q, want rescue usage", stderr.String())
	}

	stderr.Reset()
	if got := app.run([]string{"rescue", "wat"}); got != 2 {
		t.Fatalf("rescue unknown exit code = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand: wat") {
		t.Fatalf("stderr = %q, want unknown subcommand", stderr.String())
	}

	stderr.Reset()
	if got := app.run([]string{"rescue", "list", "extra"}); got != 2 {
		t.Fatalf("rescue list exit code = %d, want 2", got)
	}

	stderr.Reset()
	if got := app.run([]string{"rescue", "restore"}); got != 2 {
		t.Fatalf("rescue restore exit code = %d, want 2", got)
	}
}

func TestCommandStartSingleIteration(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:          "/tmp/repo",
		configBools:   map[string]bool{"gitreal.armed": false},
		configInts:    map[string]int{"gitreal.graceSeconds": 45},
		currentBranch: "main",
		upstream:      "origin/main",
		aheadCounts:   []int{0},
	}
	app, stdout, _, _, _ := newTestApp(repo)
	app.startIterations = 1

	if got := app.run([]string{"start"}); got != 0 {
		t.Fatalf("start exit code = %d, want 0", got)
	}
	if !strings.Contains(stdout.String(), "GitReal started for /tmp/repo") || !strings.Contains(stdout.String(), "next challenge:") {
		t.Fatalf("stdout = %q, want start output", stdout.String())
	}
}

func TestCommandStartHandlesChallengeError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		root:             "/tmp/repo",
		configBools:      map[string]bool{"gitreal.armed": false},
		configInts:       map[string]int{"gitreal.graceSeconds": 45},
		currentBranchErr: errors.New("detached"),
	}
	app, _, stderr, _, _ := newTestApp(repo)
	app.startIterations = 1

	if got := app.run([]string{"start"}); got != 0 {
		t.Fatalf("start exit code = %d, want 0", got)
	}
	if !strings.Contains(stderr.String(), "detached") {
		t.Fatalf("stderr = %q, want challenge error", stderr.String())
	}
}

func TestNextRandomSlot(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	base := time.Date(2026, 5, 1, 12, 15, 0, 0, time.UTC)
	slot := nextRandomSlot(base, rng)

	if !slot.After(base) {
		t.Fatalf("nextRandomSlot() = %s, want after %s", slot, base)
	}
	if slot.After(base.Add(2 * time.Hour)) {
		t.Fatalf("nextRandomSlot() = %s, want within two hours", slot)
	}

	base = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	slot = nextRandomSlot(base, rand.New(rand.NewSource(2)))
	if !slot.After(base) {
		t.Fatalf("nextRandomSlot() with hour boundary = %s, want after %s", slot, base)
	}
}

func TestCommandFailures(t *testing.T) {
	t.Parallel()

	discoverErr := errors.New("not inside a Git repository")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	app := &app{
		discoverRepo: func(path string) (repository, error) {
			return nil, discoverErr
		},
		now:              time.Now,
		sleep:            time.Sleep,
		sendNotification: func(title, message string) error { return nil },
		rng:              rand.New(rand.NewSource(1)),
		stdout:           stdout,
		stderr:           stderr,
	}

	for _, args := range [][]string{{"init"}, {"status"}, {"once"}, {"start"}, {"arm"}, {"disarm"}, {"rescue", "list"}} {
		stdout.Reset()
		stderr.Reset()
		if got := app.run(args); got != 1 {
			t.Fatalf("run(%v) = %d, want 1", args, got)
		}
		if !strings.Contains(stderr.String(), "not inside a Git repository") {
			t.Fatalf("stderr = %q, want discover error", stderr.String())
		}
	}
}

func TestConfigWriteFailures(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{setBoolErr: errors.New("boom")}
	app, _, stderr, _, _ := newTestApp(repo)
	if got := app.run([]string{"init"}); got != 1 {
		t.Fatalf("init exit code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q, want config error", stderr.String())
	}

	repo = &fakeRepo{setIntErr: errors.New("boom")}
	app, _, stderr, _, _ = newTestApp(repo)
	if got := app.commandInit(); got != 1 {
		t.Fatalf("commandInit exit code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q, want config error", stderr.String())
	}

	repo = &fakeRepo{setBoolErr: errors.New("boom")}
	app, _, stderr, _, _ = newTestApp(repo)
	if got := app.run([]string{"arm"}); got != 1 {
		t.Fatalf("arm exit code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q, want config error", stderr.String())
	}
	stderr.Reset()
	if got := app.run([]string{"disarm"}); got != 1 {
		t.Fatalf("disarm exit code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q, want config error", stderr.String())
	}
}
