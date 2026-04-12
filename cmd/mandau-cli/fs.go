package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	v1 "github.com/bhangun/mandau/api/v1"
	"github.com/spf13/cobra"
)

func init() {
	fsCmd.AddCommand(fsLsCmd)
	fsCmd.AddCommand(fsCpCmd)
	fsCmd.AddCommand(fsCatCmd)
	fsCmd.AddCommand(fsFetchCmd)
	fsCmd.AddCommand(fsMvCmd)
	fsCmd.AddCommand(fsRmCmd)
	fsCmd.AddCommand(fsMkdirCmd)
}

var fsCmd = &cobra.Command{
	Use:   "fs",
	Short: "Remote filesystem management",
	Long:  `Manage files and directories on remote agents.`,
}

var fsLsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List files on remote agent",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		path := "/"
		if len(args) > 0 {
			path = args[0]
		}

		resp, err := cli.coreClient.ListFiles(context.Background(), &v1.ListFilesRequest{
			AgentId: agentID,
			Path:    path,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Contents of %s on agent %s:\n\n", path, agentID)
		fmt.Printf("%-20s %-10s %-20s %s\n", "NAME", "SIZE", "MODIFIED", "MODE")
		fmt.Println(strings.Repeat("-", 70))

		for _, file := range resp.Files {
			name := file.Name
			if file.IsDir {
				name += "/"
			}
			modified := file.Modified.AsTime().Format("2006-01-02 15:04:05")
			fmt.Printf("%-20s %-10d %-20s %s\n", name, file.Size, modified, os.FileMode(file.Mode))
		}

		return nil
	},
}

var fsCpCmd = &cobra.Command{
	Use:   "cp [local-path] [remote-path]",
	Short: "Copy local file to remote agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		localPath := args[0]
		remotePath := args[1]

		content, err := os.ReadFile(localPath)
		if err != nil {
			return fmt.Errorf("read local file: %w", err)
		}

		_, err = cli.coreClient.WriteFile(context.Background(), &v1.WriteFileRequest{
			AgentId: agentID,
			Path:    remotePath,
			Content: content,
		})
		if err != nil {
			return err
		}

		fmt.Printf("✅ Successfully copied %s to %s on agent %s\n", localPath, remotePath, agentID)
		return nil
	},
}

var fsCatCmd = &cobra.Command{
	Use:   "cat [path]",
	Short: "Print contents of remote file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		path := args[0]

		resp, err := cli.coreClient.ReadFile(context.Background(), &v1.ReadFileRequest{
			AgentId: agentID,
			Path:    path,
		})
		if err != nil {
			return err
		}

		fmt.Print(string(resp.Content))
		return nil
	},
}

var fsFetchCmd = &cobra.Command{
	Use:   "fetch [remote-path] [local-path]",
	Short: "Download remote file from agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		remotePath := args[0]
		localPath := args[1]

		resp, err := cli.coreClient.ReadFile(context.Background(), &v1.ReadFileRequest{
			AgentId: agentID,
			Path:    remotePath,
		})
		if err != nil {
			return err
		}

		err = os.WriteFile(localPath, resp.Content, 0644)
		if err != nil {
			return fmt.Errorf("write local file: %w", err)
		}

		fmt.Printf("✅ Successfully downloaded %s from agent %s to %s\n", remotePath, agentID, localPath)
		return nil
	},
}

var fsMvCmd = &cobra.Command{
	Use:   "mv [source] [destination]",
	Short: "Move or rename file on remote agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		src := args[0]
		dest := args[1]

		_, err = cli.coreClient.MoveFile(context.Background(), &v1.MoveFileRequest{
			AgentId:    agentID,
			SourcePath: src,
			DestPath:   dest,
		})
		if err != nil {
			return err
		}

		fmt.Printf("✅ Successfully moved %s to %s on agent %s\n", src, dest, agentID)
		return nil
	},
}

var fsRmCmd = &cobra.Command{
	Use:   "rm [path]",
	Short: "Remove file or directory on remote agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		path := args[0]
		recursive, _ := cmd.Flags().GetBool("recursive")

		_, err = cli.coreClient.DeleteFile(context.Background(), &v1.DeleteFileRequest{
			AgentId:   agentID,
			Path:      path,
			Recursive: recursive,
		})
		if err != nil {
			return err
		}

		fmt.Printf("✅ Successfully removed %s on agent %s\n", path, agentID)
		return nil
	},
}

var fsMkdirCmd = &cobra.Command{
	Use:   "mkdir [path]",
	Short: "Create directory on remote agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		path := args[0]

		_, err = cli.coreClient.CreateDirectory(context.Background(), &v1.CreateDirectoryRequest{
			AgentId: agentID,
			Path:    path,
		})
		if err != nil {
			return err
		}

		fmt.Printf("✅ Successfully created directory %s on agent %s\n", path, agentID)
		return nil
	},
}

func init() {
	fsRmCmd.Flags().BoolP("recursive", "r", false, "Remove directories and their contents recursively")
}
