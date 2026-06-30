package gh

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	github "github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

type Client struct {
	cli      atomic.Pointer[github.Client]
	token    string
	proxyURL string
}

type RepositoryRelease = github.RepositoryRelease
type ReleaseAsset = github.ReleaseAsset

// NewClient 创建 GitHub API 客户端。
// proxyURL 非空时，所有 GitHub API 请求走该代理；为空时回退到 http.DefaultTransport
// （即尊重 HTTP_PROXY/HTTPS_PROXY 环境变量）。
func NewClient(token, proxyURL string) *Client {
	c := &Client{token: token, proxyURL: proxyURL}
	c.cli.Store(buildGithubClient(token, proxyURL))
	return c
}

// SetProxy 运行时切换代理。传空字符串表示清除代理（仅使用环境变量）。
// 安全可并发：内部用 atomic.Pointer 替换底层客户端。
func (c *Client) SetProxy(proxyURL string) {
	c.proxyURL = proxyURL
	c.cli.Store(buildGithubClient(c.token, proxyURL))
}

func (c *Client) client() *github.Client {
	return c.cli.Load()
}

// buildGithubClient 构造一个带代理与（可选）token 的 github.Client。
func buildGithubClient(token, proxyURL string) *github.Client {
	var base http.RoundTripper = http.DefaultTransport
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			if t, ok := http.DefaultTransport.(*http.Transport); ok {
				cloned := t.Clone()
				cloned.Proxy = http.ProxyURL(u)
				base = cloned
			}
		}
	}
	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = &http.Client{
			Transport: &oauth2.Transport{Source: ts, Base: base},
		}
	} else {
		httpClient = &http.Client{Transport: base}
	}
	return github.NewClient(httpClient)
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
	return c.client().Repositories.GetLatestRelease(ctx, owner, repo)
}

// LatestReleaseIncludingPrerelease 获取最新的发布版本（包括 pre-release）。
// GitHub 的 GetLatestRelease API 不返回 pre-release，所以需要使用 ListReleases。
func (c *Client) LatestReleaseIncludingPrerelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
	releases, resp, err := c.client().Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{PerPage: 10})
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
	releases, resp, err := c.client().Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{PerPage: limit})
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
		releases, resp, err := c.client().Repositories.ListReleases(ctx, owner, repo, opts)
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
		tags, resp, err := c.client().Repositories.ListTags(ctx, owner, repo, opts)
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
	return c.client().Repositories.GetReleaseByTag(ctx, owner, repo, tag)
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
