// Copyright 2020 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coffee

import (
	"github.com/gohugoio/hugo/common/herrors"
	"github.com/gohugoio/hugo/common/hugo"
	"github.com/gohugoio/hugo/media"
	"github.com/gohugoio/hugo/resources/internal"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mitchellh/mapstructure"

	"github.com/gohugoio/hugo/resources"
	"github.com/gohugoio/hugo/resources/resource"
)

// Options from https://coffeescript.org/#cli
type Options struct {
	OutputPath string // Custom output path of JS file
	InlineMap  bool   // Include the source map directly in the compiled JavaScript file
	Bare       bool   // Compile the JavaScript without the top-level function safety wrapper
	NoHeader   bool   // Suppress the “Generated by CoffeeScript” header
}

func DecodeOptions(m map[string]interface{}) (opts Options, err error) {
	if m == nil {
		return
	}
	err = mapstructure.WeakDecode(m, &opts)
	return
}
func (opts Options) toArgs() []string {
	var args []string

	if opts.InlineMap {
		args = append(args, "--inline-map")
	}
	if opts.Bare {
		args = append(args, "--bare")
	}
	if opts.NoHeader {
		args = append(args, "--no-header")
	}
	return args
}

// Client is the client used to do CoffeeScript transformations.
type Client struct {
	rs *resources.Spec
}

// New creates a new Client with the given specification.
func New(rs *resources.Spec) *Client {
	return &Client{rs: rs}
}

type coffeeTransformation struct {
	options Options
	rs      *resources.Spec
}

func (t *coffeeTransformation) Key() internal.ResourceTransformationKey {
	return internal.NewResourceTransformationKey("coffee", t.options)
}

// Delegates to coffee cli to do the transformation.
// Requires 'coffeescript' package installed (npm install --save-dev coffeescript)
// Output can be sent to babel/minify/etc for further optimisations
func (t *coffeeTransformation) Transform(ctx *resources.ResourceTransformationCtx) error {
	const localCoffeePath = "node_modules/.bin/"
	const binaryName = "coffee"

	// Look for coffee binary
	csiBinPath := filepath.Join(t.rs.WorkingDir, localCoffeePath, binaryName)
	binary := csiBinPath

	if _, err := exec.LookPath(binary); err != nil {
		// Try PATH
		binary = binaryName
		if _, err := exec.LookPath(binary); err != nil {

			// This may be on a CI server etc. Will fall back to pre-built assets.
			return herrors.ErrFeatureNotAvailable
		}
	}

	// Update resource metadata (coffee -> javascript)
	ctx.OutMediaType = media.JavascriptType
	if t.options.OutputPath != "" {
		ctx.InPath = t.options.OutputPath
	} else {
		ctx.ReplaceOutPathExtension(".js") // Default (just change extension)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "-sc") // Calls cli with 'stdio' and 'compile' params

	if optArgs := t.options.toArgs(); len(optArgs) > 0 {
		cmdArgs = append(cmdArgs, optArgs...)
	}

	cmd := exec.Command(binary, cmdArgs...)

	cmd.Stdout = ctx.To
	cmd.Stderr = os.Stderr
	cmd.Env = hugo.GetExecEnviron(t.rs.Cfg)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, ctx.From)
	}()

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Process(res resources.ResourceTransformer, options Options) (resource.Resource, error) {
	return res.Transform(
		&coffeeTransformation{rs: c.rs, options: options},
	)
}
