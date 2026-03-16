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
