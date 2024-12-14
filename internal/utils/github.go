package utils

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"release-candidate/internal/configs"
	"strconv"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v66/github"
)

func CreateGitHubClient(variables *configs.Config) (*github.Client, error) {
	if variables.AppID != "" && variables.PrivateKey != "" && variables.InstallationID != "" {
		log.Println("Using GitHub App authentication")
		appIDInt, err := strconv.ParseInt(variables.AppID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting appID: %v", err)
		}
		installationIDInt, err := strconv.ParseInt(variables.InstallationID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting installationID: %v", err)
		}
		itr, err := ghinstallation.New(http.DefaultTransport, appIDInt, installationIDInt, []byte(variables.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("error creating GitHub installation transport: %v", err)
		}
		return github.NewClient(&http.Client{Transport: itr}), nil
	} else if variables.Token != "" {
		log.Println("Using personal access token authentication")
		return github.NewClient(nil).WithAuthToken(variables.Token), nil
	} else {
		return nil, fmt.Errorf("no authentication method provided")
	}
}

// RcValidate validates the release candidate version format.
func RcVersionValidate(l LogInterface, rcVersion string) error {
	if rcVersion == "" {
		l.Fatal("rcVersion is required")
		return fmt.Errorf("rcVersion is required")
	}
	if match, _ := regexp.MatchString(`^v\d+\.\d+\.\d+$`, rcVersion); !match {
		l.Fatal("rcVersion should be in the format v*.*.*")
		return fmt.Errorf("rcVersion should be in the format v*.*.*")
	}
	l.Info("Validated rcVersion %s", rcVersion)
	return nil
}
