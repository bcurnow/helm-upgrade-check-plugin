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
	"os"
	"strings"

	"github.com/fatih/color"
	"helm-update-plugin/pkg/upgradecheck"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

var exitFunc = os.Exit

func main() {
	var update bool
	var debug bool
	var jsonOut bool

	flag.BoolVar(&update, "update", false, "run 'helm repo update' before checking")
	flag.BoolVar(&update, "u", false, "shorthand for --update")
	flag.BoolVar(&debug, "debug", false, "enable debug output")
	flag.BoolVar(&debug, "d", false, "shorthand for --debug")
	flag.BoolVar(&jsonOut, "json", false, "output results as JSON")
	flag.BoolVar(&jsonOut, "j", false, "shorthand for --json")
	flag.Parse()

	settings := cli.New()
	if debug {
		fmt.Printf("Debug: kubeconfig=%s, namespace=%s\n", settings.KubeConfig, settings.Namespace())
	}

	if update {
		if !jsonOut {
			fmt.Print("Updating Helm repositories...")
		}
		if err := updateRepositoriesFunc(settings); err != nil {
			fmt.Fprintln(os.Stderr, "error updating helm repos:", err)
			exitFunc(1)
		}
		if !jsonOut {
			fmt.Println("done!")
		}
	}

	if !jsonOut {
		fmt.Print("Loading repository list...")
	}
	repoEntries, err := loadRepoEntriesFunc(settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading repo entries:", err)
		exitFunc(1)
	}
	if !jsonOut {
		fmt.Println("done!")
	}

	getters := getter.All(settings)
	searcher := newChartSearcherFunc(repoEntries, getters)

	if !jsonOut {
		fmt.Print("Fetching Helm releases from all namespaces...")
	}
	releases, err := fetchReleasesFunc(settings, debug)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error retrieving releases:", err)
		exitFunc(1)
	}
	if !jsonOut {
		fmt.Println("done!")
	}

	type resultItem struct {
		ChartName      string   `json:"chart_name"`
		ReleaseName    string   `json:"release_name"`
		Namespace      string   `json:"namespace"`
		CurrentVersion string   `json:"current_version"`
		UpgradeVersion string   `json:"upgrade_version"`
		Repos          []string `json:"repos"`
		Upgradable     bool     `json:"upgradable"`
		Commands       []string `json:"commands,omitempty"`
	}

	var results []resultItem
	var errors []upgradecheck.MissingChartError
	for _, rel := range releases {
		chartName := upgradecheck.ChartName(rel.Chart)
		appVersion := rel.AppVersion
		if appVersion == "" {
			appVersion = "Unknown"
		}
		info := searcher.Search(chartName)
		if len(info.Repos) == 0 {
			errors = append(errors, upgradecheck.MissingChartError{Release: rel.Name, Namespace: rel.Namespace, Chart: chartName})
			continue
		}
		upgradeVersion := info.Version
		if upgradeVersion == "" || upgradeVersion == "null" {
			upgradeVersion = "N/A"
		}
		upgradable := upgradecheck.NeedsUpgrade(appVersion, upgradeVersion)
		repoList := info.Repos

		var commands []string
		if upgradable && !jsonOut {
			// print human commands immediately when not JSON
			repoListStr := strings.Join(repoList, ",")
			upgradecheck.PrintUpgradeCommands(os.Stdout, rel.Name, rel.Namespace, repoListStr, chartName, upgradeVersion)
		}
		if upgradable && jsonOut {
			repoListStr := strings.Join(repoList, ",")
			commands = []string{
				fmt.Sprintf("helm get values --namespace %s %s -o yaml > %s.values", rel.Namespace, rel.Name, rel.Name),
				fmt.Sprintf("cat %s.values", rel.Name),
				fmt.Sprintf("helm upgrade --namespace %s %s %s/%s --version %s --values %s.values", rel.Namespace, rel.Name, repoListStr, chartName, upgradeVersion, rel.Name),
			}
		}

		results = append(results, resultItem{
			ChartName:      chartName,
			ReleaseName:    rel.Name,
			Namespace:      rel.Namespace,
			CurrentVersion: appVersion,
			UpgradeVersion: upgradeVersion,
			Repos:          repoList,
			Upgradable:     upgradable,
			Commands:       commands,
		})
	}

	if jsonOut {
		out := map[string]interface{}{
			"results":        results,
			"missing_charts": errors,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintln(os.Stderr, "error encoding JSON output:", err)
			os.Exit(1)
		}
		return
	}

	// Human output: print table and missing charts
	printFormat := "%-20s %-20s %-20s %-20s %-20s %-20s\n"
	outOfDate := color.New(color.FgBlue)
	upToDate := color.New(color.FgGreen)
	fmt.Println()
	fmt.Printf(printFormat, "Chart Name", "Release Name", "Namespace", "Current Version", "Upgrade Version", "Repo(s)")
	fmt.Printf(printFormat, "----------", "------------", "---------", "---------------", "---------------", "-------")
	for _, r := range results {
		repoListStr := strings.Join(r.Repos, ",")
		if r.Upgradable {
			outOfDate.Printf(printFormat, r.ChartName, r.ReleaseName, r.Namespace, r.CurrentVersion, r.UpgradeVersion, repoListStr)
		} else {
			upToDate.Printf(printFormat, r.ChartName, r.ReleaseName, r.Namespace, r.CurrentVersion, "Up-to-date", repoListStr)
		}
	}

	if len(errors) > 0 {
		fmt.Println("\n\nUnable to find chart information in any repo for the following releases:")
		printFormat = "%-20s %-20s %-20s\n"
		fmt.Printf(printFormat, "Release", "Namespace", "Chart")
		fmt.Printf(printFormat, "-------", "---------", "-----")
		for _, e := range errors {
			fmt.Printf(printFormat, e.Release, e.Namespace, e.Chart)
		}
	}
}

type chartSearcher interface {
	Search(string) upgradecheck.ChartSearchResult
}

var loadRepoEntriesFunc = func(settings *cli.EnvSettings) ([]*repo.Entry, error) {
	return upgradecheck.LoadRepoEntries(settings)
}

var updateRepositoriesFunc = func(settings *cli.EnvSettings) error {
	return upgradecheck.UpdateRepositories(settings)
}

var fetchReleasesFunc = func(settings *cli.EnvSettings, debug bool) ([]upgradecheck.Release, error) {
	return upgradecheck.FetchReleases(settings, debug)
}

var newChartSearcherFunc = func(repos []*repo.Entry, getters getter.Providers) chartSearcher {
	return upgradecheck.NewChartSearcher(repos, getters)
}
