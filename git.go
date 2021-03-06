package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/reconquest/hierr-go"
)

const (
	PushPrune   = true
	PushNoPrune = false
)

type git struct {
	path string
}

type ref struct {
	name string
	hash string
	stat os.FileInfo
}

type refs []ref

func (refs refs) Len() int {
	return len(refs)
}

func (refs refs) Swap(i, j int) {
	refs[i], refs[j] = refs[j], refs[i]
}

func (refs refs) Less(i, j int) bool {
	if refs[i].stat == nil {
		panic(
			fmt.Sprintf("ref %s stat is nil", refs[i].hash),
		)
	}

	if refs[j].stat == nil {
		panic(
			fmt.Sprintf("ref %s stat is nil", refs[j].hash),
		)
	}

	return refs[i].stat.ModTime().Unix() < refs[j].stat.ModTime().Unix()
}

func (repo *git) updateRef(refName string, pointer string) error {
	output, err := exec.Command(
		"git", "-C", repo.path, "update-ref", refName, pointer,
	).CombinedOutput()
	if err != nil {
		return hierr.Errorf(
			err,
			"error executing git update-ref\n%s", bytes.TrimSpace(output),
		)
	}

	return nil
}

func (repo *git) removeRef(refName string) error {
	output, err := exec.Command(
		"git", "-C", repo.path, "update-ref", "-d", refName,
	).CombinedOutput()
	if err != nil {
		return hierr.Errorf(
			err,
			"error executing git update-ref -d\n%s", bytes.TrimSpace(output),
		)
	}

	return nil
}

func (repo *git) writeObject(data []byte) (string, error) {
	cmd := exec.Command(
		"git", "-C", repo.path, "hash-object", "-w", "--stdin",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", hierr.Errorf(err, "can't get stdin for git hash-object")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", hierr.Errorf(err, "can't get stdout for git hash-object")
	}

	err = cmd.Start()
	if err != nil {
		return "", hierr.Errorf(
			err,
			"can't run git hash-object",
		)
	}

	_, err = stdin.Write(data)
	if err != nil {
		return "", hierr.Errorf(err, "can't write data to git hash-object")
	}

	err = stdin.Close()
	if err != nil {
		return "", hierr.Errorf(err, "can't close git hash-object stdin")
	}

	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", hierr.Errorf(
			err,
			"can't read git hash-object result",
		)
	}

	err = cmd.Wait()
	if err != nil {
		return "", hierr.Errorf(err, "can't wait for git hash-object")
	}

	return strings.TrimSpace(string(output)), nil
}

func (repo *git) listRefs(namespace string) (refs, error) {
	output, err := exec.Command(
		"git", "-C", repo.path, "show-ref",
	).CombinedOutput()
	if err != nil {
		return nil, hierr.Errorf(
			err,
			"error executing git show-ref\n%s", bytes.TrimSpace(output),
		)
	}

	refList := []ref{}
	scanner := bufio.NewScanner(bytes.NewBuffer(output))
	for scanner.Scan() {
		var hash, name string

		_, err := fmt.Sscanf(scanner.Text(), "%s %s", &hash, &name)
		if err != nil {
			return nil, hierr.Errorf(err, "can't read from git show-ref")
		}

		if !strings.HasPrefix(name, namespace) {
			continue
		}

		stat, err := os.Stat(filepath.Join(repo.path, ".git", name))
		if err != nil {
			return nil, hierr.Errorf(err, "can't stat() ref: %s", name)
		}

		refList = append(refList, ref{
			name: name,
			hash: hash,
			stat: stat,
		})
	}

	return refList, nil
}

func (repo *git) isGitRepo() bool {
	err := exec.Command(
		"git", "-C", repo.path, "rev-parse", "--git-dir",
	).Run()
	if err != nil {
		return false
	}

	return true
}

func (repo *git) clone(remote string) error {
	cmd := exec.Command(
		"git", "-C", repo.path, "clone", "--depth=1", "--bare", "-n", remote, repo.path,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return hierr.Errorf(
			err,
			"can't run git clone '%s' -> '%s'", remote, repo.path,
		)
	}

	return nil
}

func (repo *git) fetch(remote string, ref string) error {
	cmd := exec.Command(
		"git", "-C", repo.path, "fetch", remote, ref,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return hierr.Errorf(
			err,
			"can't run git fetch '%s' '%s'", remote, ref,
		)
	}

	return nil
}

func (repo *git) push(remote string, ref string, prune bool) error {
	command := []string{
		"git", "-C", repo.path, "push", remote, ref,
	}

	if prune {
		command = append(command, "--prune")
	}

	cmd := exec.Command(command[0], command[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return hierr.Errorf(
			err,
			"can't run git push '%s' '%s'", remote, ref,
		)
	}

	return nil
}

func (repo *git) catFile(hash string) ([]byte, error) {
	output, err := exec.Command(
		"git", "-C", repo.path, "cat-file", "-p", hash,
	).CombinedOutput()
	if err != nil {
		return nil, hierr.Errorf(
			err,
			"error executing git cat-file\n%s", bytes.TrimSpace(output),
		)
	}

	return output, nil
}
