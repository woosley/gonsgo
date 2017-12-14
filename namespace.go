package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

var registered = make(map[string]func())
var name = "namespace_init"
var self = "/proc/self/exe"
var shell string = "/bin/bash"
var mountPoint = "/vagrant/abc"
var hostname = "gonsgo"

func init() {
	//register a function in memory
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.WithFields(log.Fields{"name": name}).Info("register name ")
	if _, exists := registered[name]; exists {
		panic(fmt.Sprintf("name already registered: %p", name))
	}
	registered[name] = namespace_init
	initializer, exists := registered[os.Args[0]]
	if exists {
		initializer()
		os.Exit(0)
	}
}

func exitWithError(err error, s string) {
	log.WithFields(log.Fields{"error": err}).Error(s)
	os.Exit(1)
}

func namespace_init() {
	log.WithFields(log.Fields{"hostname": hostname}).Info(fmt.Sprintf("setup hostname"))
	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		exitWithError(err, fmt.Sprintf("%s", err))
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

	// make mounted / private, see http://woosley.github.io/2017/08/18/mount-namespace-in-golang.html
	if err := syscall.Mount("", "/", "", uintptr(defaultMountFlags|syscall.MS_PRIVATE|syscall.MS_REC), ""); err != nil {
		exitWithError(err, "Error making / private")
	}

	// mount proc, this has to be done before pivot_root, or you will get permission denied with user namepsace anbled
	log.Info(fmt.Sprintf("mounting proc for %s", mountPoint))
	if err := syscall.Mount("proc", fmt.Sprintf("%s/proc", mountPoint), "proc", uintptr(defaultMountFlags), ""); err != nil {
		exitWithError(err, "Error mounting proc")
	}

	// pivotroot, assuming you have a working rootfs, try rootfs.sh to create one on Centos
	log.Info("start to pivotRoot")
	if err := pivotRoot(mountPoint); err != nil {
		exitWithError(err, fmt.Sprintf("Error when pivotRoot to %s", mountPoint))
	}

	wait_network()
	set_xeth1()
	container_command()
}

func pivotRoot(newroot string) error {

	putold := filepath.Join(newroot, "/.pivot_root")
	if err := os.MkdirAll(putold, 0700); err != nil {
		return err
	}

	// this line, is needed on Ubuntu, but not on Centos. I am very not sure why
	if err := syscall.Mount(newroot, newroot, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return err
	}

	if err := syscall.PivotRoot(newroot, putold); err != nil {
		return err
	}

	if err := os.Chdir("/"); err != nil {
		return err
	}

	if err := syscall.Unmount("/.pivot_root", syscall.MNT_DETACH); err != nil {
		return err
	}
	return nil
}

func container_command() {
	// call exec, instead of cmd.Run, so current command is replaced by shell
	// in this way, the shell pid is 1
	var err error
	var command string
	if len(os.Args) == 1 {
		log.Info(fmt.Sprintf("no command to be run passed, set command to %s ", shell))
		command, err = exec.LookPath(shell)
		os.Args = append(os.Args, command)
	} else {
		command, err = exec.LookPath(os.Args[1])
		if err != nil {
			exitWithError(err, fmt.Sprintf("can not find command %s", os.Args[1]))
		}
	}
	log.WithFields(log.Fields{"command": command}).Info("starting container command")
	if err := syscall.Exec(command, os.Args[1:], os.Environ()); err != nil {
		exitWithError(err, "error exec command")
	}
}

func setup_self_command(args ...string) *exec.Cmd {
	return &exec.Cmd{
		Path: self,
		Args: args,
		SysProcAttr: &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGTERM,
		},
	}
}

//create a veth pair
func create_veth() {
	cmd := exec.Command("sudo", "/sbin/ip", "link", "add", "xeth0", "type", "veth", "peer", "name", "xeth1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exitWithError(err, "error creating veth")
	}
}
func setup_veth(pid int) {
	log.WithFields(log.Fields{"interface": "xeth1"}).Info("move xeth1 to process network namespace")
	cmd := exec.Command("sudo", "/sbin/ip", "link", "set", "xeth1", "netns", fmt.Sprintf("%v", pid))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"error":     err,
			"interface": "xeth1",
		}).Error("error moving interface to namespace")
	}

	log.WithFields(log.Fields{"interface": "xeth0", "ip": "192.168.8.2"}).Info("set up interface ip address in host")
	cmd = exec.Command("sudo", "/sbin/ifconfig", "xeth0", "192.168.8.2/24", "up")
	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"error":     err,
			"interface": "xeth0",
		}).Error("error setting up interface ip")
	}
}

//wait for network to startup
func wait_network() error {
	log.Info("wait network/interface setup to finish")
	for i := 0; i < 10; i++ {
		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		if len(interfaces) > 1 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return nil
}

func set_xeth1() {
	log.WithFields(log.Fields{"interface": "xeth1", "ip": "192.168.8.3"}).Info("set up interface ip in process namespace")
	cmd := exec.Command("/sbin/ifconfig", "xeth1", "192.168.8.3/24", "up")
	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error settingup xeth1 ip")
	}
}

func main() {
	/*
		This command is run after script started up with proper namespace setup done
		Since it is the script itself, it will call a pre registered namespace_init after
		start up, sets up the necessary steps before the shell starts up
	*/

	os.Args[0] = name
	cmd := setup_self_command(os.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Info("run myself ...")
	if err := cmd.Start(); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error start command")
		os.Exit(1)
	}
	log.Info("creating veth pair for host")
	create_veth()
	setup_veth(cmd.Process.Pid)
	log.WithFields(log.Fields{"pid": cmd.Process.Pid}).Info("starting current process")

	if err := cmd.Wait(); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("starting current process")
	}

	log.Info("command ended")
}
