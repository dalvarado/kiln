package fetcher_test

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/release"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/builder"
	. "github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/baking"
)

var _ = Describe("LocalReleaseDirectory", func() {
	var (
		localReleaseDirectory LocalReleaseDirectory
		noConfirm             bool
		releasesDir           string
		releaseFile           string
		fakeLogger            *log.Logger
	)

	BeforeEach(func() {
		var err error
		releasesDir, err = ioutil.TempDir("", "releases")
		noConfirm = true
		Expect(err).NotTo(HaveOccurred())

		releaseFile = filepath.Join(releasesDir, "some-release.tgz")

		fakeLogger = log.New(GinkgoWriter, "", 0)
		releaseManifestReader := builder.NewReleaseManifestReader(osfs.New(""))
		releasesService := baking.NewReleasesService(fakeLogger, releaseManifestReader)

		localReleaseDirectory = NewLocalReleaseDirectory(fakeLogger, releasesService)
	})

	AfterEach(func() {
		_ = os.RemoveAll(releasesDir)
	})

	Describe("GetLocalReleases", func() {
		Context("when releases exist in the releases dir", func() {
			BeforeEach(func() {
				fixtureContent, err := ioutil.ReadFile(filepath.Join("fixtures", "some-release.tgz"))
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(releaseFile, fixtureContent, 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a map of releases to locations", func() {
				releases, err := localReleaseDirectory.GetLocalReleases(releasesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(releases).To(HaveLen(1))
				Expect(releases).To(ConsistOf(
					release.Local{
						ID:        release.ID{Name: "some-release", Version: "1.2.3"},
						LocalPath: releaseFile,
						SHA1:      "6d96f7c98610fa6d8e7f45271111221b5b8497a2",
					},
				))
			})
		})

		Context("when there are no local releases", func() {
			It("returns an empty slice", func() {
				releases, err := localReleaseDirectory.GetLocalReleases(releasesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(releases).To(HaveLen(0))
			})
		})

		Context("when the releases directory does not exist", func() {
			It("returns an empty slice", func() {
				_, err := localReleaseDirectory.GetLocalReleases("some-invalid-directory")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("some-invalid-directory"))
			})
		})
	})

	Describe("DeleteExtraReleases", func() {
		var extraFilePath string
		BeforeEach(func() {
			extraFilePath = filepath.Join(releasesDir, "extra-release-0.0-os-0-0.0.0.tgz")
			err := ioutil.WriteFile(extraFilePath, []byte("abc"), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes specified files", func() {
			extraReleaseID := release.ID{Name: "extra-release", Version: "0.0"}
			extraRelease := release.Local{ID: extraReleaseID, LocalPath: extraFilePath}

			err := localReleaseDirectory.DeleteExtraReleases([]release.Local{extraRelease}, noConfirm)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(extraFilePath)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		Context("when a file cannot be removed", func() {
			It("returns an error", func() {
				extraReleaseID := release.ID{Name: "extra-release-that-cannot-be-deleted", Version: "0.0"}
				extraRelease := release.Local{ID: extraReleaseID, LocalPath: "file-does-not-exist"}

				err := localReleaseDirectory.DeleteExtraReleases([]release.Local{extraRelease}, noConfirm)
				Expect(err).To(MatchError("failed to delete release extra-release-that-cannot-be-deleted"))
			})
		})
	})
})
