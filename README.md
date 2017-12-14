# GoNSGo
Sample code to play with Namespace in golang. Tested on Ubuntu 16.04.3 and Centos 7.2

# run

```
go get github.com/sirupsen/logrus
go run namespace.go
go run namespace.go command_to_be_run
```

# notes
- on Ubuntu, you can run this program with non root account, who has sudo
  permission to run `ip` command. This demos how `user namespaces` works
- on Centos,  non root account will fail to run this program

# content

- namespace.go: start a shell in namespace
- rootfs.sh: create a CentOS rootfs 

# sample

![gif](https://raw.githubusercontent.com/woosley/GoNSGo/master/screenshot.gif)

```
[root@localhost GoNSGo]# go run namespace.go /bin/ps -ef
INFO[0000] register name                                 name=namespace_init
INFO[0000] run myself ...
INFO[0000] creating veth pair for host
INFO[0000] register name                                 name=namespace_init
INFO[0000] setup hostname                                hostname=gonsgo
INFO[0000] mounting proc for /vagrant/abc
INFO[0000] start to pivotRoot
INFO[0000] wait network/interface setup to finish
INFO[0000] move xeth1 to process network namespace       interface=xeth1
INFO[0000] set up interface ip address in host           interface=xeth0 ip=192.168.8.2
INFO[0000] starting current process                      pid=4475
INFO[0001] set up interface ip in process namespace      interface=xeth1 ip=192.168.8.3
INFO[0001] starting container command                    command=/bin/ps
UID        PID  PPID  C STIME TTY          TIME CMD
root         1     0  0 05:01 ?        00:00:00 /bin/ps -ef
INFO[0001] command ended
```
