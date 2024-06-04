# Terrapin

Terrapin is a Go library and command-line tool for creating and verifying data attestations using SHA-256 hashes. It provides a robust method for ensuring data integrity by generating and verifying attestations.

## Features

- Create attestations for data files.
- Verify data files against provided attestations.
- Support for verifying entire files or specific byte ranges.
- Output data to standard output if verification succeeds.

## Installation

### Prerequisites

- Go 1.22 or higher

### Clone the Repository

```bash
git clone https://github.com/fkautz/terrapin-go.git
cd terrapin-go
```

### Build the Command-Line Tool

```bash
go build -o terrapin ./cmd/terrapin
```

## Usage

The `terrapin` command-line tool supports three subcommands: `attest`, `validate`, and `cat`.

### Attest

Create attestations for an input file.

```bash
./terrapin attest -input <input_file> -output <output_file>
```

- `-input`: Path to the input file (required).
- `-output`: Path to the output file for storing attestations (optional).

Example:

```bash
./terrapin attest -input example.txt -output example.attestations
```

### Validate

Verify an input file against provided attestations.

```bash
./terrapin validate -input <input_file> -attestations <attestations_file> [-start <start_byte>] [-end <end_byte>]
```

- `-input`: Path to the input file (required).
- `-attestations`: Path to the attestations file (required).
- `-start`: Start byte for range verification (optional).
- `-end`: End byte for range verification (optional).

Example:

```bash
./terrapin validate -input example.txt -attestations example.attestations
```

### Cat

Verify an input file and echo its content if verification succeeds.

```bash
./terrapin cat -input <input_file> -attestations <attestations_file> [-start <start_byte>] [-end <end_byte>]
```

- `-input`: Path to the input file (required).
- `-attestations`: Path to the attestations file (required).
- `-start`: Start byte for range verification (optional).
- `-end`: End byte for range verification (optional).

Example:

```bash
./terrapin cat -input example.txt -attestations example.attestations
```

## Library Usage

Terrapin can also be used as a Go library. Below is an example of how to use the `terrapin` package in your code.

### Example

```go
package main

import (
    "fmt"
    "io"
    "os"

    "github.com/fkautz/terrapin"
)

func main() {
    file, err := os.Open("example.txt")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
        os.Exit(1)
    }
    defer file.Close()

    terrapinInstance := terrapin.NewTerrapin()
    buffer := make([]byte, terrapin.BufferCapacity)

    for {
        n, err := file.Read(buffer)
        if err != nil && err != io.EOF {
            fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
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

    gid, attestations, err := terrapinInstance.Finalize()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to finalize terrapin: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("Gitoid URI:", gid)
    // Save the attestations or use them as needed
}
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any changes or enhancements.

## License

This project is licensed under the ApacheV2 License. See the [LICENSE](LICENSE) file for details.

## Acknowledgements

- [gitoid](https://github.com/edwarnicke/gitoid) library for generating Git-compatible object IDs.
