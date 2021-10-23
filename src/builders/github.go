package builders

import (
	"SignTools/src/util"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/google/go-github/v33/github"
	"github.com/pkg/errors"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/oauth2"
)

type GitHubData struct {
	Enable           bool   `yaml:"enable"`
	RepoName         string `yaml:"repo_name"`
	OrgName          string `yaml:"org_name"`
	WorkflowFileName string `yaml:"workflow_file_name"`
	Token            string `yaml:"token"`
	Ref              string `yaml:"ref"`
}

func MakeGitHub(data *GitHubData) *GitHub {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: data.Token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return &GitHub{
		data:   data,
		client: client,
		ctx:    ctx,
	}
}

type GitHub struct {
	data   *GitHubData
	client *github.Client
	ctx    context.Context
}

func (g *GitHub) Trigger() error {
	body := github.CreateWorkflowDispatchEventRequest{
		Ref:    g.data.Ref,
		Inputs: nil,
	}
	response, err := g.client.Actions.CreateWorkflowDispatchEventByFileName(g.ctx, g.data.OrgName, g.data.RepoName, g.data.WorkflowFileName, body)
	if err != nil {
		return err
	}
	return util.Check2xxCode(response.StatusCode)
}

func (g *GitHub) GetStatusUrl() (string, error) {
	return fmt.Sprintf("https://github.com/%s/%s/actions/workflows/%s", g.data.OrgName, g.data.RepoName, g.data.WorkflowFileName), nil
}

func (g *GitHub) SetSecrets(secrets map[string]string) error {
	keyResp, response, err := g.client.Actions.GetRepoPublicKey(g.ctx, g.data.OrgName, g.data.RepoName)
	if err != nil {
		return errors.WithMessage(err, "get repo public key")
	}
	if err := util.Check2xxCode(response.StatusCode); err != nil {
		return errors.WithMessage(err, "get repo public key")
	}
	keyBytes, err := base64.StdEncoding.DecodeString(keyResp.GetKey())
	if err != nil {
		return errors.WithMessage(err, "decode repo public key")
	}
	var recipient [32]byte
	copy(recipient[:], keyBytes[:32])
	for secretName, secretVal := range secrets {
		sealedSecret, err := box.SealAnonymous(nil, []byte(secretVal), &recipient, rand.Reader)
		if err != nil {
			return errors.WithMessage(err, "seal secret: "+secretName)
		}
		body := github.EncryptedSecret{
			Name:           secretName,
			KeyID:          keyResp.GetKeyID(),
			EncryptedValue: base64.StdEncoding.EncodeToString(sealedSecret),
		}
		response, err := g.client.Actions.CreateOrUpdateRepoSecret(g.ctx, g.data.OrgName, g.data.RepoName, &body)
		if err != nil {
			return errors.WithMessage(err, "create or update repo secret")
		}
		if err := util.Check2xxCode(response.StatusCode); err != nil {
			return errors.WithMessage(err, "create or update repo secret")
		}
	}
	return nil
}
