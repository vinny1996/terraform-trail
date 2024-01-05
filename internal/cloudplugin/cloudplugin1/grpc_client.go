// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cloudplugin1

import (
	"context"
	"io"
	"log"

	"github.com/hashicorp/terraform/internal/cloudplugin"
	"github.com/hashicorp/terraform/internal/cloudplugin/cloudproto1"
	"github.com/hashicorp/terraform/internal/command/format"
	"github.com/hashicorp/terraform/internal/terminal"
)

// GRPCCloudClient is the client interface for interacting with terraform-cloudplugin
type GRPCCloudClient struct {
	streams *terminal.Streams
	client  cloudproto1.CommandServiceClient
	context context.Context
}

// Proof that GRPCCloudClient fulfills the go-plugin interface
var _ cloudplugin.Cloud1 = GRPCCloudClient{}

// Execute sends the client Execute request and waits for the plugin to return
// an exit code response before returning
func (c GRPCCloudClient) Execute(args []string) int {
	client, err := c.client.Execute(c.context, &cloudproto1.CommandRequest{
		Args: args,
	})

	if err != nil {
		c.streams.Eprint(err.Error())
		return 1
	}

	for {
		// cloudplugin streams output as multiple CommandResponse value. Each
		// value will either contain stdout bytes, stderr bytes, or an exit code.
		response, err := client.Recv()
		if err == io.EOF {
			log.Print("[DEBUG] received EOF from cloudplugin")
			break
		} else if err != nil {
			c.streams.Eprintf("Failed to receive command response from cloudplugin: %s", err)
			return 1
		}

		if bytes := response.GetStdout(); len(bytes) > 0 {
			output := format.WordWrap(string(bytes), c.streams.Stdout.Columns())
			written, err := c.streams.Print(output)
			if err != nil {
				log.Printf("[ERROR] Failed to write cloudplugin output to stdout: %s", err)
				return 1
			}
			if written != len(bytes) {
				log.Printf("[ERROR] Wrote %d bytes to stdout but expected to write %d", written, len(bytes))
			}
		} else if bytes := response.GetStderr(); len(bytes) > 0 {
			output := format.WordWrap(string(bytes), c.streams.Stderr.Columns())
			written, err := c.streams.Eprint(output)
			if err != nil {
				log.Printf("[ERROR] Failed to write cloudplugin output to stderr: %s", err)
				return 1
			}
			if written != len(bytes) {
				log.Printf("[ERROR] Wrote %d bytes to stdout but expected to write %d", written, len(bytes))
			}
		} else {
			exitCode := response.GetExitCode()
			log.Printf("[TRACE] received exit code: %d", exitCode)
			if exitCode < 0 || exitCode > 255 {
				log.Printf("[ERROR] cloudplugin returned an invalid error code %d", exitCode)
				return 255
			}
			return int(exitCode)
		}
	}

	// This should indicate a bug in the plugin
	c.streams.Eprint("cloudplugin exited without responding with an error code")
	return 1
}
