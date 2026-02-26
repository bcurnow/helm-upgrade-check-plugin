// SPDX-License-Identifier: GPL-3.0-or-later
//
// Copyright 2024 The helm-upgrade-plugin authors
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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/registry"
	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.2.3", "1.2.2", true},
		{"1.2.3", "1.2.3", false},
		{"v2.0", "v1.9", true},
		{"1.0", "1.0.1", false},
		{"1.0.0", "1.0", true},
		{"1.2.3", "1.2.3.1", false},
		{"1.2.3.4", "1.2.3", true},
		{"", "1", false},
	}
	for _, c := range cases {
		t.Run(c.a+">"+c.b, func(t *testing.T) {
			got := CompareVersions(c.a, c.b)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestLoadRepoEntries(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "repos")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	repoYAML := `apiVersion: v1
repositories:
- name: foo
  url: https://example.com/charts
`
	path := filepath.Join(tmpdir, "repositories.yaml")
	if err := ioutil.WriteFile(path, []byte(repoYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	settings := cli.New()
	settings.RepositoryConfig = path

	entries, err := LoadRepoEntries(settings)
	assert.NoError(t, err)
	if assert.Len(t, entries, 1) {
		assert.Equal(t, "foo", entries[0].Name)
		assert.Equal(t, "https://example.com/charts", entries[0].URL)
	}
}

func TestChartSearcher_Search(t *testing.T) {
	repos := []*repo.Entry{
		{Name: "r1"},
		{Name: "bitnami"},
	}
	searcher := NewChartSearcher(repos, nil)

	searcher.idxCache["r1"] = &repo.IndexFile{
		Entries: map[string]repo.ChartVersions{
			"redis": repo.ChartVersions{&repo.ChartVersion{Metadata: &chart.Metadata{AppVersion: "1.0.0"}}},
		},
	}
	searcher.idxCache["bitnami"] = &repo.IndexFile{
		Entries: map[string]repo.ChartVersions{
			"redis": repo.ChartVersions{&repo.ChartVersion{Metadata: &chart.Metadata{AppVersion: "2.0.0"}}},
		},
	}

	res := searcher.Search("redis")
	assert.Equal(t, "2.0.0", res.Version)
	assert.Equal(t, []string{"r1"}, res.Repos)

	// ensure caching works
	searcher.idxCache = map[string]*repo.IndexFile{}
	res2 := searcher.Search("redis")
	assert.Equal(t, res, res2)
}

func TestChartSearcher_NoChart(t *testing.T) {
	repos := []*repo.Entry{{Name: "empty"}}
	s := NewChartSearcher(repos, nil)
	r := s.Search("nonexistent")
	assert.Empty(t, r.Repos)
	assert.Equal(t, "", r.Version)
}

// fakeOCI is a minimal implementation of the ociClient interface used in
// tests to simulate an OCI registry with two tags and a simple chart archive.
type fakeOCI struct{}

func (f *fakeOCI) Tags(ref string) ([]string, error) {
	return []string{"1.0.0", "2.0.0"}, nil
}

func (f *fakeOCI) Pull(ref string, opts ...registry.PullOption) (*registry.PullResult, error) {
	return &registry.PullResult{Chart: &registry.DescriptorPullSummaryWithMeta{
		DescriptorPullSummary: registry.DescriptorPullSummary{Data: archiveData},
	}}, nil
}

var archiveData []byte // set by test

func TestOCIRepositoryLookup(t *testing.T) {
	// build a minimal chart archive containing metadata with an appVersion
	chartYAML := "apiVersion: v2\nname: demo\nversion: 1.0.0\nappVersion: 5.5.5\n"
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gw)
	content := []byte(chartYAML)
	// create directory entry
	dirHdr := &tar.Header{Name: "demo/", Typeflag: tar.TypeDir, Mode: 0755}
	tarWriter.WriteHeader(dirHdr)
	hdr := &tar.Header{Name: "demo/Chart.yaml", Mode: 0644, Size: int64(len(content))}
	tarWriter.WriteHeader(hdr)
	tarWriter.Write(content)
	tarWriter.Close()
	gw.Close()
	archiveData = buf.Bytes()

	origFactory := registryClientFactory
	registryClientFactory = func() (ociClient, error) {
		return &fakeOCI{}, nil
	}
	defer func() { registryClientFactory = origFactory }()

	repoEntry := &repo.Entry{Name: "ociRepo", URL: "oci://example.com/charts/mychart"}
	s := NewChartSearcher([]*repo.Entry{repoEntry}, nil)

	res := s.Search("mychart")
	assert.Equal(t, "5.5.5", res.Version)
	assert.Equal(t, []string{"ociRepo"}, res.Repos)
}


func TestUpdateRepositories_Error(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "repos")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	repoYAML := `apiVersion: v1
repositories:
- name: bad
  url: http://this-does-not-exist.invalid
`
	path := filepath.Join(tmpdir, "repositories.yaml")
	if err := ioutil.WriteFile(path, []byte(repoYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := cli.New()
	settings.RepositoryConfig = path

	err = UpdateRepositories(settings)
	assert.Error(t, err)
}

func TestPrintUpgradeCommands(t *testing.T) {
	buf := &bytes.Buffer{}
	PrintUpgradeCommands(buf, "rel", "ns", "repo1", "chart", "1.2.3")
	out := buf.String()
	assert.Contains(t, out, "helm get values --namespace ns rel")
	assert.Contains(t, out, "helm upgrade --namespace ns rel repo1/chart --version 1.2.3")
}

func TestConvertReleaseList(t *testing.T) {
	helmRel := &release.Release{
		Name:      "r",
		Namespace: "n",
		Chart:     &chart.Chart{Metadata: &chart.Metadata{Version: "1.2.3", AppVersion: "1.2.3"}},
	}
	out := convertReleaseList([]*release.Release{helmRel})
	if assert.Len(t, out, 1) {
		assert.Equal(t, "r", out[0].Name)
		assert.Equal(t, "n", out[0].Namespace)
		assert.Equal(t, "-1.2.3", out[0].Chart)
		assert.Equal(t, "1.2.3", out[0].AppVersion)
	}
}

func TestFindHelpers(t *testing.T) {
	charts := []Chart{
		{Name: "foo/bar", AppVersion: "1.2"},
		{Name: "foo/baz", AppVersion: "2.3"},
	}	
	// verify free helper wrappers behave same as index methods
	repos := FindRepos("bar", charts)
	if len(repos) != 1 || repos[0] != "foo" {
		t.Errorf("FindRepos returned %v", repos)
	}
	if v := FindUpgradeVersion("baz", charts); v != "2.3" {
		t.Errorf("FindUpgradeVersion returned %s", v)
	}
}

func TestLoadIndex_FileURL(t *testing.T) {
	// host a minimal index file via HTTP so repo.NewChartRepository works
	indexYAML := `apiVersion: v1
entries:
  demo:
    - version: 0.1.0
      appVersion: 4.5.6
`
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(indexYAML))
	}))
	defer h.Close()

	repoEntry := &repo.Entry{Name: "local", URL: h.URL}
	searcher := NewChartSearcher([]*repo.Entry{repoEntry}, getter.All(cli.New()))

	idx, err := searcher.loadIndex(repoEntry)
	assert.NoError(t, err)
	if assert.NotNil(t, idx) {
		if ev := idx.Entries["demo"][0].Metadata.AppVersion; ev != "4.5.6" {
			t.Errorf("unexpected appversion %s", ev)
		}
	}
	// second call should hit cache
	idx2, err := searcher.loadIndex(repoEntry)
	assert.NoError(t, err)
	assert.Equal(t, idx, idx2)
}

func TestFetchReleases_Error(t *testing.T) {
	settings := cli.New()
	// point at an invalid kubeconfig to force Init failure
	settings.KubeConfig = "/nonexistent/invalid"
	_, err := FetchReleases(settings, false)
	assert.Error(t, err, "expected error when kubeconfig invalid")
}
