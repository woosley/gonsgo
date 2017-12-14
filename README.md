# GoNSGo
Sample code to play with Namespace in golang. Tested on Ubuntu 16.04.3 and Centos 7.2

# run

```
go get github.com/sirupsen/logrus
go run namespace.go
```

# notes
- on Ubuntu, you can run this program with non root account, who has sudo
  permission to run `ip` command. This demos how `user namespaces` works
- on Centos,  non root account will fail to run this program

# content

- namespace.go: start a shell in namespace
- rootfs.sh: create a CentOS rootfs 

# screenshot

![gif](https://raw.githubusercontent.com/woosley/GoNSGo/master/screenshot.gif)
