package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/google/go-github/github"
	"github.com/robertstettner/drone-terraform-github-commenter/parser"
	"golang.org/x/oauth2"
)

type (
	// Config holds input parameters for the plugin
	Config struct {
		BaseURL          string
		IssueNum         int
		Title            string
		Mode             string
		Password         string
		RepoName         string
		RepoOwner        string
		CommitSha        string
		Recreate         bool
		Username         string
		Token            string
		InitOptions      InitOptions
		Cacert           string
		Debug            bool
		RoleARN          string
		TerraformRootDir string
		TerraformDataDir string

		gitClient  *github.Client
		gitContext context.Context
	}

	// InitOptions include options for the Terraform's init command
	InitOptions struct {
		BackendConfig []string `json:"backend-config"`
		Lock          *bool    `json:"lock"`
		LockTimeout   string   `json:"lock-timeout"`
	}

	// Plugin represents the plugin instance to be executed
	Plugin struct {
		Config    Config
		Netrc     Netrc
		Terraform Terraform
	}

	// Netrc is credentials for cloning
	Netrc struct {
		Machine  string
		Login    string
		Password string
	}
)

// RunCommand executes arbitrary commands to be run in the Terraform root dir
func (p Plugin) RunCommand(c *exec.Cmd, stdout io.Writer, stderr io.Writer) error {
	if c.Dir == "" {
		wd, err := os.Getwd()
		if err == nil {
			c.Dir = wd
		}
	}
	if p.Config.TerraformRootDir != "" {
		c.Dir = c.Dir + "/" + p.Config.TerraformRootDir
	}
	c.Stdout = stdout
	c.Stderr = stderr
	if p.Config.Debug {
		trace(c)
	}

	return c.Run()
}

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

	if p.Config.RoleARN != "" {
		assumeRole(p.Config.RoleARN)
	}

	// writing the .netrc file with Github credentials in it.
	err = writeNetrc(p.Netrc.Machine, p.Netrc.Login, p.Netrc.Password)
	if err != nil {
		return err
	}

	os.Setenv("TF_DATA_DIR", p.Config.TerraformDataDir)

	var commands []*exec.Cmd

	commands = append(commands, exec.Command("terraform", "version"))

	if p.Config.Cacert != "" {
		commands = append(commands, installCaCert(p.Config.Cacert))
	}

	commands = append(commands, deleteCache(p.Config.TerraformDataDir))
	commands = append(commands, initCommand(p.Config.InitOptions))
	commands = append(commands, getModules())

	for _, c := range commands {
		var stdout io.Writer = os.Stdout
		if p.Config.Debug {
			stdout = ioutil.Discard
		}
		err := p.RunCommand(c, stdout, os.Stderr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Fatal("Failed to execute a command")
		}
		logrus.Debug("Command completed successfully")
	}

	msg, err := p.getPlanOutput()
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
			logrus.Info("Pull request number not found")
			return nil
		}
	}

	if p.Config.Recreate {
		_, _, err = p.Config.gitClient.Issues.CreateComment(p.Config.gitContext, p.Config.RepoOwner, p.Config.RepoName, p.Config.IssueNum, ic)
		logrus.Info("Created comment in PR")
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
		logrus.Info("Updated comment in PR")
	} else {
		_, _, err = p.Config.gitClient.Issues.CreateComment(p.Config.gitContext, p.Config.RepoOwner, p.Config.RepoName, p.Config.IssueNum, ic)
		logrus.Info("Created comment in PR")
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

func (p Plugin) getPlanOutput() (string, error) {
	var out bytes.Buffer

	file := getTfoutPath()

	c := exec.Command(
		"terraform",
		"show",
		"-no-color",
		file,
	)
	err := p.RunCommand(c, &out, os.Stderr)
	if err != nil {
		return "", err
	}

	opts := &parser.Parser{
		Message: out.String(),
		Mode:    p.Config.Mode,
	}
	msg, err := parser.Parse(opts)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("## %s\n\n```diff\n%s```\n", p.Config.Title, msg)

	return message, nil
}

func assumeRole(roleArn string) {
	client := sts.New(session.New())
	duration := time.Hour * 1
	stsProvider := &stscreds.AssumeRoleProvider{
		Client:          client,
		Duration:        duration,
		RoleARN:         roleArn,
		RoleSessionName: "drone",
	}

	value, err := credentials.NewCredentials(stsProvider).Get()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("Error assuming role!")
	}
	os.Setenv("AWS_ACCESS_KEY_ID", value.AccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", value.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", value.SessionToken)
}

func deleteCache(terraformDataDir string) *exec.Cmd {
	return exec.Command(
		"rm",
		"-rf",
		terraformDataDir,
	)
}

func getModules() *exec.Cmd {
	return exec.Command(
		"terraform",
		"get",
	)
}

func initCommand(config InitOptions) *exec.Cmd {
	args := []string{
		"init",
	}

	for _, v := range config.BackendConfig {
		args = append(args, fmt.Sprintf("-backend-config=%s", v))
	}

	// True is default in TF
	if config.Lock != nil {
		args = append(args, fmt.Sprintf("-lock=%t", *config.Lock))
	}

	// "0s" is default in TF
	if config.LockTimeout != "" {
		args = append(args, fmt.Sprintf("-lock-timeout=%s", config.LockTimeout))
	}

	// Fail Terraform execution on prompt
	args = append(args, "-input=false")

	return exec.Command(
		"terraform",
		args...,
	)
}

func installCaCert(cacert string) *exec.Cmd {
	ioutil.WriteFile("/usr/local/share/ca-certificates/ca_cert.crt", []byte(cacert), 0644)
	return exec.Command(
		"update-ca-certificates",
	)
}

func trace(cmd *exec.Cmd) {
	fmt.Println("$", strings.Join(cmd.Args, " "))
}

// helper function to write a netrc file.
// The following code comes from the official Git plugin for Drone:
// https://github.com/drone-plugins/drone-git/blob/8386effd2fe8c8695cf979427f8e1762bd805192/utils.go#L43-L68
func writeNetrc(machine, login, password string) error {
	if machine == "" {
		return nil
	}
	out := fmt.Sprintf(
		netrcFile,
		machine,
		login,
		password,
	)

	home := "/root"
	u, err := user.Current()
	if err == nil {
		home = u.HomeDir
	}
	path := filepath.Join(home, ".netrc")
	return ioutil.WriteFile(path, []byte(out), 0600)
}

const netrcFile = `
machine %s
login %s
password %s
`
