package version

import (
	"net/http"

	"emperror.dev/errors"
	"github.com/google/go-github/v41/github"
	"golang.org/x/net/context"

	"github.com/bartoszmajsak/prow-patcher/version"
)

func LatestRelease() (string, error) {
	httpClient := http.Client{}
	defer httpClient.CloseIdleConnections()

	client := github.NewClient(&httpClient)
	latestRelease, _, err := client.Repositories.
		GetLatestRelease(context.Background(), "bartoszmajsak", "prow-patcher")
	if err != nil {
		return "", errors.Wrap(err, "unable to determine latest released version")
	}

	return *latestRelease.Name, nil
}

func IsLatestRelease(latestRelease string) bool {
	return latestRelease == version.Version
}
