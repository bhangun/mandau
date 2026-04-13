package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(deployCmd)
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deployment helper commands",
}

func init() {
	deployCmd.AddCommand(deployContainerCmd)
	deployCmd.AddCommand(deployStatusCmd)
	deployCmd.AddCommand(deployRollbackCmd)
}

var deployContainerCmd = &cobra.Command{
	Use:   "container [local-image] [remote-image]",
	Short: "Deploy a local Docker image to a remote agent",
	Args:  cobra.ExactArgs(2),
	RunE:  func(cmd *cobra.Command, args []string) error { return cli.deployContainer(cmd, args) },
}

func (c *CLI) deployContainer(cmd *cobra.Command, args []string) error {
	localImage := args[0]
	remoteImage := args[1]
	upRemote, _ := cmd.Flags().GetBool("up-remote")
	containerName, _ := cmd.Flags().GetString("name")
	ports, _ := cmd.Flags().GetStringSlice("port")
	envVars, _ := cmd.Flags().GetStringSlice("env")
	volumes, _ := cmd.Flags().GetStringSlice("volume")
	runArgs, _ := cmd.Flags().GetStringSlice("docker-run-args")
	verify, _ := cmd.Flags().GetBool("verify")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	agentID, err := c.resolveAgent(cmd)
	if err != nil {
		return err
	}

	ctx := context.Background()

	fmt.Printf("\n📦 Deploying image '%s' to agent %s as '%s'\n", localImage, agentID, remoteImage)
	if dryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made\n")
	}

	// Step 1: Stream docker save → docker load
	fmt.Println("📤 Transferring image via streaming...")
	if err := c.streamImageToRemote(ctx, agentID, localImage); err != nil {
		return err
	}
	fmt.Println("✓ Image transferred successfully\n")

	// Step 2: Tag image
	fmt.Println("🏷️  Tagging image...")
	if err := c.tagImageOnRemote(ctx, agentID, localImage, remoteImage, dryRun); err != nil {
		fmt.Printf("⚠️  Warning: tagging failed: %v\n\n", err)
	} else {
		fmt.Println("✓ Image tagged\n")
	}

	// Step 3: Verify image exists (optional)
	if verify {
		fmt.Println("🔍 Verifying image on remote...")
		if err := c.verifyImageOnRemote(ctx, agentID, remoteImage); err != nil {
			return fmt.Errorf("image verification failed: %w", err)
		}
		fmt.Println("✓ Image verification passed\n")
	}

	// Step 4: Optionally start container
	if upRemote {
		if dryRun {
			fmt.Printf("📋 Would run: docker run -d")
			if containerName != "" {
				fmt.Printf(" --name %s", containerName)
			}
			for _, p := range ports {
				fmt.Printf(" -p %s", p)
			}
			for _, e := range envVars {
				fmt.Printf(" -e %s", e)
			}
			for _, v := range volumes {
				fmt.Printf(" -v %s", v)
			}
			for _, a := range runArgs {
				fmt.Printf(" %s", a)
			}
			fmt.Printf(" %s\n\n", remoteImage)
			fmt.Println("✓ Deployment complete (dry run)")
			return nil
		}

		fmt.Println("🚀 Starting container on remote...")
		containerID, err := c.runContainerOnRemote(ctx, agentID, remoteImage, containerName, ports, envVars, volumes, runArgs)
		if err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}
		fmt.Printf("✓ Container started: %s\n\n", containerID)
	}

	fmt.Println("✅ Deployment complete")
	return nil
}

func joinArgs(args []string) string {
	return "" + fmt.Sprint(args)[1:]
}

// streamImageToRemote pipes local docker save into remote docker load
func (c *CLI) streamImageToRemote(ctx context.Context, agentID, localImage string) error {
	saveCmd := exec.Command("docker", "save", localImage)
	stdout, err := saveCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("docker save stdout pipe: %w", err)
	}
	stderr, err := saveCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("docker save stderr pipe: %w", err)
	}

	if err := saveCmd.Start(); err != nil {
		return fmt.Errorf("start docker save: %w", err)
	}

	hs, err := c.coreClient.HostShell(ctx)
	if err != nil {
		_ = saveCmd.Process.Kill()
		return fmt.Errorf("start host shell: %w", err)
	}

	if err := hs.Send(&v1.HostShellRequest{AgentId: agentID, Data: []byte("docker load -i -\n")}); err != nil {
		_ = saveCmd.Process.Kill()
		hs.CloseSend()
		return fmt.Errorf("send docker load command: %w", err)
	}

	sendErrCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				if serr := hs.Send(&v1.HostShellRequest{AgentId: agentID, Data: buf[:n]}); serr != nil {
					sendErrCh <- fmt.Errorf("send chunk: %w", serr)
					return
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				sendErrCh <- fmt.Errorf("read docker save: %w", rerr)
				return
			}
		}
		if err := hs.CloseSend(); err != nil {
			sendErrCh <- fmt.Errorf("close send: %w", err)
			return
		}
		sendErrCh <- nil
	}()

	go func() {
		io.Copy(os.Stderr, stderr)
	}()

	for {
		resp, rerr := hs.Recv()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			select {
			case serr := <-sendErrCh:
				if serr != nil {
					return serr
				}
			default:
			}
			return fmt.Errorf("host shell recv: %w", rerr)
		}
		if len(resp.Data) > 0 {
			fmt.Print(string(resp.Data))
		}
		if resp.Error != "" {
			return fmt.Errorf("remote error: %s", resp.Error)
		}
	}

	if serr := <-sendErrCh; serr != nil {
		return serr
	}

	if err := saveCmd.Wait(); err != nil {
		return fmt.Errorf("docker save failed: %w", err)
	}

	return nil
}

// tagImageOnRemote tags an image on the remote agent
func (c *CLI) tagImageOnRemote(ctx context.Context, agentID, sourceImage, targetImage string, dryRun bool) error {
	if dryRun {
		fmt.Printf("  docker tag %s %s\n", sourceImage, targetImage)
		return nil
	}

	stream, err := c.coreClient.ExecuteDockerCommand(ctx, &v1.DockerCommandRequest{
		AgentId: agentID,
		Args:    []string{"tag", sourceImage, targetImage},
	})
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(resp.Output) > 0 {
			fmt.Print(string(resp.Output))
		}
		if resp.Error != "" {
			return fmt.Errorf("%s", resp.Error)
		}
	}
	return nil
}

// verifyImageOnRemote verifies that an image exists on the remote agent
func (c *CLI) verifyImageOnRemote(ctx context.Context, agentID, imageName string) error {
	stream, err := c.coreClient.ExecuteDockerCommand(ctx, &v1.DockerCommandRequest{
		AgentId: agentID,
		Args:    []string{"image", "inspect", imageName},
	})
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if resp.Error != "" {
			return fmt.Errorf("image not found or verify failed: %s", resp.Error)
		}
	}
	return nil
}

// runContainerOnRemote starts a container on the remote agent with the given options
func (c *CLI) runContainerOnRemote(ctx context.Context, agentID, imageName, containerName string, ports, envVars, volumes, extraArgs []string) (string, error) {
	runArgs := []string{"run", "-d"}

	if containerName != "" {
		runArgs = append(runArgs, "--name", containerName)
	}

	for _, port := range ports {
		runArgs = append(runArgs, "-p", port)
	}

	for _, env := range envVars {
		runArgs = append(runArgs, "-e", env)
	}

	for _, vol := range volumes {
		runArgs = append(runArgs, "-v", vol)
	}

	runArgs = append(runArgs, extraArgs...)
	runArgs = append(runArgs, imageName)

	fmt.Printf("  Command: docker %s\n", strings.Join(runArgs, " "))

	stream, err := c.coreClient.ExecuteDockerCommand(ctx, &v1.DockerCommandRequest{
		AgentId: agentID,
		Args:    runArgs,
	})
	if err != nil {
		return "", err
	}

	var output strings.Builder
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if len(resp.Output) > 0 {
			fmt.Print(string(resp.Output))
			output.Write(resp.Output)
		}
		if resp.Error != "" {
			return "", fmt.Errorf("%s", resp.Error)
		}
	}

	containerID := strings.TrimSpace(output.String())
	if containerID == "" {
		return "", fmt.Errorf("no container ID returned")
	}

	return containerID, nil
}

func init() {
	deployContainerCmd.Flags().Bool("up-remote", false, "Start the container on the remote agent after loading the image")
	deployContainerCmd.Flags().StringP("name", "n", "", "Container name for the running container")
	deployContainerCmd.Flags().StringSliceP("port", "p", []string{}, "Publish container port(s) (-p host:container)")
	deployContainerCmd.Flags().StringSliceP("env", "e", []string{}, "Set environment variable(s) (-e KEY=VALUE)")
	deployContainerCmd.Flags().StringSliceP("volume", "v", []string{}, "Mount volume(s) (-v /host:/container)")
	deployContainerCmd.Flags().StringSlice("docker-run-args", []string{}, "Additional docker run arguments")
	deployContainerCmd.Flags().Bool("verify", false, "Verify image exists on remote after loading")
	deployContainerCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
}

// deployStatusCmd shows deployment status on agents
var deployStatusCmd = &cobra.Command{
	Use:   "status [image-name]",
	Short: "Show deployment status of images and containers",
	Args:  cobra.MaximumNArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return cli.deployStatus(cmd, args) },
}

// deployRollbackCmd stops and removes a deployed container
var deployRollbackCmd = &cobra.Command{
	Use:   "rollback [container-name]",
	Short: "Rollback a deployed container (stop and remove)",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return cli.deployRollback(cmd, args) },
}

func (c *CLI) deployStatus(cmd *cobra.Command, args []string) error {
	filterImage := ""
	if len(args) > 0 {
		filterImage = args[0]
	}

	agentID, err := c.resolveAgent(cmd)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Get container list
	resp, err := c.coreClient.ListContainers(ctx, &v1.ListContainersRequest{
		AgentId: agentID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n📊 Deployment Status (Agent: %s)\n\n", agentID)
	fmt.Printf("%-20s %-30s %-20s %-15s\n", "CONTAINER", "IMAGE", "STATE", "STATUS")
	fmt.Println(strings.Repeat("-", 85))

	count := 0
	for _, c := range resp.Containers {
		if filterImage != "" && !strings.Contains(c.Image, filterImage) {
			continue
		}

		name := c.Name
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		image := c.Image
		if len(image) > 30 {
			image = image[:27] + "..."
		}

		fmt.Printf("%-20s %-30s %-20s %-15s\n", name, image, c.State, c.Status)
		count++
	}

	if count == 0 {
		if filterImage != "" {
			fmt.Printf("No containers found matching '%s'\n", filterImage)
		} else {
			fmt.Println("No containers found")
		}
	}

	fmt.Printf("\n%d container(s) running\n\n", count)
	return nil
}

func (c *CLI) deployRollback(cmd *cobra.Command, args []string) error {
	containerName := args[0]
	force, _ := cmd.Flags().GetBool("force")

	agentID, err := c.resolveAgent(cmd)
	if err != nil {
		return err
	}

	ctx := context.Background()

	fmt.Printf("\n🔄 Rolling back container: %s\n\n", containerName)

	if !force {
		fmt.Printf("This will stop and remove the container '%s'.\n", containerName)
		fmt.Print("Continue? (y/n) ")
		var input string
		fmt.Scanln(&input)
		if input != "y" && input != "Y" {
			fmt.Println("❌ Rollback cancelled")
			return nil
		}
	}

	// Stop container
	fmt.Println("Stopping container...")
	stopStream, err := c.coreClient.ExecuteDockerCommand(ctx, &v1.DockerCommandRequest{
		AgentId: agentID,
		Args:    []string{"stop", containerName},
	})
	if err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	for {
		resp, err := stopStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stop stream error: %w", err)
		}
		if resp.Error != "" && !strings.Contains(resp.Error, "No such container") {
			return fmt.Errorf("stop error: %s", resp.Error)
		}
	}

	// Remove container
	fmt.Println("Removing container...")
	rmStream, err := c.coreClient.ExecuteDockerCommand(ctx, &v1.DockerCommandRequest{
		AgentId: agentID,
		Args:    []string{"rm", containerName},
	})
	if err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}

	for {
		resp, err := rmStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("remove stream error: %w", err)
		}
		if resp.Error != "" && !strings.Contains(resp.Error, "No such container") {
			return fmt.Errorf("remove error: %s", resp.Error)
		}
	}

	fmt.Printf("✅ Container '%s' rolled back successfully\n\n", containerName)
	return nil
}

func init() {
	deployRollbackCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}
