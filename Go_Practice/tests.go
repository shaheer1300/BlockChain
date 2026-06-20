package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	fmt.Println("=== Go Environment Verification ===\n")

	// Check Go version
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("OS: %s\n", runtime.GOOS)
	fmt.Printf("Architecture: %s\n", runtime.GOARCH)
	fmt.Printf("Number of CPUs: %d\n\n", runtime.NumCPU())

	// Test basic operations
	fmt.Println("=== Testing Basic Operations ===")
	testArithmetic()
	testStringOperations()
	testSliceOperations()
	testMapOperations()
	testGoroutines()
	testDeferredExecution()

	fmt.Println("\n✓ All tests passed! Go is working properly.")
}

func testArithmetic() {
	fmt.Print("Arithmetic: ")
	result := (10 + 5) * 2 / 3
	fmt.Printf("(10+5)*2/3 = %d ✓\n", result)
}

func testStringOperations() {
	fmt.Print("String Operations: ")
	s := "Hello" + " " + "Go"
	fmt.Printf("\"%s\" has %d characters ✓\n", s, len(s))
}

func testSliceOperations() {
	fmt.Print("Slice Operations: ")
	slice := []int{1, 2, 3, 4, 5}
	sum := 0
	for _, v := range slice {
		sum += v
	}
	fmt.Printf("Sum of %v = %d ✓\n", slice, sum)
}

func testMapOperations() {
	fmt.Print("Map Operations: ")
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	fmt.Printf("Map %v has %d entries ✓\n", m, len(m))
}

func testGoroutines() {
	fmt.Print("Goroutines: ")
	done := make(chan bool)
	go func() {
		time.Sleep(10 * time.Millisecond)
		done <- true
	}()
	<-done
	fmt.Println("Concurrent execution successful ✓")
}

func testDeferredExecution() {
	fmt.Print("Deferred Execution: ")
	defer fmt.Println("Deferred function executed ✓")
}
