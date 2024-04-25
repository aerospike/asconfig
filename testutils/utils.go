// A common location to store utilities for testing.
package testutils

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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

func CreateAerospikeContainer(name string, c *container.Config, ch *container.HostConfig, imagePullOpts types.ImagePullOptions, cli *client.Client) (string, error) {
	ctx := context.Background()
	reader, err := cli.ImagePull(ctx, name, imagePullOpts)
	if err != nil {
		log.Printf("Unable to pull image %s: %s", name, err)
		return "", err
	}

	defer reader.Close()
	io.Copy(os.Stdout, reader)

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
