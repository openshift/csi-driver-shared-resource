package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/openshift/csi-driver-shared-resource/pkg/config"
)

// watchForConfigChangesEnv used for testing "exit(0)" behavior, it carries the configuration file
// used during testing, and enables the watching for changes.
const watchForConfigChangesEnv = "WATCH_FOR_CONFIG_CHANGES"

// TestMain_watchForConfigChanges runs watchForConfigChanges using the configuration file path passed
// via environment variable.
func TestMain_watchForConfigChanges(t *testing.T) {
	// when the environment variable is empty, just skiping the test run
	cfgFilePath := os.Getenv(watchForConfigChangesEnv)
	if cfgFilePath == "" {
		return
	}

	t.Logf("Instantiating configuration manager to watch '%s'", cfgFilePath)
	mgr := config.NewManager(cfgFilePath)
	_, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("should be able to load temporary configuration, error: '%#v'", err)
	}

	watchForConfigChanges(mgr)
}

// backgroundRun_watchForConfigChanges executes "go test ..." with specific function in order to run
// watchForConfigChanges with configuration file informed.
func backgroundRun_watchForConfigChanges(t *testing.T, cfgFilePath string, doneCh chan struct{}) {
	// instantiating binary created by "go test"  with timeout, and linking STDOUT/STDERR with the
	// current test instance
	args := []string{"-test.v", "-test.timeout=15s", "-test.run=TestMain_watchForConfigChanges"}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", watchForConfigChangesEnv, cfgFilePath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	t.Logf("Running test: '%s'", cmd.String())
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Logf("should exit(0) upon configuration change, error: '%#v'", err)
		t.Fail()
	}
	close(doneCh)
}

// TestMain_exitWhenConfigChanges make sure the function watchForConfigChanges exits when the actual
// configuration file has changed.
func TestMain_exitWhenConfigChanges(t *testing.T) {
	// original configuration file path, using the same than testing configs
	cfgFilePath := "../../test/config/config.yaml"

	// creating a temporary file, making sure it's deleted when test is done
	tmpFile, err := os.CreateTemp("/tmp", "csi-driver-config-")
	if err != nil {
		t.Fatalf("unable to create a temporary file: '%#v'", err)
	}
	defer os.Remove(tmpFile.Name())
	t.Logf("Created a temporary file at '%s'", tmpFile.Name())

	// copying original configuration file contents over the temporary file
	in, err := os.ReadFile(cfgFilePath)
	if err != nil {
		t.Fatalf("unable to read configuration file '%s', error: '%#v'", cfgFilePath, err)
	}
	if err = os.WriteFile(tmpFile.Name(), in, 0644); err != nil {
		t.Fatalf("unable to write to temporary file '%s', error: '%#v'", tmpFile.Name(), err)
	}

	// executing watchForConfigChanges in the background, and sharing a channel to be notified when
	// the background routine is done
	doneCh := make(chan struct{})
	go backgroundRun_watchForConfigChanges(t, tmpFile.Name(), doneCh)

	// graceful waiting, then modifying the configuration file contents under watch, it should
	// trigger the routine checking for modifications
	time.Sleep(3 * time.Second)
	if err = os.WriteFile(tmpFile.Name(), []byte{}, 0644); err != nil {
		t.Fatalf("unable to write to temporary file '%s', error: '%#v'", tmpFile.Name(), err)
	}

	// blocking the testing execution until watch is done
	<-doneCh
}
