package commands_test

import (
	"errors"
	"fmt"
	"github.com/onsi/gomega/gbytes"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	fetcherFakes "github.com/pivotal-cf/kiln/fetcher/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"github.com/pivotal-cf/kiln/release"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"
)

var _ = Describe("UpdateRelease", func() {
	const (
		releaseName          = "capi"
		oldReleaseVersion    = "1.8.0"
		newReleaseVersion    = "1.8.7"
		oldRemotePath        = "https://bosh.io/releases/some-release"
		newRemotePath        = "some/s3/path"
		oldReleaseSourceName = "bosh.io"
		newReleaseSourceName = "final-pcf-bosh-releases"
		oldReleaseSha1       = "old-sha1"
		newReleaseSha1       = "new-sha1"

		releasesDir = "releases"
	)

	var (
		updateReleaseCommand       UpdateRelease
		filesystem                 billy.Filesystem
		multiReleaseSourceProvider *fakes.MultiReleaseSourceProvider
		releaseSource              *fetcherFakes.MultiReleaseSource
		logger                     *log.Logger
		downloadedReleasePath      string
		expectedDownloadedRelease  release.Local
		expectedRemoteRelease      release.Remote
		kilnFileLoader             *fakes.KilnfileLoader
	)

	Context("Execute", func() {
		BeforeEach(func() {
			kilnFileLoader = new(fakes.KilnfileLoader)
			releaseSource = new(fetcherFakes.MultiReleaseSource)
			multiReleaseSourceProvider = new(fakes.MultiReleaseSourceProvider)
			multiReleaseSourceProvider.Returns(releaseSource)

			filesystem = osfs.New("/tmp/")

			kilnfile := cargo.Kilnfile{}

			kilnFileLock := cargo.KilnfileLock{
				Releases: []cargo.ReleaseLock{
					{
						Name:         "minecraft",
						SHA1:         "developersdevelopersdevelopersdevelopers",
						Version:      "2.0.1",
						RemoteSource: "bosh.io",
						RemotePath:   "not-used",
					},
					{
						Name:         releaseName,
						SHA1:         oldReleaseSha1,
						Version:      oldReleaseVersion,
						RemoteSource: oldReleaseSourceName,
						RemotePath:   oldRemotePath,
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "some-os",
					Version: "4.5.6",
				},
			}

			kilnFileLoader.LoadKilnfilesReturns(kilnfile, kilnFileLock, nil)
			logger = log.New(GinkgoWriter, "", 0)

			err := filesystem.MkdirAll(releasesDir, os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			downloadedReleasePath = filepath.Join(releasesDir, fmt.Sprintf("%s-%s.tgz", releaseName, newReleaseVersion))
			expectedDownloadedRelease = release.Local{
				ID:        release.ID{Name: releaseName, Version: newReleaseVersion},
				LocalPath: downloadedReleasePath,
				SHA1:      newReleaseSha1,
			}
			expectedRemoteRelease = release.Remote{
				ID:         expectedDownloadedRelease.ID,
				RemotePath: newRemotePath,
				SourceID:   newReleaseSourceName,
			}

			releaseSource.GetMatchedReleaseReturns(expectedRemoteRelease, true, nil)
			releaseSource.DownloadReleaseReturns(expectedDownloadedRelease, nil)
		})

		JustBeforeEach(func() {
			updateReleaseCommand = NewUpdateRelease(logger, filesystem, multiReleaseSourceProvider.Spy, kilnFileLoader)
		})

		When("updating to a version that exists in the remote", func() {
			It("downloads the release", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
					"--variable", "someKey=someValue",
					"--variables-file", "thisisafile",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(1))

				receivedReleaseRequirement := releaseSource.GetMatchedReleaseArgsForCall(0)
				releaseRequirement := release.Requirement{
					Name:            releaseName,
					Version:         newReleaseVersion,
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}
				Expect(receivedReleaseRequirement).To(Equal(releaseRequirement))

				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(1))

				receivedReleasesDir, receivedRemoteRelease, _ := releaseSource.DownloadReleaseArgsForCall(0)
				Expect(receivedReleasesDir).To(Equal(releasesDir))
				Expect(receivedRemoteRelease).To(Equal(expectedRemoteRelease))

				Expect(kilnFileLoader.LoadKilnfilesCallCount()).To(Equal(1))
				fs, kilnfilePath, variablesFiles, variables := kilnFileLoader.LoadKilnfilesArgsForCall(0)

				Expect(fs).To(Equal(filesystem))
				Expect(kilnfilePath).To(Equal("Kilnfile"))
				Expect(variablesFiles).To(Equal([]string{"thisisafile"}))
				Expect(variables).To(Equal([]string{"someKey=someValue"}))
			})

			It("writes the new version to the Kilnfile.lock", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(kilnFileLoader.SaveKilnfileLockCallCount()).To(Equal(1))

				fs, path, updatedLockfile := kilnFileLoader.SaveKilnfileLockArgsForCall(0)
				Expect(fs).To(Equal(filesystem))
				Expect(path).To(Equal("Kilnfile"))
				Expect(updatedLockfile.Releases).To(HaveLen(2))
				Expect(updatedLockfile.Releases).To(ContainElement(
					cargo.ReleaseLock{
						Name:         releaseName,
						Version:      newReleaseVersion,
						SHA1:         newReleaseSha1,
						RemoteSource: newReleaseSourceName,
						RemotePath:   newRemotePath,
					},
				))
			})

			It("considers all release sources", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
					"--variable", "someKey=someValue",
					"--variables-file", "thisisafile",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(multiReleaseSourceProvider.CallCount()).To(Equal(1))
				_, allowOnlyPublishable := multiReleaseSourceProvider.ArgsForCall(0)
				Expect(allowOnlyPublishable).To(BeFalse())
			})
		})

		When("passing the --allow-only-publishable-releases flag", func() {
			var downloadErr error

			BeforeEach(func() {
				downloadErr = errors.New("asplode!!")
				releaseSource.DownloadReleaseReturns(release.Local{}, downloadErr)
			})

			It("tells the release downloader factory to allow only publishable releases", func() {
				err := updateReleaseCommand.Execute([]string{
					"--allow-only-publishable-releases",
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
					"--variable", "someKey=someValue",
					"--variables-file", "thisisafile",
				})
				Expect(err).To(MatchError(downloadErr))

				Expect(multiReleaseSourceProvider.CallCount()).To(Equal(1))
				_, allowOnlyPublishable := multiReleaseSourceProvider.ArgsForCall(0)
				Expect(allowOnlyPublishable).To(BeTrue())
			})
		})

		When("none of the release's fields change", func() {
			var logBuf *gbytes.Buffer

			BeforeEach(func() {
				expectedDownloadedRelease = release.Local{
					ID:        release.ID{Name: releaseName, Version: oldReleaseVersion},
					LocalPath: "not-used",
					SHA1:      oldReleaseSha1,
				}
				expectedRemoteRelease = release.Remote{
					ID:         expectedDownloadedRelease.ID,
					RemotePath: oldRemotePath,
					SourceID:   oldReleaseSourceName,
				}

				releaseSource.GetMatchedReleaseReturns(expectedRemoteRelease, true, nil)
				releaseSource.DownloadReleaseReturns(expectedDownloadedRelease, nil)

				logBuf = gbytes.NewBuffer()
				logger = log.New(logBuf, "", 0)
			})

			It("doesn't update the Kilnfile.lock", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", oldReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(kilnFileLoader.SaveKilnfileLockCallCount()).To(Equal(0))
			})

			It("notifies the user", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", oldReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(string(logBuf.Contents())).To(ContainSubstring("No changes made"))
				Expect(string(logBuf.Contents())).NotTo(ContainSubstring("Updated"))
				Expect(string(logBuf.Contents())).NotTo(ContainSubstring("COMMIT"))
			})
		})

		When("the named release isn't in Kilnfile.lock", func() {
			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("no release named \"no-such-release\"")))
				Expect(err).To(MatchError(ContainSubstring("try removing the -release")))
			})

			It("does not try to download anything", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				Expect(releaseSource.GetMatchedReleaseCallCount()).To(Equal(0))
				Expect(releaseSource.DownloadReleaseCallCount()).To(Equal(0))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", "no-such-release",
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				Expect(kilnFileLoader.SaveKilnfileLockCallCount()).To(Equal(0))
			})
		})

		When("there is an error loading the Kilnfiles", func() {
			BeforeEach(func() {
				kilnFileLoader.LoadKilnfilesReturns(cargo.Kilnfile{}, cargo.KilnfileLock{}, errors.New("big bada boom"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("big bada boom")))
			})
		})

		When("the release can't be found", func() {
			BeforeEach(func() {
				releaseSource.GetMatchedReleaseReturns(release.Remote{}, false, errors.New("bad stuff"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("bad stuff")))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				Expect(kilnFileLoader.SaveKilnfileLockCallCount()).To(Equal(0))
			})
		})

		When("downloading the release fails", func() {
			BeforeEach(func() {
				releaseSource.DownloadReleaseReturns(release.Local{}, errors.New("bad stuff"))
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring("bad stuff")))
			})

			It("does not update the Kilnfile.lock", func() {
				_ = updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})

				Expect(kilnFileLoader.SaveKilnfileLockCallCount()).To(Equal(0))
			})
		})

		When("writing to the Kilnfile.lock fails", func() {
			var expectedError error

			BeforeEach(func() {
				expectedError = errors.New("i don't feel so good")
				kilnFileLoader.SaveKilnfileLockReturns(expectedError)
			})

			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{
					"--kilnfile", "Kilnfile",
					"--name", releaseName,
					"--version", newReleaseVersion,
					"--releases-directory", releasesDir,
				})
				Expect(err).To(MatchError(ContainSubstring(expectedError.Error())))
			})
		})

		When("invalid arguments are given", func() {
			It("errors", func() {
				err := updateReleaseCommand.Execute([]string{"--no-such-flag"})
				Expect(err).To(MatchError(ContainSubstring("-no-such-flag")))
			})
		})
	})
})
