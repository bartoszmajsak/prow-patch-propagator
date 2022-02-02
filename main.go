package main

import (
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/pkg/flagutil"
	"k8s.io/test-infra/prow/config/secret"
	prowflagutil "k8s.io/test-infra/prow/flagutil"
	configflagutil "k8s.io/test-infra/prow/flagutil/config"
	pluginsflagutil "k8s.io/test-infra/prow/flagutil/plugins"
	"k8s.io/test-infra/prow/interrupts"
	"k8s.io/test-infra/prow/logrusutil"
	"k8s.io/test-infra/prow/pjutil"
	"k8s.io/test-infra/prow/pluginhelp/externalplugins"

	"github.com/bartoszmajsak/prow-patcher/pkg/patcher"
)

type options struct {
	port                   int
	config                 configflagutil.ConfigOptions
	pluginsConfig          pluginsflagutil.PluginOptions
	kubernetes             prowflagutil.KubernetesOptions
	dryRun                 bool
	instrumentationOptions prowflagutil.InstrumentationOptions
	webhookSecretFile      string
}

func main() {
	logrusutil.ComponentInit()
	log := logrus.StandardLogger().WithField("plugin", patcher.PluginName)

	opts := gatherOptions(flag.NewFlagSet(os.Args[0], flag.ExitOnError), os.Args[1:]...)
	if err := opts.Validate(); err != nil {
		log.Fatalf("Invalid options: %v", err)
	}

	configAgent, err := opts.config.ConfigAgent()
	if err != nil {
		log.WithError(err).Fatal("Error starting config agent.")
	}

	if err = secret.Add(opts.webhookSecretFile); err != nil {
		log.WithError(err).Fatal("Error starting secrets agent.")
	}

	prowJobClient, err := opts.kubernetes.ProwJobClient(configAgent.Config().ProwJobNamespace, opts.dryRun)
	if err != nil {
		log.WithError(err).Fatal("Error getting ProwJob client for infrastructure cluster.")
	}

	serv := patcher.NewServer(secret.GetTokenGenerator(opts.webhookSecretFile), configAgent, prowJobClient, log)

	health := pjutil.NewHealthOnPort(opts.instrumentationOptions.HealthPort)
	health.ServeReady()

	mux := http.NewServeMux()
	mux.Handle("/", serv)
	externalplugins.ServeExternalPluginHelp(mux, log, patcher.HelpProvider)
	httpServer := &http.Server{Addr: ":" + strconv.Itoa(opts.port), Handler: mux}
	defer interrupts.WaitForGracefulShutdown()
	interrupts.ListenAndServe(httpServer, 5*time.Second)
}

func (o *options) Validate() error {
	for _, group := range []flagutil.OptionGroup{&o.kubernetes, &o.instrumentationOptions, &o.config, &o.pluginsConfig} {
		if err := group.Validate(o.dryRun); err != nil {
			return errors.Wrap(err, "failed validating options.")
		}
	}

	return nil
}

func gatherOptions(flagSet *flag.FlagSet, args ...string) options {
	opts := options{config: configflagutil.ConfigOptions{ConfigPath: "/etc/config/config.yaml"}}
	flagSet.IntVar(&opts.port, "port", 8888, "Port to listen on.")
	flagSet.BoolVar(&opts.dryRun, "dry-run", true, "Dry run for testing. Uses API tokens but does not mutate.")
	flagSet.StringVar(&opts.webhookSecretFile, "hmac-secret-file", "/etc/webhook/hmac", "Path to the file containing the GitHub HMAC secret.")
	opts.pluginsConfig.PluginConfigPathDefault = "/etc/plugins/plugins.yaml"
	for _, group := range []flagutil.OptionGroup{&opts.kubernetes, &opts.instrumentationOptions, &opts.config, &opts.pluginsConfig} {
		group.AddFlags(flagSet)
	}
	if err := flagSet.Parse(args); err != nil {
		logrus.WithError(err).Fatal("Error getting ProwJob client for infrastructure cluster.")
	}

	return opts
}
