// Copyright (c) 2017-2018 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testFixesString = "Fixes"

// An environment variable value. If set is true, set it,
// else unset it (ignoring the value).
type TestEnvVal struct {
	value string
	set   bool
}

type TestCIEnvData struct {
	name              string
	env               map[string]TestEnvVal
	expectedCommit    string
	expectedSrcBranch string
	expectedDstBranch string
}

// List of variables to restore after the tests have run
var restoreSet map[string]TestEnvVal

var travisPREnv = map[string]TestEnvVal{
	"TRAVIS":                     {"true", true},
	"TRAVIS_BRANCH":              {"master", true},
	"TRAVIS_PULL_REQUEST_SHA":    {"travis-pull-request-sha", true},
	"TRAVIS_PULL_REQUEST_BRANCH": {"travis-pr", true},
}

var travisNonPREnv = map[string]TestEnvVal{
	"TRAVIS":                     {"true", true},
	"TRAVIS_BRANCH":              {"master", true},
	"TRAVIS_PULL_REQUEST_SHA":    {"travis-pull-request-sha", true},
	"TRAVIS_PULL_REQUEST_BRANCH": {"", true},
}

var testCIEnvData = []TestCIEnvData{
	{
		name:              "TravisCI PR branch",
		env:               travisPREnv,
		expectedCommit:    travisPREnv["TRAVIS_PULL_REQUEST_SHA"].value,
		expectedSrcBranch: travisPREnv["TRAVIS_PULL_REQUEST_BRANCH"].value,
		expectedDstBranch: travisPREnv["TRAVIS_BRANCH"].value,
	},
	{
		name:              "TravisCI non-PR branch",
		env:               travisNonPREnv,
		expectedCommit:    travisNonPREnv["TRAVIS_PULL_REQUEST_SHA"].value,
		expectedSrcBranch: travisNonPREnv["TRAVIS_PULL_REQUEST_BRANCH"].value,
		expectedDstBranch: travisNonPREnv["TRAVIS_BRANCH"].value,
	},
}

func init() {
	saveEnv()
}

func createCommitConfig() (config *CommitConfig) {
	return NewCommitConfig(true, true,
		testFixesString,
		"Signed-off-by",
		"",
		defaultMaxBodyLineLength,
		defaultMaxSubjectLineLength)
}

// Save the existing values of all variables that the tests will
// manipulate. These can be restored at the end of the tests by calling
// restoreEnv().
func saveEnv() {
	// Unique list of variables the tests manipulate
	varNames := make(map[string]int)
	restoreSet = make(map[string]TestEnvVal)

	for _, d := range testCIEnvData {
		for k := range d.env {
			varNames[k] = 1
		}
	}

	for key := range varNames {
		// Determine if the variable is already set
		value, set := os.LookupEnv(key)
		restoreSet[key] = TestEnvVal{value, set}
	}
}

// Apply the set of variables saved by a call to saveEnv() to the
// environment.
func restoreEnv() {
	for key, envVal := range restoreSet {
		var err error
		if envVal.set {
			err = os.Setenv(key, envVal.value)
		} else {
			err = os.Unsetenv(key)
		}

		if err != nil {
			panic(err)
		}
	}
}

// Apply a list of CI variables to the current environment. This will
// involve either setting or unsetting variables.
func setCIVariables(env map[string]TestEnvVal) (err error) {
	for key, envVal := range env {

		if envVal.set {
			err = os.Setenv(key, envVal.value)
		} else {
			err = os.Unsetenv(key)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// XXX: This function *MUST* unset all variables for all supported CI
// systems.
//
// XXX: Call saveEnv() prior to calling this function.
func clearCIVariables() {
	envVars := []string{
		"TRAVIS",
		"TRAVIS_BRANCH",
		"TRAVIS_PULL_REQUEST_SHA",
		"TRAVIS_PULL_REQUEST_BRANCH",

		"ghprbPullId",
		"ghprbActualCommit",
		"ghprbSourceBranch",
		"ghprbTargetBranch",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

// Undo the effects of setCIVariables().
func unsetCIVariables(env map[string]TestEnvVal) (err error) {
	for key := range env {
		err := os.Unsetenv(key)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestCheckCommits(t *testing.T) {
	err := checkCommits(nil, nil)
	if err == nil {
		t.Fatal("expected failure")
	}

	config := &CommitConfig{}
	err = checkCommits(config, nil)
	if err == nil {
		t.Fatalf("expected failure")
	}

	err = checkCommits(config, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	invalidCommits := []string{
		"hello",
		"foo bar",
		"what is this?",
		"don't know!",
		"9999999999999999999999999999999999999999",
		"abcdef",
		"0123456789",
		"gggggggggggggggggggggggggggggggggggggggg",
		"ggggggggggggggggggggggggggggggggggggggggh",
	}

	err = checkCommits(nil, invalidCommits)
	if err == nil {
		t.Fatalf("expected an error")
	}

	err = checkCommits(config, invalidCommits)
	if err == nil {
		t.Fatalf("expected an error")
	}

	// Simulate a Travis build on the "master" branch
	config.NeedFixes = true
	config.NeedSOBS = true
	err = checkCommits(config, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckCommitsDetails(t *testing.T) {
	assert := assert.New(t)

	ignoreSubsystem := "release"

	ignoreSubject := fmt.Sprintf("%s: this is ignored", ignoreSubsystem)

	revertSubject := `Revert "foo: bar baz"`

	makeConfigWithIgnoreSubsys := func(ignoreFixesSubsystem string) *CommitConfig {
		return NewCommitConfig(true, true,
			testFixesString,
			"Signed-off-by",
			ignoreFixesSubsystem,
			defaultMaxBodyLineLength,
			defaultMaxSubjectLineLength)
	}

	type testData struct {
		config     *CommitConfig
		commits    []Commit
		expectFail bool
	}

	makeCommits := func(subjectLine, fixesLine string) []Commit {
		return []Commit{
			{
				"",
				subjectLine,
				"",
				[]string{
					"body line 1",
					"body line 2",
					"\n",
					fixesLine,
					"\n",
					"Signed-off-by: foo@bar.com",
				}, false,
			},
		}
	}

	data := []testData{
		// A "normal" commit
		{makeConfigWithIgnoreSubsys(""), makeCommits("foo: bar baz", "Fixes #123"), false},

		// Releases don't require a Fixes comment
		{makeConfigWithIgnoreSubsys(ignoreSubsystem), makeCommits(ignoreSubject, "foo"), false},

		// Valid since there is no instance of ignoreSubsystem and the
		// commits are "well-formed".
		{makeConfigWithIgnoreSubsys(ignoreSubsystem), makeCommits("foo: bar baz", "Fixes #123"), false},

		// Fails as no "Fixes #XXX"
		{makeConfigWithIgnoreSubsys(""), makeCommits(ignoreSubject, "foo"), true},

		// Revert commit which fails as no "Fixes #XXX"
		{makeConfigWithIgnoreSubsys(""), makeCommits(revertSubject, ""), true},

		// Successful revert commit
		{makeConfigWithIgnoreSubsys(""), makeCommits(revertSubject, "Fixes #123"), false},
	}

	for _, d := range data {
		err := checkCommitsDetails(d.config, d.commits)
		if d.expectFail {
			assert.Errorf(err, "config: %+v, commits: %+v", d.config, d.commits)
			continue
		}

		assert.NoErrorf(err, "config: %+v, commits: %+v", d.config, d.commits)
	}
}

func TestCheckCommit(t *testing.T) {
	assert := assert.New(t)

	err := checkCommit(nil, nil)
	assert.Error(err, "expected error when no config specified")

	config := NewCommitConfig(true, true, "", "", "", 0, 0)
	err = checkCommit(config, nil)
	assert.Error(err, "expected error when no commit specified")

	commit := &Commit{
		hash: "HEAD",
	}

	err = checkCommit(config, commit)
	assert.Error(err, "expected error when no commit subject specified")

	commit.subject = "subsystem: a subject"
	err = checkCommit(config, commit)
	assert.Error(err, "expected error when no commit body specified")

	commit.body = []string{"hello", "world", "Signed-off-by: me@foo.com"}
	err = checkCommit(config, commit)
	assert.NoError(err)
}

func TestCheckCommitSubject(t *testing.T) {
	assert := assert.New(t)

	config := createCommitConfig()

	type testData struct {
		subject           string
		config            *CommitConfig
		expectedSubsystem string
		expectFail        bool
		expectFixes       bool
	}

	data := []testData{
		// invalid subject
		{"", nil, "", true, false},
		{"", config, "", true, false},
		{"", nil, "", true, false},
		{"", config, "", true, false},
		{"          ", config, "", true, false},
		{"\t\t\t", config, "", true, false},
		{"\n", config, "", true, false},
		{"\r", config, "", true, false},
		{"\r\n", config, "", true, false},
		{"\n\r", config, "", true, false},
		{" \n\r", config, "", true, false},
		{"\n\r ", config, "", true, false},
		{" \n\r ", config, "", true, false},
		{"invalid as no subsystem", config, "", true, false},

		{strings.Repeat("g:", (defaultMaxSubjectLineLength/2)+1), config, "", true, false},

		{"foo bar: some words", config, "foo bar", true, false},

		// valid (no fixes)
		{"subsystem: A subject", config, "subsystem", false, false},
		{"我很好: 你好", config, "我很好", false, false},
		{strings.Repeat("h:", (defaultMaxSubjectLineLength/2)-1), config, "h", false, false},
		{strings.Repeat("i:", (defaultMaxSubjectLineLength / 2)), config, "i", false, false},
		{"foo: some words", config, "foo", false, false},
		{"foo/bar: some words", config, "foo/bar", false, false},
		{"foo-bar: some words", config, "foo-bar", false, false},
		{"foo.bar: some words", config, "foo.bar", false, false},
		{"foo&bar: some words", config, "foo&bar", false, false},
		{"foo+bar: some words", config, "foo+bar", false, false},
		{"foo=bar: some words", config, "foo=bar", false, false},
		{"release: version 1.2.3-2foo", config, "release", false, false},
		{"release: version 1.2.3-2foo. fixes #212351", config, "release", false, true},

		// valid (with fixes)
		{"subsystem: A subject fixes #1", config, "subsystem", false, true},
		{"subsystem: A subject fixes # 1", config, "subsystem", false, false},
		{"subsystem: A subject fixes #11", config, "subsystem", false, true},
		{"subsystem: A subject fixes #999", config, "subsystem", false, true},
		{"subsystem: A subject fixes : #1", config, "subsystem", false, true},
		{"subsystem: A subject fixes : # 1", config, "subsystem", false, false},
		{"subsystem: A subject fixes : #11", config, "subsystem", false, true},
		{"subsystem: A subject fixes : #999", config, "subsystem", false, true},
		{"subsystem: A subject fixes: #1", config, "subsystem", false, true},
		{"subsystem: A subject fixes: # 1", config, "subsystem", false, false},
		{"subsystem: A subject fixes: #11", config, "subsystem", false, true},
		{"subsystem: A subject fixes: #999", config, "subsystem", false, true},
		{"我很好: 你好", config, "我很好", false, false},
		{"我很好: fixes #12345. 你好", config, "我很好", false, true},
		{strings.Repeat("j:", (defaultMaxSubjectLineLength/2)-1), config, "j", false, false},
		{strings.Repeat("k:", (defaultMaxSubjectLineLength / 2)), config, "k", false, false},

		// Revert commits
		{"Revert", config, "", true, false},
		{"revert", config, "", true, false},
		{"Revert ", config, "", true, false},
		{"revert ", config, "", true, false},
		{"Revert foo: bar", config, "foo", false, false},
		{"revert foo: bar", config, "foo", false, false},
		{`Revert "foo: bar`, config, "foo", false, false},
		{`Revert foo: bar"`, config, "foo", false, false},
		{`Revert "foo: bar"`, config, "foo", false, false},
		{`Revert "foo: fixes #123"`, config, "foo", false, true},
		{`revert "foo: fixes #123"`, config, "foo", false, true},
	}

	for i, d := range data {
		msg := fmt.Sprintf("test[%d]: %+v", i, d)

		if d.config != nil {
			d.config.FoundFixes = false
		}

		commit := &Commit{
			subject: d.subject,
		}

		err := checkCommitSubject(d.config, commit)
		if d.expectFail {
			assert.Errorf(err, "expected checkCommitSubject(%+v) to fail", msg)
			continue
		}

		assert.NoErrorf(err, "unexpected checkCommitSubject(%+v) failure", msg)

		assert.Equal(commit.subsystem, d.expectedSubsystem, msg)

		if d.expectFixes && !d.config.FoundFixes {
			t.Errorf("expected fixes to be found: %+v", msg)
		}
	}
}

func makeLongFixes(count int) string {
	var fixes []string

	for i := 0; i < count; i++ {
		fixes = append(fixes, fmt.Sprintf("%s #%d", testFixesString, i))
	}

	return strings.Join(fixes, ", ")
}

func TestCheckCommitBody(t *testing.T) {
	assert := assert.New(t)

	config := createCommitConfig()

	type testData struct {
		config       *CommitConfig
		body         []string
		expectFail   bool
		expectFixes  bool
		revertCommit bool
	}

	// create a string that is definitely longer than
	// the allowed line length
	lotsOfFixes := makeLongFixes(defaultMaxBodyLineLength)

	longWord := strings.Repeat("a", (7 * defaultMaxBodyLineLength))

	data := []testData{
		// invalid body
		{nil, []string{}, true, false, false},
		{nil, []string{""}, true, false, false},
		{nil, []string{" "}, true, false, false},
		{nil, []string{" ", " ", " ", " "}, true, false, false},
		{nil, []string{"\n"}, true, false, false},
		{nil, []string{"\r"}, true, false, false},
		{nil, []string{"\r\n", " "}, true, false, false},
		{nil, []string{"\r\n", "\t"}, true, false, false},

		{nil, []string{"foo"}, true, false, false},
		{config, []string{"foo"}, true, false, false},
		{nil, []string{"foo"}, true, false, false},
		{config, []string{"foo"}, true, false, false},
		{config, []string{"", "Signed-off-by: me@foo.com"}, true, false, false},
		{config, []string{" ", "Signed-off-by: me@foo.com"}, true, false, false},
		{config, []string{"Signed-off-by: me@foo.com"}, true, false, false},
		{config, []string{"Signed-off-by: me@foo.com", ""}, true, false, false},
		{config, []string{"Signed-off-by: me@foo.com", " "}, true, false, false},

		// SOB must be at the start of the line
		{config, []string{"foo", " Signed-off-by: me@foo.com"}, true, false, false},
		{config, []string{"foo", "  Signed-off-by: me@foo.com"}, true, false, false},
		{config, []string{"foo", "\tSigned-off-by: me@foo.com"}, true, false, false},
		{config, []string{"foo", " \tSigned-off-by: me@foo.com"}, true, false, false},
		{config, []string{"foo", "\t Signed-off-by: me@foo.com"}, true, false, false},
		{config, []string{"foo", " \t Signed-off-by: me@foo.com"}, true, false, false},

		// valid

		// single-word long lines should be accepted
		{config, []string{strings.Repeat("l", (defaultMaxBodyLineLength)+1), "Signed-off-by: me@foo.com"}, false, false, false},
		{config, []string{"https://this-is-a-really-really-really-reeeeally-loooooooong-and-silly-unique-resource-locator-that-nobody-should-ever-have-to-type/27706e53e877987138d758bcfcac6af623059be7/yet-another-silly-long-file-name-foo.html", "Signed-off-by: me@foo.com"}, false, false, false},
		// indented URL
		{config, []string{" https://this-is-a-really-really-really-reeeeally-loooooooong-and-silly-unique-resource-locator-that-nobody-should-ever-have-to-type/27706e53e877987138d758bcfcac6af623059be7/yet-another-silly-long-file-name-foo.html", "Signed-off-by: me@foo.com"}, false, false, false},

		// multi-word long lines should not be accepted
		{config, []string{
			fmt.Sprintf("%s %s",
				strings.Repeat("l", (defaultMaxBodyLineLength/2)+1),
				strings.Repeat("l", (defaultMaxBodyLineLength/2)+1),
			),
			"Signed-off-by: me@foo.com"}, true, false, false},

		{config, []string{"foo", "Signed-off-by: me@foo.com"}, false, false, false},
		{config, []string{"你好", "Signed-off-by: me@foo.com"}, false, false, false},

		{config, []string{"foo", "Fixes #1", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"你好", "Fixes: #1", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"你好", "Fixes  # 1", "Signed-off-by: me@foo.com"}, false, false, false},
		{config, []string{"你好", "Fixes  #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar1", "  Fixes  #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar2", "  fixes: #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar3", "	Fixes  #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar4", "	fixes  #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar5", "	fixes	#999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar6", "	Fixes:	#999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar7", "	Fixes:	 #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar8", "	Fixes:	  #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"bar9", "	Fixes: 	  #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"你好", "fixes: #999", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"你好", "fixes #19123", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"你好", "fixes #123, #234. Fixes: #3456.", "Signed-off-by: me@foo.com"}, false, true, false},
		{config, []string{"moo", lotsOfFixes, "Signed-off-by: me@foo.com"}, true, true, false},
		{config, []string{"moo", fmt.Sprintf("  %s", lotsOfFixes), "Signed-off-by: me@foo.com"}, false, true, false},

		// SOB can be any length
		{config, []string{"foo",
			fmt.Sprintf("Signed-off-by: %s@foo.com", strings.Repeat("m", defaultMaxBodyLineLength*13))},
			false, false, false},

		// Non-alphabetic lines can be any length
		{config, []string{"foo",
			fmt.Sprintf("0%s", strings.Repeat("n", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{"foo",
			fmt.Sprintf("1%s", strings.Repeat("o", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{"foo",
			fmt.Sprintf("9%s", strings.Repeat("p", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{"foo",
			fmt.Sprintf("_%s", strings.Repeat("q", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{"foo",
			fmt.Sprintf(".%s", strings.Repeat("r", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{"foo",
			fmt.Sprintf("!%s", strings.Repeat("s", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{"foo",
			fmt.Sprintf("?%s", strings.Repeat("t", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		// Indented data can be any length
		{config, []string{"foo",
			fmt.Sprintf(" %s", strings.Repeat("u", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{"foo",
			fmt.Sprintf(" %s", strings.Repeat("?", defaultMaxBodyLineLength*7)),
			"Signed-off-by: me@foo.com"},
			false, false, false},

		{config, []string{strings.Repeat("v", (defaultMaxBodyLineLength)-1), "Signed-off-by: me@foo.com"}, false, false, false},
		{config, []string{strings.Repeat("w", defaultMaxBodyLineLength), "Signed-off-by: me@foo.com"}, false, false, false},

		{config, []string{strings.Repeat("w", (7 * defaultMaxBodyLineLength)), "Signed-off-by: me@foo.com"}, false, false, false},

		// Single word lines can be any length
		{config, []string{strings.Join([]string{longWord}, " "), "Signed-off-by: me@foo.com"}, false, false, false},

		// But multi-word lines cannot
		{config, []string{strings.Join([]string{longWord, longWord}, " "), "Signed-off-by: me@foo.com"}, true, false, false},

		// However, multi line revert commits can be any length
		{config, []string{strings.Join([]string{longWord, longWord, longWord}, " "), "Signed-off-by: me@foo.com"}, false, false, true},
	}

	for _, d := range data {
		if d.config != nil {
			d.config.FoundFixes = false
		}

		commit := &Commit{
			body:         d.body,
			revertCommit: d.revertCommit,
		}

		err := checkCommitBody(d.config, commit)
		if d.expectFail {
			assert.Errorf(err, "expected checkCommitBody(%+v) to fail", d)
		} else {
			assert.NoErrorf(err, "unexpected checkCommitBody(%+v) failure", d)
		}

		if d.expectFixes && !d.config.FoundFixes {
			t.Errorf("Expected fixes to be found: %+v", d)
		}
	}
}

func TestIgnoreSrcBranch(t *testing.T) {
	type testData struct {
		commit           string
		srcBranch        string
		expected         string
		branchesToIgnore []string
	}

	data := []testData{
		{"", "", "", nil},
		{"", "", "", []string{}},
		{"commit", "", "", []string{}},
		{"commit", "", "", []string{""}},
		{"commit", "", "", []string{"", ""}},
		{"commit", "branch", "", []string{}},
		{"commit", "branch", "", []string{""}},
		{"commit", "branch", "branch", []string{"branch"}},
		{"commit", "branch", "b.*", []string{"b.*"}},
		{"commit", "branch", "^b.*h$", []string{"^b.*h$"}},
	}

	for _, d := range data {
		result := ignoreSrcBranch(d.commit, d.srcBranch, d.branchesToIgnore)
		if result != d.expected {
			t.Fatalf("Unexpected ignoreSrcBranch return value %v (params %+v)", result, d)
		}
	}
}

func TestDetectCIEnvironment(t *testing.T) {
	for _, d := range testCIEnvData {
		err := setCIVariables(d.env)
		if err != nil {
			t.Fatal(err)
		}

		commit, dstBranch, srcBranch := detectCIEnvironment()

		if commit != d.expectedCommit {
			t.Fatalf("Unexpected commit %v (%q: %+v)", commit, d.name, d)
		}

		if dstBranch != d.expectedDstBranch {
			t.Fatalf("Unexpected destination branch %v (%q: %+v)", dstBranch, d.name, d)
		}

		if srcBranch != d.expectedSrcBranch {
			t.Fatalf("Unexpected source branch %v (%q: %+v)", srcBranch, d.name, d)
		}

		// Crudely undo the changes (it'll be fully undone later
		// using restoreEnv() but this is required to avoid
		// tests interfering with one another).
		err = unsetCIVariables(d.env)
		if err != nil {
			t.Fatal(err)
		}
	}

	restoreEnv()
}

func TestGetCommitAndBranch(t *testing.T) {
	clearCIVariables()

	type testData struct {
		expectedCommit      string
		expectedBranch      string
		args                []string
		srcBranchesToIgnore []string
		expectFail          bool
	}

	data := []testData{
		{"", "", nil, nil, true},
		{"", "", []string{}, nil, true},
		{"", "", nil, []string{}, true},
		{"HEAD", "main", []string{}, []string{}, false},
		{"commit", "main", []string{"commit"}, []string{}, false},
		{"commit", "branch", []string{"commit", "branch"}, []string{}, false},
		{"commit", "branch", []string{"too", "many", "args"}, []string{}, true},
	}

	for _, d := range data {
		commit, branch, err := getCommitAndBranch(d.args, d.srcBranchesToIgnore)

		if d.expectFail {
			if err == nil {
				t.Fatalf("Unexpected success: %+v", d)
			}
		} else {
			if err != nil {
				t.Fatalf("Unexpected failure: %+v: %v", d, err)
			}
		}

		if d.expectFail {
			continue
		}

		if commit != d.expectedCommit {
			t.Fatalf("Unexpected commit %v (%+v)", commit, d)
		}

		if branch != d.expectedBranch {
			t.Fatalf("Expected branch %v, got %v", d.expectedBranch, branch)
		}
	}

	// Now deal with CI auto-detection
	for _, d := range testCIEnvData {
		err := setCIVariables(d.env)
		if err != nil {
			t.Fatal(err)
		}

		// XXX: crucially, no arguments (to trigger the auto-detection)
		commit, dstBranch, err := getCommitAndBranch([]string{}, []string{})
		if err != nil {
			t.Fatal(err)
		}

		if commit != d.expectedCommit {
			t.Fatalf("Unexpected commit %v (%+v)", commit, d)
		}

		if dstBranch != d.expectedDstBranch {
			t.Fatalf("Unexpected destination branch %v (%+v)", dstBranch, d)
		}

		// Crudely undo the changes (it'll be fully undone later
		// using restoreEnv() but this is required to avoid
		// tests interfering with one another).
		err = unsetCIVariables(d.env)
		if err != nil {
			t.Fatal(err)
		}
	}

	restoreEnv()
}
