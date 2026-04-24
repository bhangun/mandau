package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	Short: "Copy local file or directory to remote agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		localPath := args[0]
		remotePath := args[1]
		recursive, _ := cmd.Flags().GetBool("recursive")

		info, err := os.Stat(localPath)
		if err != nil {
			// Local path not found, try remote-to-remote copy
			_, cpErr := cli.coreClient.CopyFile(context.Background(), &v1.CopyFileRequest{
				AgentId:    agentID,
				SourcePath: localPath,
				DestPath:   remotePath,
			})
			if cpErr != nil {
				return fmt.Errorf("local path: %w (and remote copy also failed: %v)", err, cpErr)
			}
			fmt.Printf("✅ Successfully copied %s to %s on agent %s (remote-to-remote)\n", localPath, remotePath, agentID)
			return nil
		}

		if info.IsDir() {
			if !recursive {
				return fmt.Errorf("%s is a directory (use -r to copy recursively)", localPath)
			}
			// If remote path ends with a slash or doesn't exist yet, it's a directory
			// In standard cp, if dst is a directory, it copies src into it (dst/src)
			if strings.HasSuffix(remotePath, "/") || strings.HasSuffix(remotePath, "\\") {
				remotePath = filepath.ToSlash(filepath.Join(remotePath, filepath.Base(localPath)))
			}
			return uploadDir(agentID, localPath, remotePath)
		}

		// Single file upload
		if strings.HasSuffix(remotePath, "/") || strings.HasSuffix(remotePath, "\\") {
			remotePath = filepath.ToSlash(filepath.Join(remotePath, filepath.Base(localPath)))
		}
		return uploadFile(agentID, localPath, remotePath)
	},
}

func uploadFile(agentID, localPath, remotePath string) error {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	_, err = cli.coreClient.WriteFile(context.Background(), &v1.WriteFileRequest{
		AgentId: agentID,
		Path:    remotePath,
		Content: content,
		Mode:    uint32(info.Mode()),
	})
	if err != nil {
		return err
	}

	fmt.Printf("✅ Successfully copied %s to %s on agent %s\n", localPath, remotePath, agentID)
	return nil
}

func uploadDir(agentID, localPath, remotePath string) error {
	fmt.Printf("📂 Uploading directory %s to %s...\n", localPath, remotePath)
	
	// Create the base directory on the remote
	_, _ = cli.coreClient.CreateDirectory(context.Background(), &v1.CreateDirectoryRequest{
		AgentId: agentID,
		Path:    remotePath,
	})

	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		targetPath := filepath.ToSlash(filepath.Join(remotePath, rel))

		if info.IsDir() {
			_, err := cli.coreClient.CreateDirectory(context.Background(), &v1.CreateDirectoryRequest{
				AgentId: agentID,
				Path:    targetPath,
			})
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, err = cli.coreClient.WriteFile(context.Background(), &v1.WriteFileRequest{
			AgentId: agentID,
			Path:    targetPath,
			Content: content,
			Mode:    uint32(info.Mode()),
		})
		if err != nil {
			return fmt.Errorf("upload %s: %w", rel, err)
		}

		return nil
	})

	if err == nil {
		fmt.Printf("✅ Successfully copied directory %s to %s on agent %s\n", localPath, remotePath, agentID)
	}
	return err
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
	Short: "Download remote file or directory from agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID, err := cli.resolveAgent(cmd)
		if err != nil {
			return err
		}

		remotePath := args[0]
		localPath := args[1]
		recursive, _ := cmd.Flags().GetBool("recursive")

		// First, try to read it as a file to see what it is
		resp, err := cli.coreClient.ReadFile(context.Background(), &v1.ReadFileRequest{
			AgentId: agentID,
			Path:    remotePath,
		})

		if err != nil {
			// If ReadFile fails, it might be a directory (since ReadFile fails on directories in our agent)
			// Or it might just not exist.
			if !recursive {
				return err
			}
			// Try as directory
			return downloadDir(agentID, remotePath, localPath)
		}

		if resp.Info.IsDir {
			if !recursive {
				return fmt.Errorf("%s is a directory (use -r to fetch recursively)", remotePath)
			}
			return downloadDir(agentID, remotePath, localPath)
		}

		// Single file download
		// If localPath is a directory, append the remote filename
		if info, err := os.Stat(localPath); err == nil && info.IsDir() {
			localPath = filepath.Join(localPath, filepath.Base(remotePath))
		}

		err = os.WriteFile(localPath, resp.Content, os.FileMode(resp.Info.Mode))
		if err != nil {
			return fmt.Errorf("write local file: %w", err)
		}

		fmt.Printf("✅ Successfully downloaded %s from agent %s to %s\n", remotePath, agentID, localPath)
		return nil
	},
}

func downloadDir(agentID, remotePath, localPath string) error {
	fmt.Printf("📂 Downloading directory %s from agent %s to %s...\n", remotePath, agentID, localPath)

	// Ensure local directory exists
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("create local directory: %w", err)
	}

	return fetchRecursive(agentID, remotePath, localPath)
}

func fetchRecursive(agentID, remotePath, localPath string) error {
	resp, err := cli.coreClient.ListFiles(context.Background(), &v1.ListFilesRequest{
		AgentId: agentID,
		Path:    remotePath,
	})
	if err != nil {
		return err
	}

	for _, file := range resp.Files {
		remoteFilePath := filepath.ToSlash(filepath.Join(remotePath, file.Name))
		localFilePath := filepath.Join(localPath, file.Name)

		if file.IsDir {
			if err := os.MkdirAll(localFilePath, 0755); err != nil {
				return err
			}
			if err := fetchRecursive(agentID, remoteFilePath, localFilePath); err != nil {
				return err
			}
		} else {
			fileResp, err := cli.coreClient.ReadFile(context.Background(), &v1.ReadFileRequest{
				AgentId: agentID,
				Path:    remoteFilePath,
			})
			if err != nil {
				fmt.Printf("⚠️ Warning: failed to download %s: %v\n", remoteFilePath, err)
				continue
			}

			if err := os.WriteFile(localFilePath, fileResp.Content, os.FileMode(file.Mode)); err != nil {
				return fmt.Errorf("write %s: %w", localFilePath, err)
			}
		}
	}

	return nil
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
	fsCpCmd.Flags().BoolP("recursive", "r", false, "Copy directories recursively")
	fsFetchCmd.Flags().BoolP("recursive", "r", false, "Download directories recursively")
}
