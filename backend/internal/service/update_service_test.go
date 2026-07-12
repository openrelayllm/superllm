//go:build unit

package service

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release *GitHubRelease
	repo    string
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	s.repo = repo
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	t.Setenv("SUB2API_ADMIN_PLUS_BINARY_SELF_UPDATE", "true")

	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func TestUpdateServiceCheckUpdateUsesAdminPlusReleaseRepoAndArchiveName(t *testing.T) {
	t.Setenv("SUB2API_ADMIN_PLUS_BINARY_SELF_UPDATE", "true")

	client := &updateServiceGitHubClientStub{
		release: &GitHubRelease{
			TagName: "v0.41.0",
			Name:    "v0.41.0",
			Assets: []GitHubAsset{
				{
					Name:               "superllm_0.41.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz",
					BrowserDownloadURL: "https://github.com/openrelayllm/superllm/releases/download/v0.41.0/superllm_0.41.0_linux_amd64.tar.gz",
					Size:               1024,
				},
			},
		},
	}
	svc := NewUpdateService(&updateServiceCacheStub{}, client, "0.40.0", "release")

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err)
	require.Equal(t, githubRepo, client.repo)
	require.Equal(t, "release", info.BuildType)
	require.True(t, info.HasUpdate)
	require.Equal(t, "superllm_0.41.0_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz", svc.getArchiveName(info.LatestVersion))
}

func TestUpdateServiceContainerDeploymentDisablesBinarySelfUpdate(t *testing.T) {
	t.Setenv("SUB2API_ADMIN_PLUS_BINARY_SELF_UPDATE", "false")
	t.Setenv("RAILWAY_ENVIRONMENT", "production")

	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.41.0",
				Name:    "v0.41.0",
			},
		},
		"0.40.0",
		"release",
	)

	err := svc.PerformUpdate(context.Background())
	require.ErrorIs(t, err, ErrContainerUpdateUnsupported)

	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, "container", info.BuildType)
	require.Contains(t, info.Warning, "Container deployment detected")
}
