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
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/garyburd/staticsite/site"
)

var (
	flagSet    = flag.NewFlagSet("s3", flag.ExitOnError)
	dryRun     = flagSet.Bool("n", false, "Dry run")
	force      = flagSet.Bool("f", false, "Force upload of all files")
	invalidate = flagSet.Bool("i", true, "Invalidate cloudfront distribution")

	Command = &site.Command{
		Name:    "s3",
		Usage:   "s3 [dir]",
		FlagSet: flagSet,
		Run:     run,
	}
)

// config represents the JSON configuration file.
type config struct {
	Bucket                   string
	Region                   string
	MaxAge                   int
	Unmanaged                []string
	CloudFrontDistributionID string
}

// updater holds state neeed while updating S3.
type updater struct {
	dir    string
	config config
	s3     *s3.S3
	cf     *cloudfront.CloudFront
}

func run() {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		log.Fatal(err)
	}

	u := updater{
		dir: flagSet.Arg(0),
	}

	if u.dir == "" {
		u.dir = "."
	}

	if err := u.readConfig(); err != nil {
		log.Fatal(err)
	}

	u.s3 = s3.New(sess, aws.NewConfig().WithRegion(u.config.Region))
	u.cf = cloudfront.New(sess)

	resources, err := u.getResourcesToUpdate()
	if err != nil {
		log.Fatal(err)
	}

	var resourceModified bool
	for _, r := range resources {
		log.Printf("%s %s\n", r.UpdateReason, r.Path)
		if *dryRun {
			continue
		}
		var err error
		if r.FilePath != "" || r.Data != nil {
			err = u.uploadResource(r)
			if r.UpdateReason != updateNew {
				resourceModified = true
			}
		} else {
			err = u.deleteResource(r)
		}
		if err != nil {
			log.Fatal(err)
		}
	}

	if !*dryRun && *invalidate && resourceModified && u.config.CloudFrontDistributionID != "" {
		log.Printf("Invalidating CloudFront distribution")
		err := u.invalidateDistribution()
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("View the updated website at http://%s.s3-website-%s.amazonaws.com/", u.config.Bucket, u.config.Region)
}

func (u *updater) readConfig() error {
	fpath := filepath.Join(u.dir, filepath.FromSlash(site.ConfigDir), "s3.json")

	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}

	err = site.DecodeConfig(fpath, data, &u.config)
	if err != nil {
		return err
	}

	if u.config.Bucket == "" {
		return fmt.Errorf("%s:1: Bucket name not set", fpath)
	}

	if u.config.Region == "" {
		return fmt.Errorf("%s:1: Region name not set", fpath)
	}

	if u.config.MaxAge == 0 {
		u.config.MaxAge = 60 * 60
	}

	for i, prefix := range u.config.Unmanaged {
		if !strings.HasSuffix(prefix, "/") {
			u.config.Unmanaged[i] = prefix + "/"
		}
	}
	return nil
}

func (u *updater) readObjects() (map[string]*s3.Object, error) {
	objects := make(map[string]*s3.Object)
	var continuationToken *string
	for {
		out, err := u.s3.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket:            aws.String(u.config.Bucket),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, err
		}
		for _, o := range out.Contents {
			objects[aws.StringValue(o.Key)] = o
		}
		continuationToken = out.ContinuationToken
		if aws.StringValue(continuationToken) == "" {
			break
		}
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

func (u *updater) getResourcesToUpdate() ([]*site.Resource, error) {
	objects, err := u.readObjects()
	if err != nil {
		return nil, err
	}

	var resources []*site.Resource
	err = site.Walk(u.dir, func(r *site.Resource) error {
		key := r.Path[1:]
		o, ok := objects[key]
		if !ok {
			r.UpdateReason = updateNew
		} else {
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
		}
		resources = append(resources, r)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Find resources to delete. Skip unmanaged.
delete:
	for key := range objects {
		path := "/" + key
		for _, prefix := range u.config.Unmanaged {
			if strings.HasPrefix(path, prefix) {
				continue delete
			}
		}
		resources = append(resources, &site.Resource{Path: path, UpdateReason: "D"})
	}
	return resources, err
}

func (u *updater) uploadResource(r *site.Resource) error {
	f, ct, err := r.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = u.s3.PutObject(&s3.PutObjectInput{
		Bucket:       aws.String(u.config.Bucket),
		Key:          aws.String(r.Path[1:]),
		Body:         f,
		ContentType:  aws.String(ct),
		ACL:          aws.String("public-read"),
		CacheControl: aws.String(fmt.Sprintf("max-age=%d", u.config.MaxAge)),
	})
	return err
}

func (u *updater) deleteResource(r *site.Resource) error {
	_, err := u.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(u.config.Bucket),
		Key:    aws.String(r.Path[1:]),
	})
	return err
}

func (u *updater) invalidateDistribution() error {
	_, err := u.cf.CreateInvalidation(&cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(u.config.CloudFrontDistributionID),
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: aws.String(time.Now().String()),
			Paths: &cloudfront.Paths{
				Items:    []*string{aws.String("/*")},
				Quantity: aws.Int64(1),
			},
		},
	})
	return err
}
