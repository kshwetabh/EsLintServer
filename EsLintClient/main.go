package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/apsdehal/go-logger"
	"github.com/fsnotify/fsnotify"
	pb "github.com/ksfnu/eslint_server/EsLintClient/agent"
	"google.golang.org/grpc"
)

var workspacePath = "C:/Users/ksfnu/eclipseWorkspace/workspace38_Photon/Frontend/src/main/webapp/app"

type Config struct {
	WorkspacePath   string // location of workspace directory
	EslintServerURL string // Address of the NodeJS server component
}

var config *Config
var watcher *fsnotify.Watcher
var log *logger.Logger

func init() {
	config = loadConfig()
}

func main() {
	goLogger, err := logger.New("main", 1, os.Stdout)
	log = goLogger

	if err != nil {
		panic(err) // Check for error
	}

	// Set up a connection to the server.
	conn, err := grpc.Dial(config.EslintServerURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	var waitgroup sync.WaitGroup
	waitgroup.Add(1)
	monitorFileSystemForChanges(conn)
	waitgroup.Wait()
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
	// log.Println("Response from server:")
	// Print ESLint warnings on console
	// log.Println("---------------------------------")
	if len(resp.Errors) > 0 {
		log.Critical(resp.Errors)
		return
	}
	log.Notice("*** Clean ***")
}

func check(e error) {
	if e != nil {
		log.ErrorF("Error occurred: %v\n", e)
	}
}

func monitorFileSystemForChanges(conn *grpc.ClientConn) {
	watcher, _ = fsnotify.NewWatcher()
	// if err != nil {
	// 	log.FatalF("Error occurred while initializing fsnotify: %v", err)
	// }
	defer watcher.Close()

	// recursively traverse the filesystem starting workspacePath root and add all subdirectories to watch
	if err := filepath.Walk(config.WorkspacePath, watchDir); err != nil {
		log.FatalF("Error occurred adding directories to recursive watch: %v", err)
	}

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
				log.ErrorF("error: %v", err)
			}
		}
	}()

	err := watcher.Add(config.WorkspacePath)
	log.InfoF("\n------- Monitoring directory [%s] for changes -------\n\n", config.WorkspacePath)
	if err != nil {
		log.FatalF("Error occurred: %v", err)
	}
	<-done
}

// watchDir gets run as a walk func, searching for directories to add watchers to
func watchDir(path string, fi os.FileInfo, err error) error {
	// since fsnotify can watch all the files in a directory, watchers only need
	// to be added to each nested directory
	if fi.Mode().IsDir() {
		return watcher.Add(path)
	}
	return nil
}

func loadConfig() *Config {
	configFile, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	defer configFile.Close()

	cfg := &Config{}
	err = json.NewDecoder(configFile).Decode(cfg)
	if err != nil {
		panic("parsing config: " + err.Error())
	}
	return cfg
}
