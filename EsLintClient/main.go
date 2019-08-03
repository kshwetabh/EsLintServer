package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	pb "github.com/ksfnu/eslint_server/EsLintClient/agent"
	"google.golang.org/grpc"
)

// Config object for various configuration information
type Config struct {
	WorkspacePath   string // location of workspace directory
	EslintServerURL string // Address of the NodeJS server component
}

// ESLintError struct
type ESLintError struct {
	FilePath string `json:"filePath"`
	Messages []struct {
		RuleID    string `json:"ruleId"`
		Severity  int    `json:"severity"`
		Message   string `json:"message"`
		Line      int    `json:"line"`
		Column    int    `json:"column"`
		NodeType  string `json:"nodeType"`
		EndLine   int    `json:"endLine,omitempty"`
		EndColumn int    `json:"endColumn,omitempty"`
		MessageID string `json:"messageId,omitempty"`
		Fix       struct {
			Range []int  `json:"range"`
			Text  string `json:"text"`
		} `json:"fix,omitempty"`
	} `json:"messages"`
	ErrorCount          int    `json:"errorCount"`
	WarningCount        int    `json:"warningCount"`
	FixableErrorCount   int    `json:"fixableErrorCount"`
	FixableWarningCount int    `json:"fixableWarningCount"`
	Source              string `json:"source"`
}

var severity = map[int]string{
	1: "Warning",
	2: "Error",
}

var config *Config
var watcher *fsnotify.Watcher

func init() {
	config = loadConfig()
}

func main() {
	// Set up a connection to the server.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("")
	color.HiGreen("Connecting to EsLint Server [ %s ] ...", config.EslintServerURL)

	conn, err := grpc.DialContext(ctx, config.EslintServerURL, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		color.HiRed("************ Could not connect to the EsLint Server. Please make sure you are on Infor network and \"eslintServerURL\" is configured correctly in config.json file. ************")
		panic(err)
	}
	defer conn.Close()

	color.HiGreen("Successfully connected to the EsLint Server")

	var waitgroup sync.WaitGroup
	waitgroup.Add(1)
	monitorFileSystemForChanges(conn)
	waitgroup.Wait()
}

func sendFileToServer(fileName string, conn *grpc.ClientConn) {

	// Send only js files for scanning
	if !strings.HasSuffix(fileName, ".js") {
		return
	}
	data, err := ioutil.ReadFile(fileName)
	check(err)
	fmt.Printf("Sending data to server: %s\n", string(data))

	currrentTime := time.Now().Format("15:04:05")

	client := pb.NewEsLintServiceClient(conn)
	req := &pb.EsLintRequest{FileContent: string(data), FileName: fileName}

	resp, err := client.LintFile(context.Background(), req)
	if err != nil {
		color.Magenta("Error when calling LintFile: %s", err)
	}

	// Print result once response is available
	// Print ESLint warnings on console
	if len(resp.Errors) > 0 {
		lintErrors := []ESLintError{}
		err = json.Unmarshal([]byte(resp.Errors), &lintErrors)
		if err != nil {
			color.Magenta("Error occurred while parsing server response: [%v]", err)
			return
		}
		messages := lintErrors[0].Messages

		if len(messages) > 0 {
			fmt.Println()
			color.HiCyan(fileName)

			for _, msg := range messages {
				paddedLintMsg := padErrorMessage(msg.Message)
				paddedLineCol := padLineColumn(strconv.Itoa(msg.Line), strconv.Itoa(msg.Column))
				paddedSev := padSevirity(severity[msg.Severity])

				if msg.Severity == 2 {
					fmt.Fprintf(color.Output, " %s\t%s\t%s\t%s\t\n", color.HiRedString(paddedLineCol), color.HiRedString(paddedSev), paddedLintMsg, msg.RuleID)
				} else {
					fmt.Fprintf(color.Output, " %s\t%s\t%s\t%s\t\n", color.YellowString(paddedLineCol), color.HiYellowString(paddedSev), paddedLintMsg, msg.RuleID)
				}
			}
			fmt.Fprintf(color.Output, "\n %s Errors, %s Warnings [%s]\n\n", color.HiRedString(strconv.Itoa(lintErrors[0].ErrorCount)), color.HiYellowString(strconv.Itoa(lintErrors[0].WarningCount)), color.HiMagentaString(currrentTime))
		} else {
			// Otherwise print clean msg
			color.HiGreen("**** Clean ****")
		}
	} else if resp.Errors == "" {
		// Otherwise print clean msg
		color.HiGreen("**** Clean ****")
	}
}

func padErrorMessage(msg string) string {
	//90 is the size of the maximum length error message we use in HMS
	return fmt.Sprintf("%-90v", msg)
}
func padLineColumn(line, col string) string {
	paddedLineCol := fmt.Sprintf("%s:%s", line, col)
	return fmt.Sprintf("%-8s", paddedLineCol)
}
func padSevirity(msg string) string {
	return fmt.Sprintf("%5v", msg)
}

func check(e error) {
	if e != nil {
		color.Magenta("Error occurred: %v\n", e)
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
		color.Magenta("Error occurred adding directories to recursive watch: %v", err)
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
				color.Magenta("error: %v", err)
			}
		}
	}()

	err := watcher.Add(config.WorkspacePath)
	color.HiCyan("\n------- Monitoring directory [%s] for changes -------\n\n", config.WorkspacePath)
	if err != nil {
		color.Magenta("Error occurred: %v", err)
	}
	<-done
}

// watchDir gets run as a walk func, searching for directories to add watchers to
func watchDir(path string, fi os.FileInfo, err error) error {
	// Since fsnotify can watch all the files in a directory, a more efficient solution would have been to add watchers only to each nested directory
	// The issue with this approach is then watcher fires duplicate modification events (one for directory and one for file modification) causing two requests
	// to be fired from the client on each file save. Watching only for the files (and not directories) reduces duplicate calls but does not completely makes it go away.
	if !fi.Mode().IsDir() {
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
