package bot

import (
	"errors"
	"fmt"
	"strings"

	"git.front.kjuulh.io/kjuulh/contractor/internal/models"
	"git.front.kjuulh.io/kjuulh/contractor/internal/providers"
)

type BotHandler struct {
	giteaClient  *providers.GiteaClient
	githubClient *providers.GitHubClient
}

func NewBotHandler(gitea *providers.GiteaClient, github *providers.GitHubClient) *BotHandler {
	return &BotHandler{giteaClient: gitea, githubClient: github}
}

func (b *BotHandler) Handle(input string) (output string, err error) {
	innerHandle := func(input string) (output string, err error) {
		if strings.HasPrefix(input, "help") {
			return b.Help(), nil
		}

		if strings.HasPrefix(input, "refresh") {
			return `
<h3>Contractor triggered renovate refresh on this repository</h3>
This comment will be updated with status

<!-- Status update start -->
<!-- Status update end -->
`, nil
		}

		return b.Help(), errors.New("could not recognize command")
	}

	output, err = innerHandle(input)
	output = fmt.Sprintf(
		"%s\n<small>This comment was generated by <a href='https://git.front.kjuulh.io/kjuulh/contractor'>Contractor</a></small>",
		output,
	)
	return output, err
}

func (b *BotHandler) Help() string {
	return `<details open>
	<summary><h3>/contractor [command]</h3></summary>

<strong>Commands:</strong>

* /contractor help
  *  triggers the help menu
* /contractor refresh
  *  triggers renovate to refresh the current pull request
</details>`
}

func (b *BotHandler) AppendComment(
	owner string,
	repository string,
	pullRequest int,
	comment string,
	backend models.SupportedBackend,
) (*models.AddCommentResponse, error) {
	switch backend {
	case models.SupportedBackendGitHub:
		return b.githubClient.AddComment(owner, repository, pullRequest, comment)
	case models.SupportedBackendGitea:
		return b.giteaClient.AddComment(owner, repository, pullRequest, comment)
	default:
		panic("backend chosen was not a valid option")
	}
}
