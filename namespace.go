package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

var registered = make(map[string]func())
var name = "namespace_init"
var self = "/proc/self/exe"
var shell string = "/bin/bash"

func init() {
	//register a function in memory
	fmt.Printf("register %s \n", name)
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
	fmt.Printf("setup hostname as container1\n")
	if err := syscall.Sethostname([]byte("container1")); err != nil {
		fmt.Println(err)
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	// make mounted / private, see http://woosley.github.io/2017/08/18/mount-namespace-in-golang.html
	if err := syscall.Mount("", "/", "", uintptr(defaultMountFlags|syscall.MS_PRIVATE|syscall.MS_REC), ""); err != nil {
		fmt.Println(err)
	}

	// privotroot, assuming you have a working rootfs, try rootfs.sh to create one
	err := privotRoot("/vagrant/abc")
	if err != nil {
		fmt.Println(err)
	}

	// mount proc
	fmt.Printf("mouting proc\n")
	if err := syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), ""); err != nil {
		fmt.Println(err)
	}

	container_command()
}

// /vagrant/abc
func privotRoot(newroot string) error {

	fmt.Printf("start to pivotRoot\n")
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

	fmt.Printf("starting container command %s\n", shell)
	// call exec, instead of cmd.Run, so current command is replaced by shell
	// in this way, the shell pid is 1
	cmd, _ := exec.LookPath(shell)
	err := syscall.Exec(cmd, []string{}, os.Environ())
	if err != nil {
		fmt.Println("error", err)
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
	fmt.Printf("creating veth pair\n")
	cmd := exec.Command("/sbin/ip", "link", "add", "xeth0", "type", "veth", "peer", "name", "xeth1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("error creating veth %s\n", err)
	}
}
func setup_veth(pid int) {
	fmt.Printf("moving xeth1 to process network namespace\n")
	cmd := exec.Command("/sbin/ip", "link", "set", "xeth1", "netns", fmt.Sprintf("%v", pid))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("error moving veth %s\n", err)
	}

	fmt.Printf("set up xeth0 ip\n")
	cmd = exec.Command("/sbin/ifconfig", "xeth0", "192.168.8.2/24", "up")
	if err := cmd.Run(); err != nil {
		fmt.Printf("error settingup xeth0 ip: %s\n", err)
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
			syscall.CLONE_NEWNET}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	create_veth()
	setup_veth(cmd.Process.Pid)
	fmt.Printf("starting current process %d\n", cmd.Process.Pid)

	if err := cmd.Wait(); err != nil {
		fmt.Printf("starting current process %s\n", err)
	}
	fmt.Printf("command ended\n")
}
