package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/vmkteam/zenrpc/v2/parser"
)

const (
	openIssueURL = "https://github.com/vmkteam/zenrpc/issues/new"
	githubURL    = "https://github.com/vmkteam/zenrpc"
)

func main() {
	log.SetFlags(log.Lshortfile)

	start := time.Now()

	parser.GeneratorVersion = appVersion()
	fmt.Printf("Generator version: %s\n", parser.GeneratorVersion)

	var filename string
	if len(os.Args) > 1 {
		filename = os.Args[len(os.Args)-1]
	} else {
		filename = os.Getenv("GOFILE")
	}

	if filename == "" {
		exitOnErr(errors.New("no filename specified"))
	}

	fmt.Printf("Entrypoint: %s\n", filename)

	// create package info
	pi, err := parser.NewPackageInfo(filename)
	exitWithErr(err)

	// remove output file if it already exists
	outputFileName := pi.OutputFilename()
	if _, err = os.Stat(outputFileName); err == nil {
		err = os.Remove(outputFileName)
		exitWithErr(err)
	}

	// parse file
	err = pi.Parse(filename)
	exitWithErr(err)

	if len(pi.Services) == 0 {
		exitOnErr(fmt.Errorf("no services found in %s", filename))
	}

	// generate file
	err = generateFile(outputFileName, pi)
	exitWithErr(err)

	fmt.Printf("Generated: %s\n", outputFileName)
	fmt.Printf("Duration: %dms\n", time.Since(start).Milliseconds())
	fmt.Println()
	fmt.Print(pi)
	fmt.Println()
}

// exitWithErr logs the error and exits the program if error is not nil with additional information.
func exitWithErr(err error) {
	if err == nil {
		return
	}

	// print error to stderr
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)

	// print contact information to stdout
	fmt.Println("\nYou may help us and create issue:")
	fmt.Printf("\t%s\n", openIssueURL)
	fmt.Println("For more information, see:")
	fmt.Printf("\t%s\n\n", githubURL)

	os.Exit(1)
}

func generateFile(outputFileName string, pi *parser.PackageInfo) error {
	file, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		e := file.Close()
		exitOnErr(e)
	}(file)

	var output bytes.Buffer
	if err = serviceTemplate.Execute(&output, pi); err != nil {
		return err
	}

	source, err := format.Source(output.Bytes())
	if err != nil {
		return err
	}

	_, err = file.Write(source)
	return err
}

func appVersion() string {
	result := "devel"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return result
	}

	if info.Main.Version != "" {
		return info.Main.Version
	}

	for _, v := range info.Settings {
		if v.Key == "vcs.revision" {
			result = v.Value
		}
	}

	if len(result) > 8 {
		result = result[:8]
	}

	return result
}

// exitOnErr logs the error and exits the program if error is not nil.
func exitOnErr(err error) {
	if err != nil {
		log.Fatal("generation failed: ", err)
	}
}
