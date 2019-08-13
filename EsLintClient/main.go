package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
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

var severity = map[int]string{
	1: "Warning",
	2: "Error",
}

var config *Config
var watcher *fsnotify.Watcher
var reportFlag *bool

func main() {

	reportFlag = flag.Bool("report", false, "Scans all javascript files in the workspace and generates report.csv file.\nWarning: if configured with remote EsLint server this might take long time depending on the number of files in the workspace.\n")
	serverArg := flag.String("server", "", "URL of the EsLint Server, defaults to 'eslintServerURL' value configured in config.json\n")
	pathArg := flag.String("src", "", "Path to the source code directory, defaults to 'workspacePath' value configured in config.json\n")
	flag.Parse()

	var err error
	if *pathArg == "" || *serverArg == "" {
		config, err = loadConfig()

		if err != nil {
			if *pathArg == "" {
				panic("\nInvalid configuration. Could not get the source directory. Either configure 'workspacePath' in config.json file or pass it on command line argument with -src option. Run EsLintClient --help for more details.")
			}
			if *serverArg == "" {
				panic("\nInvalid configuration. Could not get the Server URL. Either configure 'eslintServerURL' in config.json file or pass it on command line argument with -server option. Run EsLintClient --help for more details.")
			}
		}
	}

	// Initialize cfg if loadConfig() returned any error
	if config == nil {
		config = &Config{}
	}

	if *pathArg != "" {
		config.WorkspacePath = *pathArg
	}
	if *serverArg != "" {
		config.EslintServerURL = *serverArg
	}

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

	// if reportFlag argument passed on command line, then send all js files in workspace for scanning.
	//TODO: Note that this is pretty network intensive and might crash client/server.
	if *reportFlag {
		fmt.Println("So you want to scan all the files in the workspace ?")
		scanAllFilesInWorkspace(conn)
	} else {
		var waitgroup sync.WaitGroup
		waitgroup.Add(1)
		monitorFileSystemForChanges(conn)
		waitgroup.Wait()
	}
}

func sendFileToServer(fileName string, conn *grpc.ClientConn) {

	// Send only js files for scanning
	if !strings.HasSuffix(fileName, ".js") {
		return
	}
	data, err := ioutil.ReadFile(fileName)
	checkError("Could not read file to send to server", err)
	// fmt.Printf("Sending data to server: %s\n", string(data))

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

		fmt.Println()
		color.HiCyan(fileName)

		if len(messages) > 0 {
			for _, msg := range messages {
				if *reportFlag {
					// save this into the database or CSV
					saveToCSV(msg, fileName)
				} else {
					printOnConsole(msg)
				}
			}
		}
		fmt.Fprintf(color.Output, "\n %s Errors, %s Warnings [%s]\n\n", color.HiRedString(strconv.Itoa(lintErrors[0].ErrorCount)), color.HiYellowString(strconv.Itoa(lintErrors[0].WarningCount)), color.HiMagentaString(currrentTime))
	} else if resp.Errors == "" {
		// Otherwise print clean msg
		color.HiGreen("**** Clean ****")
	}
}

func saveToCSV(msg Message, fileName string) {
	// file, err := os.Create("report.csv")
	// If the file doesn't exist, create it, or append to the file
	file, err := os.OpenFile("report.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	checkError("Cannot create file", err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	//filename, row, col, type, msg, ruleid
	row := []string{fileName, strconv.Itoa(msg.Line), strconv.Itoa(msg.Column), strconv.Itoa(msg.Severity), msg.Message, msg.RuleID}
	err = writer.Write(row)
	checkError("Cannot write to file", err)
}

func printOnConsole(msg Message) {
	paddedLintMsg := padErrorMessage(msg.Message)
	paddedLineCol := padLineColumn(strconv.Itoa(msg.Line), strconv.Itoa(msg.Column))
	paddedSev := padSevirity(severity[msg.Severity])

	if msg.Severity == 2 {
		fmt.Fprintf(color.Output, " %s\t%s\t%s\t%s\t\n", color.HiRedString(paddedLineCol), color.HiRedString(paddedSev), paddedLintMsg, msg.RuleID)
	} else {
		fmt.Fprintf(color.Output, " %s\t%s\t%s\t%s\t\n", color.YellowString(paddedLineCol), color.HiYellowString(paddedSev), paddedLintMsg, msg.RuleID)
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

func checkError(message string, err error) {
	if err != nil {
		color.Magenta("%s: %v\n", message, err)
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
		err := watcher.Add(path)
		if err != nil {
			color.Magenta("Error occurred watching filesystem: %v", err)
			panic("")
		}
		return err
	}
	return nil
}

func scanAllFilesInWorkspace(conn *grpc.ClientConn) {
	// recursively traverse the filesystem starting workspacePath root and send the files to server for scanning
	err := filepath.Walk(config.WorkspacePath, func(path string, info os.FileInfo, err error) error {
		// fmt.Println("Sending file")
		sendFileToServer(path, conn)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func loadConfig() (*Config, error) {
	configFile, err := os.Open("config.json")
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	cfg := &Config{}
	err = json.NewDecoder(configFile).Decode(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
