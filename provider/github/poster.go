/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/github"
	"gopkg.in/src-d/go-errors.v1"

	"github.com/sniperkit/snk.fork.lookout"
)

var (
	// ErrGitHubAPI signals an error while making a request to the GitHub API.
	ErrGitHubAPI = errors.NewKind("github api error")
	// ErrEventNotSupported signals that this provider does not support the
	// given event for a given operation.
	ErrEventNotSupported = errors.NewKind("event not supported")
)

const (
	statusTargetURL = "https://github.com/sniperkit/snk.fork.lookout"
	statusContext   = "lookout"
)

// Poster posts comments as Pull Request Reviews.
type Poster struct {
	pool *ClientPool
	conf ProviderConfig
}

var _ lookout.Poster = &Poster{}

// NewPoster creates a new poster for the GitHub API.
func NewPoster(pool *ClientPool, conf ProviderConfig) *Poster {
	return &Poster{
		pool: pool,
		conf: conf,
	}
}

// Post posts comments as a Pull Request Review.
// If the event is not a GitHub Pull Request, ErrEventNotSupported is returned.
// If a GitHub API request fails, ErrGitHubAPI is returned.
func (p *Poster) Post(ctx context.Context, e lookout.Event,
	aCommentsList []lookout.AnalyzerComments) error {
	switch ev := e.(type) {
	case *lookout.ReviewEvent:
		if ev.Provider != Provider {
			return ErrEventNotSupported.Wrap(
				fmt.Errorf("unsupported provider: %s", ev.Provider))
		}

		return p.postPR(ctx, ev, aCommentsList)
	default:
		return ErrEventNotSupported.Wrap(fmt.Errorf("unsupported event type"))
	}
}

func (p *Poster) postPR(ctx context.Context, e *lookout.ReviewEvent,
	aCommentsList []lookout.AnalyzerComments) error {

	owner, repo, pr, err := p.validatePR(e)
	if err != nil {
		return err
	}

	client, err := p.getClient(owner, repo)
	if err != nil {
		return err
	}

	// TODO: make this request lazily, only if there are comments using
	// positions.
	cc, resp, err := client.Repositories.CompareCommits(ctx, owner, repo,
		e.Base.Hash,
		e.Head.Hash)
	if err = p.handleAPIError(resp, err); err != nil {
		return err
	}

	dl := newDiffLines(cc)
	review, err := p.createReviewRequest(aCommentsList, dl)
	if err != nil {
		return err
	}

	_, resp, err = client.PullRequests.CreateReview(ctx, owner, repo, pr, review)
	if err = p.handleAPIError(resp, err); err != nil {
		return err
	}

	return nil
}

func (p *Poster) validatePR(
	e *lookout.ReviewEvent) (owner, repo string, pr int, err error) {

	base := e.Base
	owner, err = extractOwner(base)
	if err != nil {
		err = ErrEventNotSupported.Wrap(err)
		return
	}

	repo, err = extractRepo(base)
	if err != nil {
		err = ErrEventNotSupported.Wrap(err)
		return
	}

	name := e.Head.ReferenceName.String()
	if _, err = fmt.Sscanf(name, "refs/pull/%d/head", &pr); err != nil {
		err = ErrEventNotSupported.Wrap(fmt.Errorf("bad PR: %s", name))
		return
	}

	return
}

func (p *Poster) handleAPIError(resp *github.Response, err error) error {
	if err != nil {
		return ErrGitHubAPI.Wrap(err)
	}

	if resp.StatusCode == 200 {
		return nil
	}

	return ErrGitHubAPI.Wrap(fmt.Errorf("bad HTTP status: %d", resp.StatusCode))
}

func (p *Poster) addFootnote(aConf lookout.AnalyzerConfig, c *lookout.Comment) string {
	tmpl := p.conf.CommentFooter
	url := aConf.Feedback

	if tmpl == "" || url == "" {
		return c.Text
	}

	return fmt.Sprintf("%s\n\n%s", c.Text, fmt.Sprintf(tmpl, url))
}

var approveEvent = "APPROVE"

func (p *Poster) createReviewRequest(
	aCommentsList []lookout.AnalyzerComments,
	dl *diffLines) (*github.PullRequestReviewRequest, error) {
	req := &github.PullRequestReviewRequest{
		// TODO: Add CommitID of HEAD to ensure that comments are attached to
		//       the right lines.
		Event: &approveEvent,
	}

	var bodyComments []string

	for _, aComments := range aCommentsList {
		for _, c := range aComments.Comments {
			text := p.addFootnote(aComments.Config, c)

			if c.File == "" {
				bodyComments = append(bodyComments, text)
			} else if c.Line < 1 {
				line := 1
				comment := &github.DraftReviewComment{
					Path:     &c.File,
					Position: &line,
					Body:     &text,
				}
				req.Comments = append(req.Comments, comment)
			} else {
				line, err := dl.ConvertLine(c.File, int(c.Line))
				if err != nil {
					return nil, err
				}

				comment := &github.DraftReviewComment{
					Path:     &c.File,
					Position: &line,
					Body:     &text,
				}
				req.Comments = append(req.Comments, comment)
			}
		}
	}

	body := strings.Join(bodyComments, "\n\n")
	req.Body = &body

	return req, nil
}

// Status sets the Pull Request global status, visible from the GitHub UI
func (p *Poster) Status(ctx context.Context, e lookout.Event, status lookout.AnalysisStatus) error {
	switch ev := e.(type) {
	case *lookout.ReviewEvent:
		if ev.Provider != Provider {
			return ErrEventNotSupported.Wrap(
				fmt.Errorf("unsupported provider: %s", ev.Provider))
		}

		return p.statusPR(ctx, ev, status)
	default:
		return ErrEventNotSupported.Wrap(fmt.Errorf("unsupported event type"))
	}
}

// StatusCreator creates statuses on GitHub. *github.RepositoriesService
// fulfills this interface.
type StatusCreator interface {
	// CreateStatus creates a new status for a repository at the specified
	// reference. Ref can be a SHA, a branch name, or a tag name.
	CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (
		*github.RepoStatus, *github.Response, error)
}

var _ StatusCreator = &github.RepositoriesService{}

func statusStrings(s lookout.AnalysisStatus) (string, string, error) {
	switch s {
	case lookout.ErrorAnalysisStatus:
		return "error", "There was an error during the analysis", nil
	case lookout.FailureAnalysisStatus:
		return "failure", "The analysis result was negative", nil
	case lookout.PendingAnalysisStatus:
		return "pending", "The analysis is in progress", nil
	case lookout.SuccessAnalysisStatus:
		return "success", "The analysis was performed", nil
	default:
		return "", "", fmt.Errorf("unsupported AnalysisStatus %s", s)
	}
}

func (p *Poster) statusPR(ctx context.Context, e *lookout.ReviewEvent, status lookout.AnalysisStatus) error {
	owner, repo, _, err := p.validatePR(e)
	if err != nil {
		return err
	}

	statusStr, description, err := statusStrings(status)
	if err != nil {
		return err
	}
	targetURL := statusTargetURL
	context := statusContext

	repoStatus := &github.RepoStatus{
		State:       &statusStr,
		TargetURL:   &targetURL,
		Description: &description,
		Context:     &context,
	}

	client, err := p.getClient(owner, repo)
	if err != nil {
		return err
	}

	_, _, err = client.Repositories.CreateStatus(ctx, owner, repo, e.CommitRevision.Head.Hash, repoStatus)
	return err
}

func (p *Poster) getClient(username, repository string) (*Client, error) {
	client, ok := p.pool.Client(username, repository)
	if !ok {
		return nil, fmt.Errorf("client for %s/%s doesn't exists", username, repository)
	}
	return client, nil
}
