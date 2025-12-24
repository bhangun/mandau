package main

import (
	"context"
	"fmt"
	"io"
	"log"

	v1 "github.com/bhangun/mandau/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// Example: Deploy a complete web service

	// Connect to Mandau Core
	creds, err := credentials.NewClientTLSFromFile("ca.crt", "")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.Dial("localhost:8443", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := v1.NewServiceDeploymentServiceClient(conn)

	// Deploy a Node.js application with nginx reverse proxy and SSL
	req := &v1.DeployWebServiceRequest{
		AgentId:     "agent-001",
		Name:        "my-nodejs-app",
		Description: "My Node.js Application",
		Domain:      "app.example.com",
		Port:        3000,
		Command:     "/usr/bin/node /opt/myapp/server.js",
		WorkingDir:  "/opt/myapp",
		User:        "nodejs",
		Ssl:         true,
		Environment: map[string]string{
			"NODE_ENV": "production",
			"PORT":     "3000",
		},
	}

	stream, err := client.DeployWebService(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}

	// Watch deployment progress
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("[%s] %s\n", event.State, event.Message)

		if event.Error != "" {
			fmt.Printf("Error: %s\n", event.Error)
		}
	}

	fmt.Println("Deployment complete!")
}
