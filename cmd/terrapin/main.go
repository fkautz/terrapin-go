package main

import (
	"flag"
	"fmt"
	"github.com/fkautz/terrapin-go"
	"io"
	"os"
)

// blockSize is set to the buffer capacity defined in the terrapin package
const blockSize = terrapin.BufferCapacity

func main() {
	// Ensure there is at least one argument provided (the subcommand)
	if len(os.Args) < 2 {
		fmt.Println("Expected 'attest', 'validate', or 'cat' subcommands")
		os.Exit(1)
	}

	// Switch based on the first argument to determine which subcommand to execute
	switch os.Args[1] {
	case "attest":
		// Setup and parse flags for the "attest" subcommand
		attestCmd := flag.NewFlagSet("attest", flag.ExitOnError)
		inputFile := attestCmd.String("input", "", "Input file path")
		outputFile := attestCmd.String("output", "", "Output file path for terrapin attestations")
		attestCmd.Parse(os.Args[2:])

		// Ensure the input file path is provided
		if *inputFile == "" {
			fmt.Println("Input file path is required")
			attestCmd.Usage()
			os.Exit(1)
		}

		// Process the input file and generate attestations
		processInputFile(*inputFile, *outputFile)

	case "validate":
		// Setup and parse flags for the "validate" subcommand
		validateCmd := flag.NewFlagSet("validate", flag.ExitOnError)
		inputFile := validateCmd.String("input", "", "Input file path")
		attestationsFile := validateCmd.String("attestations", "", "Attestations file path for verification")
		start := validateCmd.Int64("start", 0, "Start byte for range")
		end := validateCmd.Int64("end", -1, "End byte for range")
		validateCmd.Parse(os.Args[2:])

		// Ensure both the input file path and attestations file path are provided
		if *inputFile == "" || *attestationsFile == "" {
			fmt.Println("Input file path and attestations file path are required")
			validateCmd.Usage()
			os.Exit(1)
		}

		// Validate the input file against the provided attestations
		validate(*inputFile, *attestationsFile, *start, *end)

	case "cat":
		// Setup and parse flags for the "cat" subcommand
		catCmd := flag.NewFlagSet("cat", flag.ExitOnError)
		inputFile := catCmd.String("input", "", "Input file path")
		attestationsFile := catCmd.String("attestations", "", "Attestations file path for verification")
		start := catCmd.Int64("start", 0, "Start byte for range")
		end := catCmd.Int64("end", -1, "End byte for range")
		catCmd.Parse(os.Args[2:])

		// Ensure both the input file path and attestations file path are provided
		if *inputFile == "" || *attestationsFile == "" {
			fmt.Println("Input file path and attestations file path are required")
			catCmd.Usage()
			os.Exit(1)
		}

		// Verify the input file and echo its content if verification succeeds
		cat(*inputFile, *attestationsFile, *start, *end)

	default:
		// Print an error message if the provided subcommand is not recognized
		fmt.Println("Expected 'attest', 'validate', or 'cat' subcommands")
		os.Exit(1)
	}
}

// processInputFile reads the input file, processes it with Terrapin, and writes the attestations
func processInputFile(inputFile, outputFile string) {
	// Open the input file
	file, err := os.Open(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open input file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Create a new Terrapin instance
	terrapinInstance := terrapin.NewTerrapin()
	buffer := make([]byte, blockSize)

	// Read the input file in chunks and add to the Terrapin instance
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "Failed to read input file: %v\n", err)
			os.Exit(1)
		}
		if n == 0 {
			break
		}

		err = terrapinInstance.Add(buffer[:n])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add data to terrapin: %v\n", err)
			os.Exit(1)
		}
	}

	// Finalize the Terrapin instance to generate the gitoid URI and attestations
	gid, attestations, err := terrapinInstance.Finalize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to finalize terrapin: %v\n", err)
		os.Exit(1)
	}

	// Write the attestations to the output file if specified
	if outputFile != "" {
		err = os.WriteFile(outputFile, attestations, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write attestations to output file: %v\n", err)
			os.Exit(1)
		}
	}

	// Print the gitoid URI
	fmt.Println("Gitoid URI:", gid)
}

// validate verifies the file against the provided attestations
func validate(filePath, attestationsPath string, start, end int64) {
	// Read the attestations file
	attestations, err := os.ReadFile(attestationsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read attestations file: %v\n", err)
		os.Exit(1)
	}

	// Open the input file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Create a new Terrapin instance with the provided attestations
	terrapinInstance, err := terrapin.NewTerrapinWithAttestations(attestations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create terrapin instance with attestations: %v\n", err)
		os.Exit(1)
	}

	// Verify a specific range if start and/or end is specified
	if start > 0 || end > 0 {
		if end == -1 {
			fi, err := file.Stat()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to stat file: %v\n", err)
				os.Exit(1)
			}
			end = fi.Size()
		}

		// Align the start and end offsets to buffer boundaries
		alignedStart := (start / blockSize) * blockSize
		_, err = file.Seek(alignedStart, io.SeekStart)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seek start position: %v\n", err)
			os.Exit(1)
		}

		// Verify the specified range
		valid, err := terrapinInstance.VerifyBufferRange(file, int(alignedStart), int(end))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to verify file: %v\n", err)
			os.Exit(1)
		}
		if !valid {
			fmt.Fprintf(os.Stderr, "File verification failed\n")
			os.Exit(1)
		}

		fmt.Println("File verification succeeded")
		return
	}

	// Verify the entire file
	valid, err := terrapinInstance.VerifyBuffer(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to verify file: %v\n", err)
		os.Exit(1)
	}
	if !valid {
		fmt.Fprintf(os.Stderr, "File verification failed\n")
		os.Exit(1)
	}

	fmt.Println("File verification succeeded")
}

// cat reads the file and attestations, verifies the file, and echoes it if validation succeeds
func cat(filePath, attestationsPath string, start, end int64) {
	// Read the attestations file
	attestations, err := os.ReadFile(attestationsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read attestations file: %v\n", err)
		os.Exit(1)
	}

	// Open the input file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Create a new Terrapin instance with the provided attestations
	terrapinInstance, err := terrapin.NewTerrapinWithAttestations(attestations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create terrapin instance with attestations: %v\n", err)
		os.Exit(1)
	}

	// Verify a specific range if start and/or end is specified
	if start > 0 || end > 0 {
		if end == -1 {
			fi, err := file.Stat()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to stat file: %v\n", err)
				os.Exit(1)
			}
			end = fi.Size()
		}

		// Align the start and end offsets to buffer boundaries
		alignedStart := (start / blockSize) * blockSize
		alignedEnd := ((end + blockSize - 1) / blockSize) * blockSize
		_, err = file.Seek(alignedStart, io.SeekStart)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seek start position: %v\n", err)
			os.Exit(1)
		}

		// Verify the specified range
		valid, err := terrapinInstance.VerifyBufferRange(file, int(alignedStart), int(alignedEnd))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to verify file: %v\n", err)
			os.Exit(1)
		}
		if !valid {
			fmt.Fprintf(os.Stderr, "File verification failed\n")
			os.Exit(1)
		}

		// Seek to the start position and echo the file content
		_, err = file.Seek(start, io.SeekStart)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to reset file reader: %v\n", err)
			os.Exit(1)
		}

		if _, err := io.CopyN(os.Stdout, file, end-start); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to echo file contents: %v\n", err)
			os.Exit(1)
		}

		return
	}

	// Verify the entire file
	valid, err := terrapinInstance.VerifyBuffer(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to verify file: %v\n", err)
		os.Exit(1)
	}
	if !valid {
		fmt.Fprintf(os.Stderr, "File verification failed\n")
		os.Exit(1)
	}

	// Reset file reader and echo the file content
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to reset file reader: %v\n", err)
		os.Exit(1)
	}

	if _, err := io.Copy(os.Stdout, file); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to echo file contents: %v\n", err)
		os.Exit(1)
	}
}
