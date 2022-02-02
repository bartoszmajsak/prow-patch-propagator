package patcher

import (
	"context"

	gogh "github.com/google/go-github/v41/github"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	prowapi "k8s.io/test-infra/prow/apis/prowjobs/v1"
	v1 "k8s.io/test-infra/prow/client/clientset/versioned/typed/prowjobs/v1"
	"k8s.io/test-infra/prow/config"
)

type prowJobClient interface {
	Create(context.Context, *prowapi.ProwJob, metav1.CreateOptions) (*prowapi.ProwJob, error)
}

type server struct {
	tokenGenerator func() []byte
	configAgent    *config.Agent
	prowJobClient  prowJobClient
	log            *logrus.Entry
}

func NewServer(tokenGenerator func() []byte, configAgent *config.Agent, prowJobClient v1.ProwJobInterface, log *logrus.Entry) *server { //nolint
	return &server{
		tokenGenerator: tokenGenerator,
		configAgent:    configAgent,
		prowJobClient:  prowJobClient,
		log:            log,
	}
}

type changes struct {
	DefaultBranch *defaultBranchChange `json:"default_branch,omitempty"`
}

type defaultBranchChange struct {
	From *string `json:"from,omitempty"`
}
type repoChangeEvent struct {
	Changes *changes `json:"changes,omitempty"`
	*gogh.RepositoryEvent
}
