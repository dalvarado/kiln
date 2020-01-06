package commands

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
)

//go:generate counterfeiter -o ./fakes/s3_uploader.go --fake-name S3Uploader . S3Uploader

type S3Uploader interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

type PushRelease struct {
	FS             billy.Filesystem
	KilnfileLoader KilnfileLoader
	UploaderConfig func(*cargo.ReleaseSourceConfig) (S3Uploader, error)

	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`

		Name   string `short:"n" long:"name" required:"true" description:"name of release to update"`
		Remote string `short:"r" long:"remote" required:"true" description:"name of remote source"`
		Path   string `short:"p" long:"path" required:"true" description:"path to BOSH release tarball, the file should be be named like 'my-rel-1.2.3.tgz'"`
	}
}

func (pushRelease PushRelease) Execute(args []string) error {
	_, err := jhanda.Parse(&pushRelease.Options, args)
	if err != nil {
		return err
	}

	kilnfile, _, err := pushRelease.KilnfileLoader.LoadKilnfiles(
		pushRelease.FS,
		pushRelease.Options.Kilnfile,
		pushRelease.Options.VariablesFiles,
		pushRelease.Options.Variables,
	)
	if err != nil {
		return fmt.Errorf("error loading Kilnfiles: %w", err)
	}

	file, err := pushRelease.FS.Open(pushRelease.Options.Path)
	if err != nil {
		return fmt.Errorf("could not open release: %w", err)
	}

	var (
		rc *cargo.ReleaseSourceConfig

		validSourcesForErrOutput []string
	)

	for index, rel := range kilnfile.ReleaseSources {
		if rel.Type == fetcher.ReleaseSourceTypeS3 {
			validSourcesForErrOutput = append(validSourcesForErrOutput, rel.Bucket)
			if rel.Bucket == pushRelease.Options.Remote {
				rc = &kilnfile.ReleaseSources[index]
				break
			}
		}
	}

	if rc == nil {
		const msg = "remote release source could not be found in Kilnfile (only release sources of type s3 are supported)"
		if len(validSourcesForErrOutput) > 0 {
			return fmt.Errorf(msg+", some acceptable sources are: %v", validSourcesForErrOutput)
		}
		return errors.New(msg)
	}

	uploader, err := pushRelease.UploaderConfig(rc)
	if err != nil {
		return fmt.Errorf("could not configure s3 uploader client: %w", err)
	}

	if _, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(rc.Bucket),
		Key:    aws.String(filepath.Base(pushRelease.Options.Path)),
		Body:   file,
	}); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	return nil
}

func (pushRelease PushRelease) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Uploads a Bosh Release to a release source for use in kiln fetch",
		ShortDescription: "Upload BOSH Release",
		Flags:            pushRelease.Options,
	}
}