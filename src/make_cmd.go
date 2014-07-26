package src

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sourcegraph/makex"
	"github.com/sourcegraph/srclib/build"
	"github.com/sourcegraph/srclib/buildstore"
)

var testBuildDataDirName = buildstore.BuildDataDirName + "-exp"

func makefile(args []string) {
	make_(append(args, "-mf"))
}

func make_(args []string) {
	fs := flag.NewFlagSet("make", flag.ExitOnError)
	showOnly := fs.Bool("mf", false, "print generated makefile and exit")
	conf := &makex.Default
	makex.Flags(fs, conf, "")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `usage: `+Name+` make [options] [target...]

Generates and executes a Makefile that processes a repository, creating graph of
definitions, references, and dependencies in a repository's code at a specific
revision.

Run "`+Name+` makefile" to print the generated Makefile and exit.

This command uses makex to execute the Makefile, but the Makefile is also
compatible with GNU make. You can use the "`+Name+` makefile" command to
generate a Makefile to use with GNU make, if you'd like.

The options are:
	   `)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
	fs.Parse(args)
	goals := fs.Args()

	repoConf, err := OpenAndConfigureRepo(Dir)
	if err != nil {
		log.Fatal(err)
	}

	mk, mf, err := NewMaker(goals, repoConf, conf)
	if err != nil {
		log.Fatal(err)
	}

	if gopt.Verbose || *showOnly {
		data, err := makex.Marshal(mf)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(data))
		if *showOnly {
			return
		}
	}

	if conf.DryRun {
		err := mk.DryRun(os.Stdout)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	err = mk.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func NewMaker(goals []string, repoConf *ConfiguredRepo, conf *makex.Config) (*makex.Maker, *makex.Makefile, error) {
	if err := WriteRepositoryConfig(repoConf.RootDir, repoConf.CommitID, repoConf.Config, false); err != nil {
		return nil, nil, fmt.Errorf("unable to write repository config file due to error %s", err)
	}

	repoStore, err := buildstore.NewRepositoryStore(repoConf.RootDir)
	if err != nil {
		return nil, nil, err
	}
	buildDir, err := buildstore.BuildDir(repoStore, repoConf.CommitID)
	if err != nil {
		return nil, nil, err
	}
	// Use a relative base path for the Makefile so that we aren't tied to
	// absolute paths. This makes the Makefile more portable between hosts. (And
	// makex uses vfs, which restricts it to accessing only files under a
	// certain path.)
	buildDir, err = filepath.Rel(repoConf.RootDir, buildDir)
	if err != nil {
		return nil, nil, err
	}

	mf, err := build.CreateMakefile(buildDir, repoConf.Config)
	if err != nil {
		return nil, nil, err
	}

	if len(goals) == 0 {
		if defaultRule := mf.DefaultRule(); defaultRule != nil {
			goals = []string{defaultRule.Target()}
		}
	}

	// Change to the directory that make prereqs are relative to, so that makex
	// can properly compute the DAG.
	if err := os.Chdir(repoConf.RootDir); err != nil {
		return nil, nil, err
	}

	return conf.NewMaker(mf, goals...), mf, nil
}
