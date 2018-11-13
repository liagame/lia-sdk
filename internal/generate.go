package internal

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"github.com/liagame/lia-SDK"
	"github.com/liagame/lia-SDK/internal/config"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type GameFlags struct {
	GameSeed   int
	MapSeed    int
	Port       int
	MapPath    string
	ReplayPath string
	ConfigPath string
	DebugBots  []int
}

func GenerateGame(bot1Dir string, bot2Dir string, gameFlags *GameFlags) {
	bot1Debug := contains(gameFlags.DebugBots, 1)
	uidBot1 := getBotUid(bot1Debug)

	bot2Debug := contains(gameFlags.DebugBots, 2)
	uidBot2 := getBotUid(bot2Debug)

	// Set config path if not provided
	if gameFlags.ConfigPath == "" {
		gameFlags.ConfigPath = filepath.Join(gameFlags.ConfigPath, "game-config.json")
		if len(gameFlags.DebugBots) > 0 {
			gameFlags.ConfigPath = strings.Replace(gameFlags.ConfigPath, ".json", "-debug.json", 1)
		}
	}
	// Set port if not already set
	if gameFlags.Port == 0 {
		gameFlags.Port = config.Cfg.GamePort
	}

	// Create channel that will listen to results
	// from game generator and both bots
	result := make(chan error)

	cmdBot1 := &CommandRef{}
	cmdBot2 := &CommandRef{}
	cmdGameGenerator := &CommandRef{}

	generatorStarted := make(chan bool)

	// Run game-generator
	go func() {
		fmt.Printf("Running game generator\n")
		bot1Name := parseBotName(bot1Dir)
		bot2Name := parseBotName(bot2Dir)
		err := runGameGenerator(generatorStarted, cmdGameGenerator, gameFlags, bot1Name, bot2Name, uidBot1, uidBot2)
		cmdGameGenerator.cmd = nil
		result <- err
	}()

	// Wait until game generator has started
	<-generatorStarted

	// Run bots
	runBotWrapper := func(cmdBot *CommandRef, botDir, botUid string) {
		fmt.Printf("Running bot %s\n", botDir)
		err := runBot(cmdBot, botDir, botUid, gameFlags.Port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		cmdBot.cmd = nil
	}
	if !bot1Debug {
		go runBotWrapper(cmdBot1, bot1Dir, uidBot1)
	}
	if !bot2Debug {
		go runBotWrapper(cmdBot2, bot2Dir, uidBot2)
	}

	// Wait for game engine to finish
	err := <-result
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate game\n %s\n", err)
		defer os.Exit(lia_SDK.FailedToGenerateGame)
	}

	// Attempt to kill the process to prevent daemons
	killProcess(cmdBot1)
	killProcess(cmdBot2)

	// Wait for outputs to appear on the console (nicer way to fix this?)
	time.Sleep(time.Millisecond * 100)
}

func parseBotName(botDir string) string {
	if config.OperatingSystem == "windows" {
		split := strings.Split(botDir, "\\")
		return split[len(split)-1]
	} else {
		split := strings.Split(botDir, "/")
		return split[len(split)-1]
	}
}

func contains(slice []int, e int) bool {
	for _, e2 := range slice {
		if e == e2 {
			return true
		}
	}
	return false
}

func getBotUid(debug bool) string {
	if debug {
		return ""
	}
	uid, err := generateUuid()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate uid. %s", err)
		os.Exit(lia_SDK.Generic)
	}
	return uid
}

func killProcess(cmdRef *CommandRef) {
	if cmdRef.cmd != nil {
		if err := cmdRef.cmd.Process.Kill(); err != nil {
			// Ignore, no valuable information
		}
	}
}

type CommandRef struct {
	cmd *exec.Cmd
}

func runBot(cmdRef *CommandRef, name, uid string, port int) error {
	runScriptName := "run.sh"

	botDir := filepath.Join(config.PathToBots, name)

	var cmd *exec.Cmd
	if config.OperatingSystem == "windows" {
		cmd = exec.Command(config.Cfg.PathToBash, runScriptName, strconv.Itoa(port), uid)
	} else {
		cmd = exec.Command("/bin/bash", runScriptName, strconv.Itoa(port), uid)
	}

	cmdRef.cmd = cmd
	cmd.Dir = botDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil && !strings.Contains(err.Error(), "signal=killed") {
		fmt.Fprintf(os.Stderr, "Running bot %s failed.\n", name)
		return err
	}

	return nil
}

func runGameGenerator(started chan bool, cmdRef *CommandRef, gameFlags *GameFlags, nameBot1, nameBot2, uidBot1, uidBot2 string) error {
	cmd := exec.Command(
		"java", "-jar", "game-engine.jar",
		"-g", fmt.Sprint(gameFlags.GameSeed),
		"-m", fmt.Sprint(gameFlags.MapSeed),
		"-p", fmt.Sprint(gameFlags.Port),
	)
	cmdRef.cmd = cmd

	// Append string flags if they are not empty
	if len(gameFlags.MapPath) > 0 {
		cmd.Args = append(cmd.Args, "-M", gameFlags.MapPath)
	}
	if len(gameFlags.ReplayPath) > 0 {
		cmd.Args = append(cmd.Args, "-r", gameFlags.ReplayPath)
	}
	if len(gameFlags.ConfigPath) > 0 {
		cmd.Args = append(cmd.Args, "-c", gameFlags.ConfigPath)
	}
	// Append bot1 and his uid
	cmd.Args = append(cmd.Args, nameBot1, uidBot1)
	// Append bot2 and his uid
	cmd.Args = append(cmd.Args, nameBot2, uidBot2)

	cmd.Dir = config.PathToData

	// Get pipes for stdout and stderr
	stdoutIn, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stdout pipe for game generator.")
		return err
	}
	stderrIn, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stdin pipe for game generator.")
		return err
	}
	// Create multi writer that will pass result to stdout, stderr and buffers
	var stdoutBuf, stderrBuf bytes.Buffer
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)

	// Set the data flow from command to writers
	var errStdout, errStderr error
	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
	}()

	go func() {
		_, errStderr = io.Copy(stderr, stderrIn)
	}()

	// Send true to started channel when game generator outputs something
	// (means that websocket server is prepared)
	go func() {
		for {
			if stdoutBuf.Len() > 0 || stderrBuf.Len() > 0 {
				started <- true
				return
			}
			time.Sleep(time.Millisecond * 20)
		}
	}()

	// Run game generator
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Game generator failed.")
		return err
	}

	if errStdout != nil {
		fmt.Fprintf(os.Stderr, "Failed to capture stdout.")
		return err
	}
	if errStderr != nil {
		fmt.Fprintf(os.Stderr, "Failed to capture stderr.")
		return err
	}

	return nil
}

func generateUuid() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get random number.")
		return "", err
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid, nil
}
