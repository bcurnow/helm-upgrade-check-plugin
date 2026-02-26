// SPDX-License-Identifier: GPL-3.0-or-later
//
// Copyright 2024 The helm-upgrade-check authors
//
// Licensed under the GNU General Public License v3.0 or later (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.gnu.org/licenses/gpl-3.0.html
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"helm-update-plugin/pkg/upgradecheck"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

type fakeSearcher struct {
	res upgradecheck.ChartSearchResult
}

func (f *fakeSearcher) Search(_ string) upgradecheck.ChartSearchResult { return f.res }

func TestMain_JSONOutput(t *testing.T) {
	// save originals
	origLoad := loadRepoEntriesFunc
	origFetch := fetchReleasesFunc
	origNew := newChartSearcherFunc
	origArgs := os.Args
	defer func() {
		loadRepoEntriesFunc = origLoad
		fetchReleasesFunc = origFetch
		newChartSearcherFunc = origNew
		os.Args = origArgs
	}()

	// stub functions
	loadRepoEntriesFunc = func(settings *cli.EnvSettings) ([]*repo.Entry, error) {
		return []*repo.Entry{}, nil
	}

	fetchReleasesFunc = func(settings *cli.EnvSettings, debug bool) ([]upgradecheck.Release, error) {
		return []upgradecheck.Release{{Name: "rel1", Namespace: "ns", Chart: "mychart-1.0", AppVersion: "1.0"}}, nil
	}

	newChartSearcherFunc = func(repos []*repo.Entry, getters getter.Providers) chartSearcher {
		return &fakeSearcher{res: upgradecheck.ChartSearchResult{Repos: []string{"testrepo"}, Version: "2.0"}}
	}

	// capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	// simulate --json flag
	os.Args = []string{"cmd", "--json"}

	// run
	main()

	// restore stdout and read
	w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)

	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\noutput: %s", err, string(out))
	}

	results, ok := parsed["results"].([]interface{})
	if !ok {
		t.Fatalf("results not present or wrong type: %v", parsed["results"])
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	res := results[0].(map[string]interface{})
	cmds, ok := res["commands"].([]interface{})
	if res["upgrade_version"] != "2.0" {
		t.Fatalf("expected upgrade_version 2.0, got %v", res["upgrade_version"])
	}
	if up, _ := res["upgradable"].(bool); !up {
		t.Fatalf("expected upgradable=true")
	}
	if !ok || len(cmds) != 3 {
		t.Fatalf("expected 3 commands in JSON, got %v", res["commands"])
	}
}

func TestMain_LoadRepoEntriesError(t *testing.T) {
	origLoad := loadRepoEntriesFunc
	origExit := exitFunc
	origArgs := os.Args
	defer func() {
		loadRepoEntriesFunc = origLoad
		exitFunc = origExit
		os.Args = origArgs
	}()

	loadRepoEntriesFunc = func(settings *cli.EnvSettings) ([]*repo.Entry, error) {
		return nil, fmt.Errorf("fail")
	}
	var code int
	exitFunc = func(c int) { code = c; panic("exit") }

	// reset flags between test invocations
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"cmd"}
	defer func() {
		r := recover()
		if r != "exit" {
			t.Fatalf("unexpected panic: %v", r)
		}
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	}()
	main()
}

func TestMain_FetchReleasesError(t *testing.T) {
	origFetch := fetchReleasesFunc
	origExit := exitFunc
	origArgs := os.Args
	defer func() {
		fetchReleasesFunc = origFetch
		exitFunc = origExit
		os.Args = origArgs
	}()

	fetchReleasesFunc = func(settings *cli.EnvSettings, debug bool) ([]upgradecheck.Release, error) {
		return nil, fmt.Errorf("no cluster")
	}
	var code int
	exitFunc = func(c int) { code = c; panic("exit") }

	// reset flags between test invocations
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"cmd"}
	defer func() {
		r := recover()
		if r != "exit" {
			t.Fatalf("unexpected panic: %v", r)
		}
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	}()
	main()
}

func TestMain_UpdateAndHumanOutput(t *testing.T) {
	origLoad := loadRepoEntriesFunc
	origFetch := fetchReleasesFunc
	origNew := newChartSearcherFunc
	origUpdate := updateRepositoriesFunc
	origArgs := os.Args
	defer func() {
		loadRepoEntriesFunc = origLoad
		fetchReleasesFunc = origFetch
		newChartSearcherFunc = origNew
		updateRepositoriesFunc = origUpdate
		os.Args = origArgs
	}()

	updated := false
	updateRepositoriesFunc = func(settings *cli.EnvSettings) error {
		updated = true
		return nil
	}

	loadRepoEntriesFunc = func(settings *cli.EnvSettings) ([]*repo.Entry, error) {
		return []*repo.Entry{}, nil
	}
	fetchReleasesFunc = func(settings *cli.EnvSettings, debug bool) ([]upgradecheck.Release, error) {
		return []upgradecheck.Release{
			{Name: "r1", Namespace: "ns", Chart: "c-1.0", AppVersion: "1.0"},
			{Name: "r2", Namespace: "ns", Chart: "d-2.0", AppVersion: "2.0"},
		}, nil
	}
	// first searcher returns upgrade info for chart c only
	newChartSearcherFunc = func(repos []*repo.Entry, getters getter.Providers) chartSearcher {
		return &fakeSearcher{res: upgradecheck.ChartSearchResult{Repos: []string{"repo"}, Version: "2.0"}}
	}

	// capture stdout
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"cmd", "--update", "--debug"}
	main()

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if !updated {
		t.Fatalf("expected updateRepositoriesFunc to be called")
	}
	if !strings.Contains(string(out), "Debug:") {
		t.Fatalf("debug message not printed")
	}
	if !strings.Contains(string(out), "Updating Helm repositories") {
		t.Fatalf("update message missing")
	}
	if !strings.Contains(string(out), "Loading repository list") {
		t.Fatalf("load message missing")
	}
	if !strings.Contains(string(out), "Fetching Helm releases") {
		t.Fatalf("fetch message missing")
	}
}
