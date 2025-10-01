package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"
)

////////////////////////////////////////////////////////////////////////////////////////
// Run
////////////////////////////////////////////////////////////////////////////////////////

func run(out io.Writer, path string, routine int) (failExportInvariants bool, err error) {
	localLog := consoleLogger(out)

	home := "/" + strconv.Itoa(routine)
	localLog.Info().Str("path", path).Msgf("Loading regression test")

	// clear data directory
	localLog.Debug().Msg("Clearing data directory")
	mayanodePath := filepath.Join(home, ".mayanode")
	cmdOut, err := exec.Command("rm", "-rf", mayanodePath).CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("failed to clear data directory")
	}

	// use same environment for all commands
	env := []string{
		"HOME=" + home,
		"SIGNER_NAME=mayachain",
		"SIGNER_PASSWD=password",
		"CHAIN_HOME_FOLDER=" + mayanodePath,
		"MAYA_TENDERMINT_INSTRUMENTATION_PROMETHEUS=false",
		// block time should be short, but all consecutive checks must complete within timeout
		fmt.Sprintf("MAYA_TENDERMINT_CONSENSUS_TIMEOUT_COMMIT=%s", time.Second*3/2*getTimeFactor()),
		// all ports will be offset by the routine number
		fmt.Sprintf("MAYA_COSMOS_API_ADDRESS=tcp://0.0.0.0:%d", 1317+routine),
		fmt.Sprintf("MAYA_TENDERMINT_RPC_LISTEN_ADDRESS=tcp://0.0.0.0:%d", 26657+routine),
		fmt.Sprintf("MAYA_TENDERMINT_P2P_LISTEN_ADDRESS=tcp://0.0.0.0:%d", 27000+routine),
		"CREATE_BLOCK_PORT=" + strconv.Itoa(8080+routine),
		"GOCOVERDIR=/mnt/coverage",
	}

	// if DEBUG is set also output mayanode debug logs
	if os.Getenv("DEBUG") != "" {
		env = append(env, "MAYA_TENDERMINT_LOG_LEVEL=debug")
		env = append(env, "RUST_BACKTRACE=1")
	}

	// init chain with dog mnemonic
	localLog.Debug().Msg("Initializing chain")
	cmd := exec.Command("mayanode", "init", "local", "--chain-id", "mayachain", "--recover")
	cmd.Stdin = bytes.NewBufferString(dogMnemonic + "\n")
	cmd.Env = env
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("failed to initialize chain")
	}

	// init keys with dog mnemonic
	localLog.Debug().Msg("Initializing keys")
	cmd = exec.Command("mayanode", "keys", "--keyring-backend=file", "add", "--recover", "mayachain")
	cmd.Stdin = bytes.NewBufferString(dogMnemonic + "\npassword\npassword\n")
	cmd.Env = env
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("failed to initialize keys")
	}

	// init chain
	localLog.Debug().Msg("Initializing chain")
	cmd = exec.Command("mayanode", "init", "local", "--chain-id", "mayachain", "-o")
	cmd.Env = env
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("failed to initialize chain")
	}

	// create routine local state (used later by custom template functions in operations)
	nativeTxIDsMu.Lock()
	nativeTxIDs[routine] = []string{}
	nativeTxIDsMu.Unlock()
	tmpls := template.Must(templates.Clone())

	// ensure no naming collisions
	if tmpls.Lookup(filepath.Base(path)) != nil {
		log.Fatal().Msgf("test name collision: %s", filepath.Base(path))
	}

	ops, opLines, env, failExportInvariants := parseOps(localLog, path, tmpls, env)

	// warn if no operations found
	if len(ops) == 0 {
		err = errors.New("no operations found")
		localLog.Err(err).Msg("")
		return false, err
	}

	localLog.Info().Str("path", path).Int("blocks", blockCount(ops)).Msgf("Running regression test")

	// clear block directory
	blocksPath := filepath.Join("/mnt/blocks", strings.TrimPrefix(path, "suites/"))
	blocksPath = strings.TrimSuffix(blocksPath, ".yaml")
	_ = os.RemoveAll(blocksPath)

	// execute all state operations
	stateOpCount := 0
	for i, op := range ops {
		if _, ok := op.(*OpState); ok {
			localLog.Info().Int("line", opLines[i]).Msgf(">>> [%d] %s", i+1, op.OpType())
			err = op.Execute(out, path, opLines[i], routine, cmd.Process, nil)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to execute state operation")
			}
			stateOpCount++
		}
	}
	ops = ops[stateOpCount:]
	opLines = opLines[stateOpCount:]

	// validate genesis
	localLog.Debug().Msg("Validating genesis")
	cmd = exec.Command("mayanode", "validate-genesis")
	cmd.Env = env
	cmdOut, err = cmd.CombinedOutput()
	if err != nil {
		// dump the genesis
		fmt.Println(ColorPurple + "Genesis:" + ColorReset)
		var f *os.File
		f, err = os.OpenFile(filepath.Join(home, ".mayanode/config/genesis.json"), os.O_RDWR, 0o644)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to open genesis file")
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		f.Close()

		// dump error and exit
		fmt.Println(string(cmdOut))
		log.Fatal().Err(err).Msg("genesis validation failed")
	}

	// render config
	localLog.Debug().Msg("Rendering config")
	cmd = exec.Command("mayanode", "render-config")
	cmd.Env = env
	err = cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to render config")
	}

	// overwrite private validator key
	localLog.Debug().Msg("Overwriting private validator key")
	keyPath := filepath.Join(home, ".mayanode/config/priv_validator_key.json")
	cmd = exec.Command("cp", "/mnt/priv_validator_key.json", keyPath)
	err = cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to overwrite private validator key")
	}

	logLevel := "info"
	switch os.Getenv("DEBUG") {
	case "trace", "debug", "info", "warn", "error", "fatal", "panic":
		logLevel = os.Getenv("DEBUG")
	}

	// setup process io
	mayanode := exec.Command("/regtest/cover-mayanode", "--log_level", logLevel, "start")
	mayanode.Env = env

	stderr, err := mayanode.StderrPipe()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to setup mayanode stderr")
	}
	stderrScanner := bufio.NewScanner(stderr)
	stderrLines := make(chan string, 100)
	go func() {
		for stderrScanner.Scan() {
			stderrLines <- stderrScanner.Text()
		}
	}()
	if os.Getenv("DEBUG") != "" {
		mayanode.Stdout = os.Stdout
		mayanode.Stderr = os.Stderr
	}

	localLog.Debug().Msg("Clearing data directory")

	// start mayanode process
	localLog.Debug().Str("ENV", strings.Join(env, "; ")).Msg("Starting mayanode")

	err = mayanode.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start mayanode")
	}

	// wait for mayanode to listen on block creation port
	time.Sleep(time.Second)
	for i := 0; ; i++ {
		if i%100 == 0 {
			localLog.Debug().Msg("Waiting for mayanode to listen")
		}
		time.Sleep(100 * time.Millisecond)
		var conn net.Conn
		conn, err = net.Dial("tcp", fmt.Sprintf("localhost:%d", 8080+routine))
		if err == nil {
			conn.Close()
			break
		}
	}

	// run the operations
	var returnErr error
	localLog.Info().Msgf("Executing %d operations", len(ops))
	for i, op := range ops {

		// prefetch sequences of check operations
		if os.Getenv("AUTO_UPDATE") == "" && op.OpType() == "check" && ops[i-1].OpType() != "check" {
			wg := sync.WaitGroup{}
			for j := i; j < len(ops); j++ {
				if ops[j].OpType() != "check" {
					break
				}
				wg.Add(1)
				go func(j int) {
					defer wg.Done()
					// trunk-ignore(golangci-lint/forcetypeassert)
					ops[j].(*OpCheck).prefetch(routine)
				}(j)
			}
			wg.Wait()
		}

		localLog.Info().Int("line", opLines[i]).Msgf(">>> [%d] %s", stateOpCount+i+1, op.OpType())
		returnErr = op.Execute(out, path, opLines[i], routine, mayanode.Process, stderrLines)
		if returnErr != nil {
			localLog.Error().Err(returnErr).
				Int("line", opLines[i]).
				Int("op", stateOpCount+i+1).
				Str("type", op.OpType()).
				Str("path", path).
				Msg("operation failed")
			dumpLogs(out, stderrLines)
			break
		}

		// if in AUTO_UPDATE and the operation was a check, re-render future operations
		if os.Getenv("AUTO_UPDATE") != "" && op.OpType() == "check" {
			newOps, _, _, _ := parseOps(localLog, path, tmpls, env)
			remainingOps := len(ops) - i - 1
			newOps = newOps[len(newOps)-remainingOps:]
			for j, newOp := range newOps {
				ops[i+j+1] = newOp
			}
		}
	}

	// log success
	if returnErr == nil {
		localLog.Info().Msg("All operations succeeded")
	}

	// stop mayanode process
	localLog.Debug().Msg("Stopping mayanode")
	err = mayanode.Process.Signal(syscall.SIGUSR1)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to stop mayanode")
	}

	// wait for process to exit
	_, err = mayanode.Process.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to wait for mayanode")
	}

	// if failed and debug enabled restart to allow inspection
	if returnErr != nil && os.Getenv("DEBUG") != "" {

		// remove validator key (otherwise mayanode will hang in begin block)
		localLog.Debug().Msg("Removing validator key")
		cmd = exec.Command("rm", keyPath)
		cmdOut, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Println(string(cmdOut))
			log.Fatal().Err(err).Msg("failed to remove validator key")
		}

		// restart mayanode
		localLog.Debug().Msg("Restarting mayanode")
		mayanode = exec.Command("mayanode", "--log_level", logLevel, "start")
		mayanode.Env = env
		mayanode.Stdout = os.Stdout
		mayanode.Stderr = os.Stderr
		err = mayanode.Start()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to restart mayanode")
		}

		// wait for mayanode
		localLog.Debug().Msg("Waiting for mayanode")
		_, err = mayanode.Process.Wait()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to wait for mayanode")
		}
	}

	return failExportInvariants, returnErr
}
