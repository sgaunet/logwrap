package executor_test

import (
	"fmt"
	"log"

	"github.com/sgaunet/logwrap/pkg/executor"
)

// ExampleNew demonstrates creating an executor for a simple command.
func ExampleNew() {
	exec, err := executor.New([]string{"echo", "hello"})
	if err != nil {
		log.Fatal(err)
	}
	defer exec.Cleanup()

	if err := exec.Start(); err != nil {
		log.Fatal(err)
	}

	if err := exec.Wait(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("exit code: %d\n", exec.GetExitCode())
	// Output: exit code: 0
}

// ExampleExecutor_GetExitCode demonstrates capturing the exit code of a
// failed command.
func ExampleExecutor_GetExitCode() {
	exec, err := executor.New([]string{"sh", "-c", "exit 42"})
	if err != nil {
		log.Fatal(err)
	}
	defer exec.Cleanup()

	if err := exec.Start(); err != nil {
		log.Fatal(err)
	}

	// Wait returns nil even for non-zero exit codes; the exit code
	// is available via GetExitCode.
	_ = exec.Wait()

	fmt.Printf("exit code: %d\n", exec.GetExitCode())
	// Output: exit code: 42
}
