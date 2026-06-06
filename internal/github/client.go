package gh

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	github "github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

type Client struct {
	cli *github.Client
}

type RepositoryRelease = github.RepositoryRelease
type ReleaseAsset = github.ReleaseAsset

func NewClient(token string) *Client {
	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient = oauth2.NewClient(context.Background(), ts)
	}
	return &Client{cli: github.NewClient(httpClient)}
}

// ParseOwnerRepo 从完整的 GitHub 仓库 URL 中提取所有者和仓库名。
func ParseOwnerRepo(repoURL string) (string, string, error) {
	// 预期格式：https://github.com/<owner>/<repo>
	parts := strings.Split(strings.TrimPrefix(repoURL, "https://github.com/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("无效的仓库 url，需要 https://github.com/<owner>/<repo>")
	}
	owner := parts[0]
	repo := parts[1]
	return owner, repo, nil
}

// LatestRelease 仅获取最新的发布元数据。
func (c *Client) LatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
	return c.cli.Repositories.GetLatestRelease(ctx, owner, repo)
}

// LatestReleaseIncludingPrerelease 获取最新的发布版本（包括 pre-release）。
// GitHub 的 GetLatestRelease API 不返回 pre-release，所以需要使用 ListReleases。
func (c *Client) LatestReleaseIncludingPrerelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
	releases, resp, err := c.cli.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{PerPage: 10})
	if err != nil {
		return nil, resp, err
	}
	if len(releases) == 0 {
		return nil, resp, errors.New("没有找到任何 release")
	}
	return releases[0], resp, nil
}

// ListReleases 获取指定数量的 release 列表。
func (c *Client) ListReleases(ctx context.Context, owner, repo string, limit int) ([]*github.RepositoryRelease, *github.Response, error) {
	releases, resp, err := c.cli.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{PerPage: limit})
	if err != nil {
		return nil, resp, err
	}
	if len(releases) > limit {
		releases = releases[:limit]
	}
	return releases, resp, nil
}

func (c *Client) ListReleasesByPolicy(ctx context.Context, owner, repo string, limit int, includePrerelease bool) ([]*github.RepositoryRelease, *github.Response, error) {
	if limit <= 0 {
		limit = 1
	}

	filtered := make([]*github.RepositoryRelease, 0, limit)
	opts := &github.ListOptions{PerPage: min(limit*3, 100)}
	var lastResp *github.Response

	for {
		releases, resp, err := c.cli.Repositories.ListReleases(ctx, owner, repo, opts)
		if err != nil {
			return nil, resp, err
		}
		lastResp = resp
		if len(releases) == 0 {
			break
		}

		for _, rel := range releases {
			if rel == nil {
				continue
			}
			if !includePrerelease && rel.GetPrerelease() {
				continue
			}
			filtered = append(filtered, rel)
			if len(filtered) >= limit {
				return filtered[:limit], lastResp, nil
			}
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return filtered, lastResp, nil
}

func (c *Client) ListTags(ctx context.Context, owner, repo string, limit int) ([]*github.RepositoryTag, *github.Response, error) {
	if limit <= 0 {
		limit = 1
	}

	filtered := make([]*github.RepositoryTag, 0, limit)
	opts := &github.ListOptions{PerPage: min(limit*3, 100)}
	var lastResp *github.Response

	for {
		tags, resp, err := c.cli.Repositories.ListTags(ctx, owner, repo, opts)
		if err != nil {
			return nil, resp, err
		}
		lastResp = resp
		if len(tags) == 0 {
			break
		}

		for _, tag := range tags {
			if tag == nil || tag.GetName() == "" {
				continue
			}
			filtered = append(filtered, tag)
			if len(filtered) >= limit {
				return filtered[:limit], lastResp, nil
			}
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return filtered, lastResp, nil
}

func (c *Client) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.RepositoryRelease, *github.Response, error) {
	return c.cli.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
}

// BackoffIfRateLimited 检查响应是否受到速率限制，并在需要时休眠。
func BackoffIfRateLimited(resp *github.Response) {
	if resp == nil || resp.Rate.Remaining > 0 {
		return
	}
	reset := resp.Rate.Reset.Time
	// 休眠直到重置时间 + 小段余量
	d := time.Until(reset) + 2*time.Second
	if d > 0 && d < 15*time.Minute {
		time.Sleep(d)
	}
}
