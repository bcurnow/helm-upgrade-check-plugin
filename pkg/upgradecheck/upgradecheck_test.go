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

package upgradecheck

import (
    "testing"
)

func TestChartName(t *testing.T) {
    cases := map[string]string{
        "redis-14.8.8":     "redis",
        "nginx-1.2.3":      "nginx",
        "simple":           "simple",
        "complex-name-0.1": "complex-name",
    }
    for in, want := range cases {
        if got := ChartName(in); got != want {
            t.Errorf("ChartName(%q)=%q, want %q", in, got, want)
        }
    }
}

func TestFindRepos(t *testing.T) {
    charts := []Chart{
        {Name: "bitnami/redis", AppVersion: "6.0.1"},
        {Name: "stable/redis", AppVersion: "6.0.0"},
        {Name: "bitnami/nginx", AppVersion: "1.2.3"},
    }

    idx := NewChartIndex(charts)

    // redis exists in bitnami and stable; bitnami should be removed
    repos := idx.Repos("redis")
    if len(repos) != 1 || repos[0] != "stable" {
        t.Errorf("expected single repo 'stable', got %v", repos)
    }

    // nginx only in bitnami
    repos = idx.Repos("nginx")
    if len(repos) != 1 || repos[0] != "bitnami" {
        t.Errorf("expected single repo 'bitnami', got %v", repos)
    }

    // nonexistent chart
    repos = idx.Repos("doesnotexist")
    if len(repos) != 0 {
        t.Errorf("expected no repos, got %v", repos)
    }
}

func TestFindUpgradeVersion(t *testing.T) {
    charts := []Chart{
        {Name: "bitnami/redis", AppVersion: "6.0.1"},
        {Name: "stable/redis", AppVersion: "6.0.0"},
        {Name: "bitnami/nginx", AppVersion: "1.2.3"},
    }

    idx := NewChartIndex(charts)

    if v := idx.UpgradeVersion("redis"); v != "6.0.0" {
        t.Errorf("expected 6.0.0, got %s", v)
    }
    if v := idx.UpgradeVersion("nginx"); v != "1.2.3" {
        t.Errorf("expected 1.2.3, got %s", v)
    }
    if v := idx.UpgradeVersion("none"); v != "" {
        t.Errorf("expected empty string, got %s", v)
    }
}

func TestNeedsUpgrade(t *testing.T) {
    cases := []struct{
        cur string
        up  string
        want bool
    }{
        {"1.2.3", "1.2.3", false},
        {"v1.2.3", "1.2.3", false},
        {"1.2.3", "v1.2.4", true},
        {"", "1.2.3", false},
        {"1.2.3", "", false},
        {"1.2.3", "null", false},
    }
    for _, c := range cases {
        if got := NeedsUpgrade(c.cur, c.up); got != c.want {
            t.Errorf("NeedsUpgrade(%q,%q)=%v, want %v", c.cur, c.up, got, c.want)
        }
    }
}
