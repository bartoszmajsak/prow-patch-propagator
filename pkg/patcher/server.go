package patcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	prowapi "k8s.io/test-infra/prow/apis/prowjobs/v1"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/pjutil"
	"k8s.io/test-infra/prow/pluginhelp"
)

const PluginName = "patch-propagator"

func HelpProvider(_ []config.OrgRepo) (*pluginhelp.PluginHelp, error) {
	pluginHelp := &pluginhelp.PluginHelp{
		Description: `The patch-propagator plugin is used for carrying over patchset from previous development stream to newly created one.`,
	}

	return pluginHelp, nil
}

func (s *server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	eventType, eventGUID, payload, ok, _ := github.ValidateWebhook(response, request, s.tokenGenerator)
	if !ok {
		s.log.Info(eventGUID, eventType)

		return
	}

	fmt.Fprint(response, "Event received. Have a nice day.")
	s.log.Infof("received %s", eventType)

	if err := s.handleEvent(eventType, eventGUID, payload); err != nil {
		logrus.WithError(err).Error("Error parsing event.")
	}
}

func (s *server) handleEvent(eventType, eventGUID string, payload []byte) error {
	log := logrus.WithFields(
		logrus.Fields{
			"event-type":     eventType,
			github.EventGUID: eventGUID,
		},
	)

	switch eventType {
	case "repository":
		var repoEvent repoChangeEvent
		if err := json.Unmarshal(payload, &repoEvent); err != nil {
			return errors.Wrap(err, "failed unmarshalling event")
		}
		if repoEvent.Changes == nil {
			log.Infof("unhandled repo event")

			break
		}
		postsubmits := s.configAgent.Config().PostsubmitsStatic[*repoEvent.Repo.FullName]
		for i := range postsubmits {
			job := postsubmits[i]
			if job.Labels[s.jobSelectionLabel] != "true" {
				continue
			}
			log.Infof("Starting %s build.", job.Name)

			jobSpec := pjutil.PostsubmitSpec(job, createRef(repoEvent))
			prowJob := pjutil.NewProwJob(jobSpec, map[string]string{}, map[string]string{})
			if err := createWithRetry(context.TODO(), s.prowJobClient, &prowJob); err != nil {
				log.WithError(err).Error("Failed to create prowjob.")
			}
		}
	default:
		log.Debugf("skipping unhandled event of type %q", eventType)
	}

	return nil
}

func createRef(event repoChangeEvent) prowapi.Refs {
	orgRepo := strings.Split(*event.Repo.FullName, "/")

	return prowapi.Refs{
		Org:     orgRepo[0],
		Repo:    orgRepo[1],
		BaseRef: fmt.Sprintf("%s:%s", *event.Changes.DefaultBranch.From, *event.Repo.DefaultBranch),
	}
}

// createWithRetry will retry the creation of a ProwJob. The Name must be set, otherwise we might end up creating it multiple times
// if one Create request errors but succeeds under the hood.
func createWithRetry(ctx context.Context, client prowJobClient, job *prowapi.ProwJob, millisecondOverride ...time.Duration) error {
	millisecond := time.Millisecond
	if len(millisecondOverride) == 1 {
		millisecond = millisecondOverride[0]
	}

	var errs []error
	if err := wait.ExponentialBackoff(wait.Backoff{Duration: 250 * millisecond, Factor: 2.0, Jitter: 0.1, Steps: 8}, func() (bool, error) {
		if _, err := client.Create(ctx, job, metav1.CreateOptions{}); err != nil {
			// Can happen if a previous request was successful but returned an error
			if apierrors.IsAlreadyExists(err) {
				return true, nil
			}
			// Store and swallow errors, if we end up timing out we will return all of them
			errs = append(errs, err)

			return false, nil
		}

		return true, nil
	}); err != nil {
		if !errors.Is(err, wait.ErrWaitTimeout) {
			return errors.Wrap(err, "failed retrying to create prow job.")
		}

		return utilerrors.NewAggregate(errs)
	}

	return nil
}
