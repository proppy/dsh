package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dotcloud/docker/pkg/units"
	"github.com/samalba/dockerclient"
)

var (
	builtins = map[string]func([]string) error{
		"exit":   exit,
		"ps":     ps,
		"kill":   kill,
		"ls":     ls,
		"run":    run,
		"commit": commit,
	}

	docker          *dockerclient.DockerClient
	lastContainerID string
	lastImageID     string
)

func init() {
	var err error

	if docker, err = dockerclient.NewDockerClient(os.Getenv("DOCKER_HOST"), nil); err != nil {
		log.Fatal(err)
	}
}

func exit(args []string) error {
	code := 0

	if len(args) == 1 {
		c, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		code = c
	}

	os.Exit(code)

	return nil
}

func ps(args []string) error {
	containers, err := docker.ListContainers(false)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tIMAGE\tCMD\n")

	for _, c := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\n", c.Id[0:5], c.Image, c.Command)
	}

	w.Flush()

	return nil
}

func kill(args []string) error {
	return docker.KillContainer(args[0])
}

func ls(args []string) error {
	images, err := docker.ListImages()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tSIZE\tDATE\tNAME\n")

	for _, i := range images {
		if strings.Contains(i.RepoTags[0], "<none>") {
			continue
		}

		name := strings.Split(i.RepoTags[0], ":")[0]

		t := time.Unix(i.Created, 0).Format("Jan 02")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Id[:5], units.HumanSize(i.VirtualSize), t, name)
	}

	w.Flush()

	return nil
}

func run(args []string) error {
	d := args[len(args)-1] == "&"

	if d {
		args = args[:len(args)-1]
	}

	img := args[0][2:]
	if img == "_" {
		img = lastImageID
	}

	cmd := exec.Command("docker", append([]string{"run", "-it", fmt.Sprintf("-d=%t", d), img}, args[1:]...)...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	out, err := exec.Command("docker", "ps", "-l", "-q").Output()
	if err != nil {
		return err
	}

	lastContainerID = strings.TrimSpace(string(out))
	fmt.Println(lastContainerID)

	return nil
}

func commit(arg []string) error {
	repo, tag := "", ""
	if len(arg) > 0 && len(arg[0]) > 0 {
		parts := strings.Split(arg[0], ":")
		repo = parts[0]
		if len(parts) > 1 {
			tag = parts[1]
		}
	}
	img, err := docker.Commit(lastContainerID, repo, tag)
	if err != nil {
		return fmt.Errorf("failed to commit: %v", err)
	}
	lastImageID = img
	fmt.Println(lastImageID)
	return nil
}

func prompt() {
	fmt.Fprintf(os.Stdout, "> ")
}

func main() {
	s := bufio.NewScanner(os.Stdin)

	fmt.Fprintln(os.Stdout, "the shell for the 2000nds")

	for {
		prompt()
		if !s.Scan() {
			break
		}

		tokens := strings.Split(s.Text(), " ")

		if len(tokens[0]) > 2 && tokens[0][:2] == "./" {
			if err := run(tokens); err != nil {
				log.Fatal(err)
			}
			continue
		}

		if b, exists := builtins[tokens[0]]; exists {
			if err := b(tokens[1:]); err != nil {
				log.Fatal(err)
			}
			continue
		}

		cmd := exec.Command(tokens[0], tokens[1:]...)

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
}
