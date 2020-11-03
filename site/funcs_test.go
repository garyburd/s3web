// Copyright 2019 Gary Burd
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

package site

import (
	"image"
	"reflect"
	"testing"
)

var srcSetTests = []*struct {
	fpaths    []string
	upaths    []string
	configs   []image.Config
	maxWidth  int
	maxHeight int
	expected  *ImageSrcSet
}{
	{
		fpaths:   []string{"/best.jpg", "/other.jpg"},
		upaths:   []string{"best.jpg", "other.jpg"},
		configs:  []image.Config{{Width: 600, Height: 600}, {Width: 500, Height: 500}},
		maxWidth: 600,
		expected: &ImageSrcSet{Image: Image{Width: 600, Height: 600, Src: "best.jpg"}, SrcSet: "best.jpg 600w,other.jpg 500w"},
	},
	{
		fpaths:   []string{"/other.jpg", "/best.jpg"},
		upaths:   []string{"other.jpg", "best.jpg"},
		configs:  []image.Config{{Width: 500, Height: 500}, {Width: 600, Height: 600}},
		maxWidth: 600,
		expected: &ImageSrcSet{Image: Image{Width: 600, Height: 600, Src: "best.jpg"}, SrcSet: "other.jpg 500w,best.jpg 600w"},
	},
	{
		fpaths:   []string{"/best.jpg", "/other.jpg"},
		upaths:   []string{"best.jpg", "other.jpg"},
		configs:  []image.Config{{Width: 600, Height: 600}, {Width: 700, Height: 700}},
		maxWidth: 600,
		expected: &ImageSrcSet{Image: Image{Width: 600, Height: 600, Src: "best.jpg"}, SrcSet: "best.jpg 600w,other.jpg 700w"},
	},
	{
		fpaths:   []string{"/other.jpg", "/best.jpg"},
		upaths:   []string{"other.jpg", "best.jpg"},
		configs:  []image.Config{{Width: 700, Height: 700}, {Width: 600, Height: 600}},
		maxWidth: 600,
		expected: &ImageSrcSet{Image: Image{Width: 600, Height: 600, Src: "best.jpg"}, SrcSet: "other.jpg 700w,best.jpg 600w"},
	},
	{
		fpaths:   []string{"/best.jpg", "/other.jpg"},
		upaths:   []string{"best.jpg", "other.jpg"},
		configs:  []image.Config{{Width: 600, Height: 600}, {Width: 500, Height: 500}},
		maxWidth: 550,
		expected: &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "best.jpg 600w,other.jpg 500w"},
	},
	{
		fpaths:   []string{"/other.jpg", "/best.jpg"},
		upaths:   []string{"other.jpg", "best.jpg"},
		configs:  []image.Config{{Width: 500, Height: 500}, {Width: 600, Height: 600}},
		maxWidth: 550,
		expected: &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "other.jpg 500w,best.jpg 600w"},
	},
	{
		fpaths:   []string{"/best.jpg", "/other.jpg"},
		upaths:   []string{"best.jpg", "other.jpg"},
		configs:  []image.Config{{Width: 600, Height: 600}, {Width: 700, Height: 700}},
		maxWidth: 550,
		expected: &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "best.jpg 600w,other.jpg 700w"},
	},
	{
		fpaths:   []string{"/other.jpg", "/best.jpg"},
		upaths:   []string{"other.jpg", "best.jpg"},
		configs:  []image.Config{{Width: 700, Height: 700}, {Width: 600, Height: 600}},
		maxWidth: 550,
		expected: &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "other.jpg 700w,best.jpg 600w"},
	},

	{
		fpaths:    []string{"/best.jpg", "/other.jpg"},
		upaths:    []string{"best.jpg", "other.jpg"},
		configs:   []image.Config{{Width: 600, Height: 600}, {Width: 500, Height: 500}},
		maxWidth:  600,
		maxHeight: 550,
		expected:  &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "best.jpg 600w,other.jpg 500w"},
	},
	{
		fpaths:    []string{"/other.jpg", "/best.jpg"},
		upaths:    []string{"other.jpg", "best.jpg"},
		configs:   []image.Config{{Width: 500, Height: 500}, {Width: 600, Height: 600}},
		maxWidth:  600,
		maxHeight: 550,
		expected:  &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "other.jpg 500w,best.jpg 600w"},
	},
	{
		fpaths:    []string{"/best.jpg", "/other.jpg"},
		upaths:    []string{"best.jpg", "other.jpg"},
		configs:   []image.Config{{Width: 600, Height: 600}, {Width: 700, Height: 700}},
		maxWidth:  600,
		maxHeight: 550,
		expected:  &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "best.jpg 600w,other.jpg 700w"},
	},
	{
		fpaths:    []string{"/other.jpg", "/best.jpg"},
		upaths:    []string{"other.jpg", "best.jpg"},
		configs:   []image.Config{{Width: 700, Height: 700}, {Width: 600, Height: 600}},
		maxWidth:  600,
		maxHeight: 550,
		expected:  &ImageSrcSet{Image: Image{Width: 550, Height: 550, Src: "best.jpg"}, SrcSet: "other.jpg 700w,best.jpg 600w"},
	},
}

func TestWalk(t *testing.T) {
	for i, tt := range srcSetTests {
		iss, _ := computeSrcSet(tt.fpaths, tt.upaths, tt.configs, tt.maxWidth, tt.maxHeight)
		if !reflect.DeepEqual(iss, tt.expected) {
			t.Errorf("fail %d\n got = %#v\nwant = %#v", i, iss, tt.expected)
		}
	}
}
