package git

import (
	"fmt"
	"os/exec"
)

var defaultCache = cache{}

type cache map[string]map[string]bool

func BranchExists(repo, branch string) (bool, error) {
	return defaultCache.BranchExists(repo, branch)
}

func (c cache) BranchExists(repo, branch string) (bool, error) {
	if c[repo] == nil {
		c[repo] = map[string]bool{}
	}
	exists, ok := c[repo][branch]
	if ok {
		return exists, nil
	}
	cmd := exec.Command("git", "ls-remote", "--heads", repo, "refs/heads/"+branch)
	out, err := cmd.Output()
	if err != nil {
		eerr, ok := err.(*exec.ExitError)
		if !ok {
			return true, err
		}
		return true, fmt.Errorf("%s: %s", err, eerr.Stderr)
	}
	exists = len(out) != 0 // Branch exists if ls-remote returns something
	c[repo][branch] = exists
	return exists, nil
}
