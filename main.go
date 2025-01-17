package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cheggaaa/pb/v3"
	"github.com/pkg/errors"

	"github.com/incu6us/goimports-reviser/v2/pkg/module"
	"github.com/incu6us/goimports-reviser/v2/reviser"
)

const (
	fileDirArg             = "dir-path"
	projectNameArg         = "project-name"
	filePathArg            = "file-path"
	versionArg             = "version"
	removeUnusedImportsArg = "rm-unused"
	setAliasArg            = "set-alias"
	localPkgPrefixesArg    = "local"
	outputArg              = "output"
	formatArg              = "format"
	ignoreDirArg           = "ignore-dir"
)

// Project build specific vars
var (
	Tag       string
	Commit    string
	SourceURL string
	GoVersion string

	shouldShowVersion         *bool
	shouldRemoveUnusedImports *bool
	shouldSetAlias            *bool
	shouldFormat              *bool
)

var projectName, ignoreDir, filePath, dirPath, localPkgPrefixes, output string

var ignoreDirs []string

func init() {

	flag.StringVar(
		&ignoreDir,
		ignoreDirArg,
		"",
		"ignore dir path to fix imports",
	)

	flag.StringVar(
		&dirPath,
		fileDirArg,
		"",
		"dir path to fix imports",
	)

	flag.StringVar(
		&filePath,
		filePathArg,
		"",
		"File path to fix imports(ex.: ./reviser/reviser.go). Required parameter.",
	)

	flag.StringVar(
		&projectName,
		projectNameArg,
		"",
		"Your project name(ex.: github.com/incu6us/goimports-reviser). Optional parameter.",
	)

	flag.StringVar(
		&localPkgPrefixes,
		localPkgPrefixesArg,
		"",
		"Local package prefixes which will be placed after 3rd-party group(if defined). Values should be comma-separated. Optional parameters.",
	)

	flag.StringVar(
		&output,
		outputArg,
		"file",
		`Can be "file" or "stdout". Whether to write the formatted content back to the file or to stdout. Optional parameter.`,
	)

	shouldRemoveUnusedImports = flag.Bool(
		removeUnusedImportsArg,
		false,
		"Remove unused imports. Optional parameter.",
	)

	shouldSetAlias = flag.Bool(
		setAliasArg,
		false,
		"Set alias for versioned package names, like 'github.com/go-pg/pg/v9'. "+
			"In this case import will be set as 'pg \"github.com/go-pg/pg/v9\"'. Optional parameter.",
	)

	shouldFormat = flag.Bool(
		formatArg,
		false,
		"Option will perform additional formatting. Optional parameter.",
	)

	if Tag != "" {
		shouldShowVersion = flag.Bool(
			versionArg,
			false,
			"Show version.",
		)
	}
}

func printUsage() {
	if _, err := fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0]); err != nil {
		log.Fatalf("failed to print usage: %s", err)
	}

	flag.PrintDefaults()
}

func printVersion() {
	fmt.Printf(
		"version: %s\nbuild with: %s\ntag: %s\ncommit: %s\nsource: %s\n",
		strings.TrimPrefix(Tag, "v"),
		GoVersion,
		Tag,
		Commit,
		SourceURL,
	)
}

func main() {
	flag.Parse()
	ignoreDirs = strings.Split(ignoreDir, ",")
	fmt.Println("ignore dir: ", ignoreDirs)
	if shouldShowVersion != nil && *shouldShowVersion {
		printVersion()
		return
	}
	err := validateRequiredParam(filePath)
	errDir := validateDir(dirPath)
	if err != nil && errDir != nil {
		fmt.Printf("%s . and -dir-path %s\n\n", err, errDir)
		printUsage()
		os.Exit(1)
	}

	projectName, err := determineProjectName(projectName, filePath)
	if err != nil {
		fmt.Printf("%s\n\n", err)
		printUsage()
		os.Exit(1)
	}

	var options reviser.Options
	if shouldRemoveUnusedImports != nil && *shouldRemoveUnusedImports {
		options = append(options, reviser.OptionRemoveUnusedImports)
	}

	if shouldSetAlias != nil && *shouldSetAlias {
		options = append(options, reviser.OptionUseAliasForVersionSuffix)
	}

	if shouldFormat != nil && *shouldFormat {
		options = append(options, reviser.OptionFormat)
	}

	var count int64 = 0
	// create and start new bar
	bar := pb.StartNew(int(count))
	executor := func(filePath string) {
		if isIgnore(filePath) {
			return
		}

		if !isFormatFile(filePath) {
			return
		}

		formattedOutput, hasChange, err := reviser.Execute(projectName, filePath, localPkgPrefixes, options...)
		count++
		bar.SetCurrent(count - 2)
		bar.SetTotal(count)
		bar.Increment()
		if err != nil {
			log.Fatalf("%+v", errors.WithStack(err))
		}

		if output == "stdout" {
			fmt.Print(string(formattedOutput))
		} else if output == "file" {
			if !hasChange {
				return
			}
			fmt.Println(filePath)
			if err := ioutil.WriteFile(filePath, formattedOutput, 0644); err != nil {
				log.Fatalf("failed to write fixed result to file(%s): %+v", filePath, errors.WithStack(err))
			}
		} else {
			log.Fatalf(`invalid output "%s" specified`, output)
		}
	}

	switch {
	case dirPath == "./...":
		load("./", executor)
	case dirPath == "./":
		load(dirPath, executor)
	case dirPath != "":
		load(dirPath, executor)
	case dirPath == "":
		executor(filePath)
	}

	executor(filePath)
	// finish bar
	bar.Finish()
}

func isIgnore(path string) bool {
	for _, val := range ignoreDirs {
		if strings.Contains(path, val) {
			fmt.Println("ignore dir: ", path)
			return true
		}
	}
	return false
}

func validateDir(p string) error {
	if p == "./..." {
		return nil
	}

	s, err := os.Stat(p)
	if err != nil {
		return err
	}
	if !s.IsDir() {
		return errors.Errorf("dir error")
	}
	return nil
}

func errorCheck(err error) {
	if err != nil {
		panic(err)
	}
}

func pwd() string {
	path, err := os.Getwd()
	errorCheck(err)
	return path
}

func load(rootPath string, executor func(string)) {
	err := filepath.Walk(
		rootPath,
		func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			executor(path)
			return err
		},
	)
	errorCheck(err)
}

func isFormatFile(p string) bool {
	e := filepath.Ext(p)
	if e == ".go" {
		return true
	}
	return false
}

func determineProjectName(projectName, filePath string) (string, error) {
	if projectName == "" {
		projectRootPath, err := module.GoModRootPath(filePath)
		if err != nil {
			return "", err
		}

		moduleName, err := module.Name(projectRootPath)
		if err != nil {
			return "", err
		}

		return moduleName, nil
	}

	return projectName, nil
}

func validateRequiredParam(filePath string) error {
	if filePath == "" {
		return errors.Errorf("-%s should be set", filePathArg)
	}

	return nil
}
