package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var cmdPullRequest = &Command{
	Run:   pullRequest,
	Usage: "pull-request [-f] [TITLE|-i ISSUE] [-b BASE] [-h HEAD]",
	Short: "Open a pull request on GitHub",
	Long: `Opens a pull request on GitHub for the project that the "origin" remote
points to. The default head of the pull request is the current branch.
Both base and head of the pull request can be explicitly given in one of
the following formats: "branch", "owner:branch", "owner/repo:branch".
This command will abort operation if it detects that the current topic
branch has local commits that are not yet pushed to its upstream branch
on the remote. To skip this check, use -f.

If TITLE is omitted, a text editor will open in which title and body of
the pull request can be entered in the same manner as git commit message.

If instead of normal TITLE an issue number is given with -i, the pull
request will be attached to an existing GitHub issue. Alternatively, instead
of title you can paste a full URL to an issue on GitHub.
`,
}

var flagPullRequestBase, flagPullRequestHead string

func init() {
	// TODO: delay calculation of owner and current branch until being used
	cmdPullRequest.Flag.StringVar(&flagPullRequestBase, "b", git.Owner()+":master", "BASE")
	cmdPullRequest.Flag.StringVar(&flagPullRequestHead, "h", git.Owner()+":"+git.CurrentBranch(), "HEAD")
}

func pullRequest(cmd *Command, args []string) {
	messageFile := filepath.Join(git.Dir(), "PULLREQ_EDITMSG")

	err := writePullRequestChanges(messageFile, flagPullRequestBase, flagPullRequestHead)
	if err != nil {
		log.Fatal(err)
	}

	editCmd := buildEditCommand(messageFile)
	err = execCmd(editCmd)
	if err != nil {
		log.Fatal(err)
	}

	title, body, err := readTitleAndBodyFromFile(messageFile)
	if err != nil {
		log.Fatal(err)
	}
	if len(title) == 0 {
		log.Fatal("Aborting due to empty pull request title")
	}

	params := PullRequestParams{title, body, flagPullRequestBase, flagPullRequestHead}
	err = gh.CreatePullRequest(git.Owner(), git.Repo(), params)
	if err != nil {
		log.Fatal(err)
	}
}

func writePullRequestChanges(messageFile, base, head string) error {
	message := `
# Requesting a pull to %s from %s
#
# Write a message for this pull reuqest. The first block
# of the text is the title and the rest is description.
#
# Changes:
#
%s
`
	startRegexp := regexp.MustCompilePOSIX("^")
	endRegexp := regexp.MustCompilePOSIX(" +$")

	commitLogs := git.CommitLogs(getLocalBranch(base), getLocalBranch(head))
	commitLogs = strings.TrimSpace(commitLogs)
	commitLogs = startRegexp.ReplaceAllString(commitLogs, "# ")
	commitLogs = endRegexp.ReplaceAllString(commitLogs, "")

	message = fmt.Sprintf(message, base, head, commitLogs)

	return ioutil.WriteFile(messageFile, []byte(message), 0644)
}

func getLocalBranch(branchName string) string {
	result := strings.Split(branchName, ":")
	return result[len(result)-1]
}

func buildEditCommand(messageFile string) []string {
	editCmd := make([]string, 0)
	gitEditor := git.Editor()
	editCmd = append(editCmd, gitEditor)
	r := regexp.MustCompile("^[mg]?vim$")
	if r.MatchString(gitEditor) {
		editCmd = append(editCmd, "-c")
		editCmd = append(editCmd, "set ft=gitcommit")
	}
	editCmd = append(editCmd, messageFile)

	return editCmd
}

func execCmd(command []string) error {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	return err
}

func readTitleAndBodyFromFile(messageFile string) (title, body string, err error) {
	f, err := os.Open(messageFile)
	defer f.Close()
	if err != nil {
		return "", "", err
	}

	reader := bufio.NewReader(f)
	return readTitleAndBody(reader)
}

func readTitleAndBody(reader *bufio.Reader) (title, body string, err error) {
	r := regexp.MustCompile("\\S")
	var titleParts, bodyParts []string

	line, err := readln(reader)
	for err == nil {
		if strings.HasPrefix(line, "#") {
			break
		}
		if len(bodyParts) == 0 && r.MatchString(line) {
			titleParts = append(titleParts, line)
		} else {
			bodyParts = append(bodyParts, line)
		}
		line, err = readln(reader)
	}

	title = strings.Join(titleParts, " ")
	title = strings.TrimSpace(title)

	body = strings.Join(bodyParts, "\n")
	body = strings.TrimSpace(body)

	return title, body, nil
}

func readln(r *bufio.Reader) (string, error) {
	var (
		isPrefix bool  = true
		err      error = nil
		line, ln []byte
	)
	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}
	return string(ln), err
}
