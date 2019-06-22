package fetcher

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/kiln/internal/cargo"
)

var repos = []string{
	"cloudfoundry",
	"pivotal-cf",
	"cloudfoundry-incubator",
	"pivotal-cf-experimental",
	"bosh-packages",
	"cppforlife",
	"vito",
	"flavorjones",
	"xoebus",
	"dpb587",
	"jamlo",
	"concourse",
	"cf-platform-eng",
	"starkandwayne",
	"cloudfoundry-community",
	"vmware",
	"DataDog",
	"Dynatrace",
	"SAP",
	"hybris",
	"minio",
	"rakutentech",
	"frodenas",
}

var suffixes = []string{
	"-release",
	"-boshrelease",
	"-bosh-release",
	"",
}

func ReleaseExistOnBoshio(name string) bool {
	resp, err := http.Get("https://bosh.io/api/v1/releases/github.com/" + name)
	if err != nil {
		fmt.Errorf("Bosh.io API is down with error: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if string(body) == "null" {
		return false
	} else {
		return true
	}
}

type BOSHIOReleaseSource struct {
	logger *log.Logger
}

func NewBOSHIOReleaseSource(logger *log.Logger) *BOSHIOReleaseSource {
	return &BOSHIOReleaseSource{logger}
}

func (r *BOSHIOReleaseSource) Configure(assets cargo.Assets) {
	return
}

func (r BOSHIOReleaseSource) GetMatchedReleases(desiredReleaseSet cargo.CompiledReleaseSet) (cargo.CompiledReleaseSet, error) {
	matchedBOSHIOReleases := make(cargo.CompiledReleaseSet)

	for compRelease := range desiredReleaseSet {
	found:
		for _, repo := range repos {
			for _, suf := range suffixes {
				fullName := repo + "/" + compRelease.Name + suf
				exists := ReleaseExistOnBoshio(fullName)
				if exists {
					downloadURL := "https://bosh.io/d/github.com/" + fullName + "?v=" + compRelease.Version
					matchedBOSHIOReleases[compRelease] = downloadURL
					break found
				}
			}
		}
	}

	return matchedBOSHIOReleases, nil //no foreseen error to return to a higher level
}

func (r BOSHIOReleaseSource) DownloadReleases(releaseDir string, matchedBOSHObjects cargo.CompiledReleaseSet, downloadThreads int) error {
	r.logger.Printf("downloading %d objects from bosh.io...", len(matchedBOSHObjects))

	for _, downloadURL := range matchedBOSHObjects {

		r.logger.Printf("downloading %s...\n", downloadURL)
		// Get the data
		resp, err := http.Get(downloadURL)
		if err != nil {
			return err
		}

		// Create the file
		fileName := strings.Split(resp.Header["Content-Disposition"][0], "=")[1]
		out, err := os.Create(filepath.Join(releaseDir, fileName))
		if err != nil {
			return err
		}

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}