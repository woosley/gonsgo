#!/usr/bin/env bash

ROOTFS_ROOT="/opt/rootfs"
ROOTFS_SIZE=1000000
IMAGE_NAME=os.image
MOUNT_POINT=/opt/rootfsmount/
RELEASE=http://centos.ustc.edu.cn/centos/6.9/os/x86_64/Packages/centos-release-6-9.el6.12.3.x86_64.rpm
YUM_OPTS="-y --installroot=$MOUNT_POINT"
create_rootfs () {
    mkdir -p $ROOTFS_ROOT
    dd if=/dev/zero of="$ROOTFS_ROOT/$IMAGE_NAME" bs=1k count=$ROOTFS_SIZE
}

mk_filesystem() {
    mkfs.ext4 -F -j $ROOTFS_ROOT/$IMAGE_NAME
}

mount_os() {
    mkdir -p $MOUNT_POINT
    mount -o loop $ROOTFS_ROOT/$IMAGE_NAME $MOUNT_POINT
}

rebuild_rpm() {
    mkdir -p $MOUNT_POINT/var/lib/rpm
    /bin/rpm --rebuilddb --root=$MOUNT_POINT
    /bin/rpm -ivh --root=$MOUNT_POINT --nodeps $RELEASE
    /usr/bin/yum $YUM_OPTS install yum findutils --nogpgcheck rpm
    cp /etc/resolv.conf $MOUNT_POINT/etc/
    chroot $MOUNT_POINT /bin/rpm --rebuilddb
    chroot $MOUNT_POINT /usr/bin/yum -y groupinstall core
    chroot $MOUNT_POINT /usr/bin/yum clean all
}

create_rootfs
mk_filesystem
mount_os
rebuild_rpm
