package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/fsnotify/fsnotify"
	pb "github.com/ksfnu/eslint_server/agents/agent"
	"google.golang.org/grpc"
)

var workspacePath = "C:/Users/ksfnu/eclipseWorkspace/workspace38_Photon/Frontend/src/main/webapp/app"

const (
	address     = "localhost:4040" // Address of the NodeJS server component
	defaultName = "world"
)

type server struct{}

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	monitorFileSystemForChanges(conn)
}

func monitorFileSystemForChanges(conn *grpc.ClientConn) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					sendFileToServer(event.Name, conn)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error: ", err)
			}
		}
	}()

	err = watcher.Add(workspacePath)
	fmt.Printf("\n------- Monitoring directory [%s] for changes -------\n\n", workspacePath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

func sendFileToServer(fileName string, conn *grpc.ClientConn) {
	data, err := ioutil.ReadFile(fileName)
	check(err)
	// fmt.Printf("Sending data to server: %s\n", string(data))

	client := pb.NewEsLintServiceClient(conn)
	req := &pb.EsLintRequest{FileContent: string(data)}

	resp, err := client.LintFile(context.Background(), req)
	if err != nil {
		log.Fatalf("Error when calling LintFile: %s", err)
	}
	// Print ESLint warnings on console
	// log.Println("Response from server:")
	log.Println(resp.Errors)
}

func check(e error) {
	if e != nil {
		log.Println(e)
	}
}
