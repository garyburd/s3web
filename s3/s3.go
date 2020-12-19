// Copyright 2011 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package s3

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/garyburd/staticsite/common"
	"github.com/garyburd/staticsite/common/action"
	"github.com/garyburd/staticsite/site"
)

var (
	flagSet    = flag.NewFlagSet("s3", flag.ExitOnError)
	dryRun     = flagSet.Bool("n", false, "Dry run")
	force      = flagSet.Bool("f", false, "Force upload of all files")
	invalidate = flagSet.Bool("i", true, "Invalidate cloudfront distribution")

	Command = &common.Command{
		Name:    "s3",
		Usage:   "s3 [dir]",
		FlagSet: flagSet,
		Run:     run,
	}
)

// updater holds state neeed while updating S3.
type updater struct {
	dir string
	s3  *s3.S3
	cf  *cloudfront.CloudFront

	bucket                   string
	region                   string
	maxAge                   int
	unmanaged                []string
	cloudFrontDistributionID string
}

func run() {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		log.Fatal(err)
	}

	u := updater{
		dir:    flagSet.Arg(0),
		maxAge: 60 * 60,
	}

	if u.dir == "" {
		u.dir = "."
	}

	if err := u.readConfig(); err != nil {
		log.Fatal(err)
	}

	u.s3 = s3.New(sess, aws.NewConfig().WithRegion(u.region))
	u.cf = cloudfront.New(sess)

	uploadResources, deletePaths, err := u.getResourcesToUpdate()
	if err != nil {
		log.Fatal(err)
	}

	var invalidatePath string
	for _, r := range uploadResources {
		log.Printf("%s %s\n", r.UpdateReason, r.Path)
		if *dryRun {
			continue
		}
		if err := u.uploadResource(r); err != nil {
			log.Fatal(err)
		}
		if r.UpdateReason != updateNew {
			if invalidatePath == "" {
				invalidatePath = r.Path
			} else {
				invalidatePath = "/*"
			}
		}
	}

	if !*dryRun && *invalidate && invalidatePath != "" && u.cloudFrontDistributionID != "" {
		if strings.HasSuffix(invalidatePath, "/index.html") {
			invalidatePath = invalidatePath[:len(invalidatePath)-len("index.html")]
		}
		log.Printf("Invalidating CloudFront distribution: %s", invalidatePath)
		err := u.invalidateDistribution(invalidatePath)
		if err != nil {
			log.Fatal(err)
		}
	}

	for _, p := range deletePaths {
		log.Printf("D %s\n", p)
		if *dryRun {
			continue
		}
		if err := u.deleteResource(p); err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("View the updated website at http://%s.s3-website-%s.amazonaws.com/", u.bucket, u.region)
}

func (u *updater) readConfig() error {
	fpath := filepath.Join(u.dir, filepath.FromSlash(common.ConfigDir), "s3.txt")

	actions, lc, err := action.ParseFile(fpath)
	if err != nil {
		return err
	}

	for _, a := range actions {
		switch a.Name {
		case action.TextAction:
			if b := bytes.TrimSpace(a.Text); len(b) != 0 {
				return fmt.Errorf("%s: unknown text %q", a.Location(lc), b)
			}
		case "set":
			for k, v := range a.Args {
				switch k {
				case "bucket":
					u.bucket = v.Text
				case "region":
					u.region = v.Text
				case "cloudFrontDistributionID":
					u.cloudFrontDistributionID = v.Text
				case "maxAge":
					var err error
					u.maxAge, err = strconv.Atoi(v.Text)
					if err != nil {
						return fmt.Errorf("%s: %w", v.Location(lc), err)
					}
				case "unmanged":
					u.unmanaged = strings.Split(v.Text, ":")
					for i, p := range u.unmanaged {
						if !strings.HasSuffix(p, "/") {
							u.unmanaged[i] = p + "/"
						}
					}
				default:
					return fmt.Errorf("%s: unknown argument %q", v.Location(lc), k)
				}
			}
		default:
			return fmt.Errorf("%s: unknown command %q", a.Location(lc), a.Name)
		}
	}

	if u.bucket == "" {
		return fmt.Errorf("%s:1: Bucket name not set", fpath)
	}

	if u.region == "" {
		return fmt.Errorf("%s:1: Region name not set", fpath)
	}

	return nil
}

func (u *updater) readObjects() (map[string]*s3.Object, error) {
	objects := make(map[string]*s3.Object)
	var continuationToken *string
	for {
		out, err := u.s3.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket:            aws.String(u.bucket),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, err
		}
		for _, o := range out.Contents {
			objects[aws.StringValue(o.Key)] = o
		}
		if !aws.BoolValue(out.IsTruncated) {
			break
		}
		continuationToken = out.NextContinuationToken
	}
	return objects, nil
}

const (
	updateNew        = "N"
	updateHashChange = "H"
	updateForce      = "F"
	updateTimeChange = "T"
	updateSizeChange = "S"
)

func (u *updater) getResourcesToUpdate() ([]*site.Resource, []string, error) {
	objects, err := u.readObjects()
	if err != nil {
		return nil, nil, err
	}

	var (
		newResources      []*site.Resource
		modifiedResources []*site.Resource
	)
	err = site.Visit(u.dir, os.Stderr, func(r *site.Resource) error {
		if strings.HasSuffix(r.Path, "/") {
			r.Path = r.Path + "index.html"
		}
		key := r.Path[1:]
		o, ok := objects[key]
		if !ok {
			r.UpdateReason = updateNew
			newResources = append(newResources, r)
			return nil
		}
		delete(objects, key)
		if r.Data != nil {
			switch {
			case aws.StringValue(o.ETag) != fmt.Sprintf(`"%x"`, md5.Sum(r.Data)):
				r.UpdateReason = updateHashChange
			case *force:
				r.UpdateReason = updateForce
			default:
				return nil
			}
		} else {
			switch {
			case r.Size != aws.Int64Value(o.Size):
				r.UpdateReason = updateSizeChange
			case r.ModTime.After(aws.TimeValue(o.LastModified)):
				r.UpdateReason = updateTimeChange
			case *force:
				r.UpdateReason = updateForce
			default:
				return nil
			}
		}
		modifiedResources = append(modifiedResources, r)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Find resources to delete. Skip unmanaged.

	var deletePaths []string
delete:
	for key := range objects {
		path := "/" + key
		for _, prefix := range u.unmanaged {
			if strings.HasPrefix(path, prefix) {
				continue delete
			}
		}
		deletePaths = append(deletePaths, path)
	}

	return append(newResources, modifiedResources...), deletePaths, err
}

func (u *updater) uploadResource(r *site.Resource) error {
	f, ct, err := r.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	input := &s3.PutObjectInput{
		Bucket:       aws.String(u.bucket),
		Key:          aws.String(r.Path[1:]),
		Body:         f,
		ContentType:  aws.String(ct),
		ACL:          aws.String("public-read"),
		CacheControl: aws.String(fmt.Sprintf("public, max-age=%d", u.maxAge)),
	}
	if r.Redirect != "" {
		input.WebsiteRedirectLocation = aws.String(r.Redirect)
	}
	_, err = u.s3.PutObject(input)
	return err
}

func (u *updater) deleteResource(p string) error {
	_, err := u.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(p[1:]),
	})
	return err
}

func (u *updater) invalidateDistribution(path string) error {
	_, err := u.cf.CreateInvalidation(&cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(u.cloudFrontDistributionID),
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: aws.String(time.Now().String()),
			Paths: &cloudfront.Paths{
				Items:    []*string{aws.String(path)},
				Quantity: aws.Int64(1),
			},
		},
	})
	return err
}
