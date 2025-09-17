// A common location to store utilities for testing.
package testutils

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type DockerAuth struct {
	Username string
	Password string
}

type TestData struct {
	Source               string
	Destination          string
	Expected             string
	Arguments            []string
	SkipServerTest       bool
	ServerErrorAllowList []string
	ServerImage          string
	DockerAuth           DockerAuth
}

func GetAerospikeContainerID(name string) ([]byte, error) {
	cmd := fmt.Sprintf("docker ps -a | grep '%s' | awk 'NF>1{print $NF}'", name)

	output, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return nil, err
	}

	if output[len(output)-1] == '\n' {
		output = output[:len(output)-1]
	}

	return output, nil
}

func StopAerospikeContainer(id string, cli *client.Client) error {
	ctx := context.Background()

	if err := cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
		log.Printf("Unable to stop container %s: %s", id, err)
		return err
	}

	return nil
}

func RemoveAerospikeContainer(id string, cli *client.Client) error {
	ctx := context.Background()

	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{}); err != nil {
		log.Printf("Unable to remove container %s: %s", id, err)
		return err
	}

	return nil
}

func CreateAerospikeContainer(
	name string,
	c *container.Config,
	ch *container.HostConfig,
	imagePullOpts image.PullOptions,
	cli *client.Client,
) (string, error) {
	ctx := context.Background()

	// Retry configuration for Docker Hub rate limiting
	maxRetries := 10
	baseBackoff := 10 * time.Second // Base backoff time
	maxBackoff := 5 * time.Minute   // Maximum backoff time

	var reader io.ReadCloser

	var err error

	// Retry loop for image pulling with exponential backoff
	for attempt := 1; attempt <= maxRetries; attempt++ {
		reader, err = cli.ImagePull(ctx, name, imagePullOpts)
		if err == nil {
			break
		}

		// Check if this is a rate limit error
		if strings.Contains(err.Error(), "toomanyrequests") || strings.Contains(err.Error(), "rate limit") {
			if attempt == maxRetries {
				log.Printf("Failed to pull image %s after %d attempts due to rate limiting: %s", name, maxRetries, err)
				return "", err
			}

			// Exponential backoff: 2^(attempt-1) * baseBackoff, capped at maxBackoff
			backoffTime := time.Duration(1<<(attempt-1)) * baseBackoff
			if backoffTime > maxBackoff {
				backoffTime = maxBackoff
			}

			log.Printf("Docker pull rate limit reached for %s, retrying in %v (attempt %d/%d)", name, backoffTime, attempt, maxRetries)
			time.Sleep(backoffTime)

			continue
		}

		// If it's not a rate limit error, don't retry
		log.Printf("Unable to pull image %s: %s", name, err)

		return "", err
	}

	defer reader.Close()

	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		log.Printf("Unable to read image pull response for %s: %s", name, err)
		return "", err
	}

	platform := &v1.Platform{
		Architecture: imagePullOpts.Platform,
	}

	resp, err := cli.ContainerCreate(ctx, c, ch, nil, platform, "")
	if err != nil {
		log.Printf("Unable to create container %s: %s", name, err)
		return "", err
	}

	return resp.ID, nil
}

func StartAerospikeContainer(id string, cli *client.Client) error {
	ctx := context.Background()

	if err := cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		log.Printf("Unable to start container %s: %s", id, err)
		return err
	}

	return nil
}

func IndexOf(l []string, s string) int {
	for i, e := range l {
		if e == s {
			return i
		}
	}

	return -1
}
