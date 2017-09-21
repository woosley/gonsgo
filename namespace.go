package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
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

func namespace_init() {
	hostname := "container1"
	log.WithFields(log.Fields{"hostname": hostname}).Info(fmt.Sprintf("setup hostname"))
	if err := syscall.Sethostname([]byte("container1")); err != nil {
		log.WithFields(log.Fields{"error": err}).Error(err)
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// make mounted / private, see http://woosley.github.io/2017/08/18/mount-namespace-in-golang.html
	if err := syscall.Mount("", "/", "", uintptr(defaultMountFlags|syscall.MS_PRIVATE|syscall.MS_REC), ""); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Error makeing / private")
	}

	// privotroot, assuming you have a working rootfs, try rootfs.sh to create
	// one on Centos
	if err := privotRoot("/vagrant/abc"); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Error when privot root")
	}

	// mount proc
	log.Info("mounting proc")
	if err := syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), ""); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Error mounting proc")
	}
	wait_network()
	set_xeth1()
	container_command()
}

func privotRoot(newroot string) error {

	log.Info("start to pivotRoot")
	putold := filepath.Join(newroot, "/.pivot_root")
	if err := os.MkdirAll(putold, 0700); err != nil {
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

	log.WithFields(log.Fields{"command": shell}).Info("starting container command")
	// call exec, instead of cmd.Run, so current command is replaced by shell
	// in this way, the shell pid is 1
	cmd, _ := exec.LookPath(shell)
	if err := syscall.Exec(cmd, []string{}, os.Environ()); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error exec command")
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
	log.Info("creating veth pair")
	cmd := exec.Command("/sbin/ip", "link", "add", "xeth0", "type", "veth", "peer", "name", "xeth1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error creating veth")
	}
}
func setup_veth(pid int) {
	log.WithFields(log.Fields{"interface": "xeth1"}).Info("moving interface to process network namespace")
	cmd := exec.Command("/sbin/ip", "link", "set", "xeth1", "netns", fmt.Sprintf("%v", pid))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"error":     err,
			"interface": "xeth1",
		}).Error("error moving interface to namespace")
	}

	log.WithFields(log.Fields{"interface": "xeth0"}).Info("set up interface ip")
	cmd = exec.Command("/sbin/ifconfig", "xeth0", "192.168.8.2/24", "up")
	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"error":     err,
			"interface": "xeth0",
		}).Error("error setting up interface ip")
	}

}

//wait for network to startup
func wait_network() error {
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

func setup_uid_mapping(pid int) {
	str := []byte("1000 0 1")
	err := ioutil.WriteFile(fmt.Sprintf("/proc/%v/uid_map", pid), str, 0644)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error writing file")
	}
}

func set_xeth1() {
	log.WithFields(log.Fields{"interface": "xeth1"}).Info("set up interface ip")
	cmd := exec.Command("/sbin/ifconfig", "xeth1", "192.168.8.3/24", "up")
	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error settingup xeth3 ip")
	}
}

func main() {
	/*
		This command is run after script started up with proper namespace setup done
		Since it is the script itself, it will call a pre registered namespace_init after
		start up, sets up the necessary steps before the shell starts up
	*/

	cmd := setup_self_command(name)
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
				HostID:      0,
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      0,
				Size:        1,
			},
		},
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("error start command")
		os.Exit(1)
	}
	create_veth()
	setup_veth(cmd.Process.Pid)
	log.WithFields(log.Fields{"pid": cmd.Process.Pid}).Info("starting current process")

	if err := cmd.Wait(); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("starting current process")

	}
	log.Info("command ended")
}
