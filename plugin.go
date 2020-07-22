package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type (
	// Config holds input parameters for the plugin
	Config struct {
		BaseURL          string
		IssueNum         int
		Title            string
		Password         string
		RepoName         string
		RepoOwner        string
		CommitSha        string
		Recreate         bool
		Username         string
		Token            string
		TerraformRootDir string
		TerraformDataDir string

		gitClient  *github.Client
		gitContext context.Context
	}

	// Plugin represents the plugin instance to be executed
	Plugin struct {
		Config    Config
		Terraform Terraform
	}
)

// Exec executes the plugin
func (p Plugin) Exec() error {
	var err error

	// setup GitHub client
	err = p.setupGitHub()
	if err != nil {
		return err
	}

	// Install specified version of terraform
	if p.Terraform.Version != "" {
		err := installTerraform(p.Terraform.Version)

		if err != nil {
			return err
		}
	}
	os.Setenv("TF_DATA_DIR", p.Config.TerraformDataDir)

	msg, err := getPlanMessage(p.Config)
	if err != nil {
		return err
	}

	ic := &github.IssueComment{
		Body: &msg,
	}

	if p.Config.IssueNum == 0 {
		p.Config.IssueNum, err = p.getPullRequestNumber(p.Config.gitContext)
		if err != nil {
			return err
		}
		if p.Config.IssueNum == 0 && err == nil {
			fmt.Println("Pull request number not found")
			return nil
		}
	}

	if p.Config.Recreate {
		_, _, err = p.Config.gitClient.Issues.CreateComment(p.Config.gitContext, p.Config.RepoOwner, p.Config.RepoName, p.Config.IssueNum, ic)
		return err
	}

	key := generateKey(p.Config)
	// Append plugin comment ID to comment message so we can search for it later
	message := fmt.Sprintf("%s\n<!-- id: %s -->\n", msg, key)
	ic.Body = &message

	comment, err := p.Comment(key)

	if err != nil {
		return err
	}

	if comment != nil {
		_, _, err = p.Config.gitClient.Issues.EditComment(p.Config.gitContext, p.Config.RepoOwner, p.Config.RepoName, int64(*comment.ID), ic)
		return err
	}

	return nil
}

func (p *Plugin) setupGitHub() error {
	err := p.validate()

	if err != nil {
		return err
	}

	err = p.initGitClient()

	if err != nil {
		return err
	}

	return nil
}

// Comment returns existing comment, nil if none exist
func (p Plugin) Comment(key string) (*github.IssueComment, error) {
	comments, err := p.allIssueComments(p.Config.gitContext)

	if err != nil {
		return nil, err
	}

	return filterComment(comments, key), nil
}

func (p Plugin) allIssueComments(ctx context.Context) ([]*github.IssueComment, error) {
	if p.Config.gitClient == nil {
		return nil, fmt.Errorf("allIssueComments(): git client not initialized")
	}

	opts := &github.IssueListCommentsOptions{}

	// get all pages of results
	var allComments []*github.IssueComment
	for {
		comments, resp, err := p.Config.gitClient.Issues.ListComments(ctx, p.Config.RepoOwner, p.Config.RepoName, p.Config.IssueNum, opts)
		if err != nil {
			return nil, err
		}
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

func (p *Plugin) initGitClient() error {
	if !strings.HasSuffix(p.Config.BaseURL, "/") {
		p.Config.BaseURL = p.Config.BaseURL + "/"
	}

	baseURL, err := url.Parse(p.Config.BaseURL)

	if err != nil {
		return fmt.Errorf("Failed to parse base URL. %s", err)
	}

	p.Config.gitContext = context.Background()

	if p.Config.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: p.Config.Token})
		tc := oauth2.NewClient(p.Config.gitContext, ts)
		p.Config.gitClient = github.NewClient(tc)
	} else {
		tp := github.BasicAuthTransport{
			Username: strings.TrimSpace(p.Config.Username),
			Password: strings.TrimSpace(p.Config.Password),
		}
		p.Config.gitClient = github.NewClient(tp.Client())
	}
	p.Config.gitClient.BaseURL = baseURL

	return nil
}

func generateKey(config Config) string {
	key := fmt.Sprintf("%s/%s/%s/%d", config.RepoOwner, config.RepoName, config.Title, config.IssueNum)
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash)
}

func filterComment(comments []*github.IssueComment, key string) *github.IssueComment {
	for _, comment := range comments {
		if strings.Contains(*comment.Body, fmt.Sprintf("<!-- id: %s -->", key)) {
			return comment
		}
	}

	return nil
}

func (p Plugin) validate() error {
	if p.Config.Token == "" && (p.Config.Username == "" || p.Config.Password == "") {
		return fmt.Errorf("You must provide an API key or Username and Password")
	}

	return nil
}

func (p Plugin) getPullRequestNumber(ctx context.Context) (int, error) {
	res, _, err := p.Config.gitClient.Search.Issues(ctx, fmt.Sprintf("%s repo:%s/%s", p.Config.CommitSha, p.Config.RepoOwner, p.Config.RepoName), nil)
	if err != nil {
		return 0, err
	}
	if len(res.Issues) == 0 {
		return 0, nil
	}

	return *res.Issues[0].Number, nil
}

func getTfoutPath() string {
	terraformDataDir := os.Getenv("TF_DATA_DIR")
	if terraformDataDir == ".terraform" || terraformDataDir == "" {
		return "plan.tfout"
	}

	return fmt.Sprintf("%s.plan.tfout", terraformDataDir)
}

func getPlanMessage(config Config) (string, error) {
	file := getTfoutPath()

	c := exec.Command(
		"terraform",
		"show",
		"-no-color",
		file,
	)
	if c.Dir == "" {
		wd, err := os.Getwd()
		if err == nil {
			c.Dir = wd
		}
	}
	if config.TerraformRootDir != "" {
		c.Dir = c.Dir + "/" + config.TerraformRootDir
	}
	out, err := c.Output()
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("## %s\n\n```\n%s```\n", config.Title, out)

	return message, nil
}
