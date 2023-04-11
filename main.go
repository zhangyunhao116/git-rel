package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/types"
	"golang.org/x/exp/slices"
)

var forcepush = flag.Bool("f", false, "force push release branch")

func main() {
	cbranch := branchCurrent()
	rbranch := releaseBranchName(cbranch)

	flag.Parse()
	cfg := NewConfig()
	defer cfg.Save()
	var icommit string // e.g. "f2fe3c80141d5febf72e1ca78e0a79dd9a10d233"
	if len(os.Args) < 2 || len(flag.Args()) < 1 {
		// Use saved icommit.
		v, ok := cfg.Get(cbranch)
		if !ok {
			panic("no icommit (the latest commit you don't want to merge)")
		}
		println("Use history icommit:" + v)
		icommit = v
	} else {
		// Use command line icommit.
		icommit = flag.Args()[0]
		cfg.Add(cbranch, icommit)
	}

	// Check arguments.
	if !slices.Contains(allCommits(cbranch), icommit) {
		panic("invalid commit: " + icommit)
	}

	// Delete the release branch if exists.
	if slices.Contains(branchList(), rbranch) {
		if _, err := git.Branch(func(g *types.Cmd) {
			g.AddOptions("-D")
			g.AddOptions(rbranch)
		}); err != nil {
			panic("maybe in the release branch\n" + err.Error())
		}
	}

	// Create and go to the release branch.
	// Defer go back to the current branch.
	if _, err := git.Checkout(func(g *types.Cmd) {
		g.AddOptions("-B")
		g.AddOptions(rbranch)
	}); err != nil {
		panic(err)
	}
	defer func() {
		if _, err := git.Checkout(func(g *types.Cmd) {
			g.AddOptions(cbranch)
		}); err != nil {
			panic(err)
		}
	}()

	// Save commit messsage for the first commit (the commmit after the icommit).
	var (
		firstcommit    string
		firstcommitmsg string
	)
	commits := allCommits(cbranch)
	if len(commits) <= 1 {
		panic("no enough commits")
	}
	for i, v := range commits {
		if v == icommit {
			firstcommit = commits[i-1]
		}
	}
	firstcommitmsg = commitMsg(firstcommit)

	// Release branch reset to the icommit.
	if _, err := git.Reset(func(g *types.Cmd) {
		g.AddOptions("--hard")
		g.AddOptions(icommit)
	}); err != nil {
		panic(err)
	}

	// Cherry pick (icommit,latestcommit] with changes.
	// defer cherry-pick --abort
	cherryPickAbort()
	cherryPickCommits(icommit, commits[0])
	defer cherryPickAbort()

	// Commit all changes.
	// git add .
	// git commit -m "{{commit msg}}"
	if _, err := git.Add(func(g *types.Cmd) {
		g.AddOptions(".")
	}); err != nil {
		panic(err)
	}
	git.Commit(func(g *types.Cmd) {
		g.AddOptions("-m")
		g.AddOptions(firstcommitmsg)
	})

	// git push -f --set-upstream origin {{rbranch}}
	if *forcepush {
		if output, err := git.Push(func(g *types.Cmd) {
			g.AddOptions("-f")
			g.AddOptions("--set-upstream")
			g.AddOptions("origin")
			g.AddOptions(rbranch)
		}); err != nil {
			panic(output + "\n" + err.Error())
		}
	}
}

func branchCurrent() string {
	branch, err := git.RevParse(func(g *types.Cmd) {
		g.AddOptions("--abbrev-ref")
		g.AddOptions("HEAD")
	})
	if err != nil {
		panic(branch + err.Error())
	}
	return strings.TrimSuffix(branch, "\n")
}

func branchList() []string {
	var branchlist []string
	branches, err := git.Branch(func(g *types.Cmd) {
		g.AddOptions("-l")
	})
	if err != nil {
		panic(branches + err.Error())
	}
	for _, v := range strings.Split(branches, "\n") {
		if v == "" {
			continue
		}
		branchname := strings.TrimPrefix(v, "* ")
		branchname = strings.TrimPrefix(branchname, "  ")
		branchlist = append(branchlist, branchname)
	}
	return branchlist
}

func releaseBranchName(b string) string {
	const devSuffix = "_dev"
	if !strings.HasSuffix(b, devSuffix) {
		panic("inlalid current branch:" + b)
	}
	return strings.TrimSuffix(b, devSuffix)
}

func commitMsg(commit string) string {
	// git show -s --format=%B {{commit}}
	g := types.NewCmd("show")
	g.ApplyOptions(
		[]types.Option{func(g *types.Cmd) {
			g.AddOptions("--format=%B")
			g.AddOptions("-s")
			g.AddOptions(commit)
		}}...,
	)
	msg, err := g.Exec(context.Background(), g.Base, g.Debug, g.Options...)
	if err != nil {
		panic(msg + err.Error())
	}
	return msg
}

func allCommits(b string) []string {
	// git log {{b}} --pretty=format:"%H"
	g := types.NewCmd("log")
	g.ApplyOptions(
		[]types.Option{func(g *types.Cmd) {
			g.AddOptions(b)
			g.AddOptions("--pretty=format:%H")
		}}...,
	)
	s, err := g.Exec(context.Background(), g.Base, g.Debug, g.Options...)
	if err != nil {
		panic(s + err.Error())
	}
	var commmits []string
	for _, v := range strings.Split(s, "\n") {
		if v != "" {
			v = strings.Trim(v, "\"")
			commmits = append(commmits, v)
		}
	}
	return commmits
}

// cherryPickCommits
func cherryPickCommits(c1, c2 string) {
	// git cherry-pick -n c1..c2
	g := types.NewCmd("cherry-pick")
	g.ApplyOptions(
		[]types.Option{func(g *types.Cmd) {
			g.AddOptions("-n")
			g.AddOptions(c1 + ".." + c2)
		}}...,
	)
	out, err := g.Exec(context.Background(), g.Base, g.Debug, g.Options...)
	if err != nil {
		panic(out + err.Error())
	}
}

func cherryPickAbort() {
	// git cherry-pick --abort
	g := types.NewCmd("cherry-pick")
	g.ApplyOptions(
		[]types.Option{func(g *types.Cmd) {
			g.AddOptions("--abort")
		}}...,
	)
	g.Exec(context.Background(), g.Base, g.Debug, g.Options...)
}
