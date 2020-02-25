## 何为Docker?
* Docker 是一个开源工具,它可以将你的应用打包成一个标准格式的镜像(Image),并以容器的方式运行
* Docker 将程序运行的所需要的一切:`Code`,`ENV`,`System`等等包装在一个完整的文件系统中
* 所有容器会共享一个`Kernel`

```
|----------------|
|  CODE - Image  |
|----------------|
|  JRE - Image   |
|----------------|
|      KERNEL    |
|----------------|
```


## Linux Namespace 介绍

`Linux Namespace` 是`Kernel`的一个功能,它提供了内核级别隔离系统资源的方法,通过将系统的全局资源放到不同的`Namespace`来实现隔离资源的目的.目前用到`Namespace`有:

| 名称 | 宏定义 | 隔离内容 |
| ------ | ------ | ------ |
| IPC | CLONE_NEWIPC | 实现容器与宿主机、容器与容器之间的IPC隔离。IPC资源包括信号量、消息队列和共享内存 |
| Network | CLONE_NEWNET | 提供了关于网络资源的隔离，包括网络设备、IPv4和IPv6协议栈、IP路由表、防火墙,套接字等 |
| Mount |CLONE_NEWNS | 实现隔离文件系统挂载点,使容器内有独立的挂载文件系统 |
| PID | CLONE_NEWPID | 实现容器内有独立的进程树 (也就意味着每个容器都有自己的PID为1的进程) |
| User | CLONE_NEWUSER | 实现用户可以将不同的主机用户映射到容器,比如`user`用户映射到容器内的`root`用户上
| UTS | CLONE_NEWUTS | 实现容器可以拥有独立的主机名和域名，在网络上可以视为独立的节点 |
| Cgroup | CLONE_NEWCGROUP | 实现资源的限制(CPU,Memory等等)

`Namespace`的`API`主要用到下面3个系统调用:
* `CLONE` 创建新进程,并且系统调用参数会判断哪个类型的`Namespace`被创建(如上表格中的`CLONE_NEWUSER`),并且子进程也会包含到这些`Namespace`中
* `UNSHARE` 将进程移出某个`Namespace`
* `SETNS` 将进程切换/加入到某个`Namespace`中


-----------------------------------

### **Linux Namespace**

-----------------------------------

#### `UTS Namespace`
`UTS Namespace` 主要用来隔离`nodename`和`domainname`两个系统标识,每个`Namespace`允许有自己的`hostname`

下面使用将使用`Go`来做一个` UTS Namespace`的例子.


```Go
// UTS/clone.go

package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:syscall.CLONE_NEWUTS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run();err != nil{
		log.Fatal(err)
	}
}
```

解释下代码, `exec.Command("sh")`是来指定被`fork`出来的新进程内的初始命令,默认用`sh`来执行(当然可以换成`bash`或`zsh`之类的)

下面就是设置系统调用参数,使用`CLONE_NEWUTS`这个标识符来创建一个`UTS Namespace`

`syscall`库封装好对`clone()`函数的调用,这段代码被执行后就会进入到一个`sh`运行环境中


执行`go run clone.go`的命令后,在这个交互式环境里面,使用`pstree -pl`查看下系统中进程之间的关系,如下:

```Bash
init(1)───init(537)───init(538)───sh(539)───sh(540)───sh(545)───node(547)───node(597)──zsh(612)───go(2005)───clone(2098)───sh(2103)───pstree(2104)
```

然后输出一下当前的`PID`

```Bash
# echo $$
2103
```

验证父进程和子进程是否不在同一个`UTS Namespace`中

```Bash
# readlink /proc/2098/ns/uts
uts:[4026532185]

# readlink /proc/2103/ns/uts
uts:[4026532195]
```

可以看到它们确实不在同一个`UTS Namespace`中,由于我们对进程的`CLONE`进了一个新的`UTS Namespace`内,所以这个环境下修改`hostname`/` NIS domain name` 对宿主主机没有任何影响,下面进行下实验

```Bash
## 在clone出来的sh执行
# hostname -b air
# hostname
air
```

另外在宿主机另启动一个`shell`

```Bash
root@DESKTOP-UMENNVI:~# hostname
DESKTOP-UMENNVI
```

可以看到,外部的`hostname`并没有被内部的修改所影响,由此可以了解到`UTS Namespeace`的作用


---------------------------------

#### `IPC Namespace`
`IPC Namespace`用来隔离`System V IPC`和`POSIX message queues`.每一个`IPC Namespace`都有自己的`System V IPC`和`POSIX message queue`

创建`IPC Namespace`跟之前的方法类似,只需要把`CLONE_NEWUTS`替换成`CLONE_NEWIPC`

```Go
// IPC/clone.go
package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:syscall.CLONE_NEWIPC,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
```

##### 下面来演示以下隔离效果

1. 在宿主机上打开一个`shell`

```Bash
root@DESKTOP-UMENNVI:~# ipcs -q

--------- 消息队列 -----------
键        msqid      拥有者  权限     已用字节数 消息
```

2. 在宿主机创建一个`message queue`

```Bash
root@DESKTOP-UMENNVI:~# ipcmk -Q
消息队列 id：0

# 查看消息队列
root@DESKTOP-UMENNVI:~# ipcs -q

--------- 消息队列 -----------
键        msqid      拥有者  权限     已用字节数 消息      
0x9dce743a 0          root       644        0            0 
```
3. 这里我们已经在宿主机上创建好一个`queue`了,下面将使用另一个`shell`去运行`IPC/clone.go`新建`IPC Namespace`

```Bash
root@DESKTOP-UMENNVI:# go run clone.go 
# ipcs -q 

--------- 消息队列 -----------
键        msqid      拥有者  权限     已用字节数 消息
```

可以发现,在新创建的`Namespace`里,看不到宿主机上已经创建的`message queue`,说明`IPC Namespace`创建成功,已经被隔离了

---------------------------------

#### `PID Namespace`

`PID Namespace`是用来隔离进程ID(PID).同样在不同的`PID Namespace`里可以拥有不同的进程树.在`docker container`里面,使用`ps -ef`就会发现前台运行的进程`PID`为**1**,那是因为每个容器都创建了独自的`PID Namespace`

基于上面的基础,把`CLONE_NEWIPC`替换成`CLONE_NEWPID`

```Go
// PID/clone.go
package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:syscall.CLONE_NEWPID,
	}


	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run();err != nil{
		log.Fatal(err)
	}
}
```

我们需要分别在宿主机和`go run PID/clone.go`内运行`shell`

1. 首先运行`go run PID/clone.go`查看shell的PID

```Bash
root@DESKTOP-UMENNVI:# go run PID/clone.go 
# echo $$
1
```

2. 在宿主机上查看进程树,查看一下进程的真实`PID`

```Bash
init(1)─┬─init(11)─┬─at-spi-bus-laun(294)─┬─dbus-daemon(307)
        ├─init(537)───init(538)───sh(539)───sh(540)───sh(545)───node(547)─┬─node(597)─┬─node(722)─┬─{node}(723)
        │                                                                 │           │           ├─{node}(724)
        │                                                                 │           │           ├─{node}(725)
        │                                                                 │           │           ├─{node}(726)
        │                                                                 │           │           ├─{node}(727)
        │                                                                 │           │           └─{node}(728)
        │                                                                 │           ├─zsh(610)───bash(3571)───pstree(4090)
        │                                                                 │           ├─zsh(612)───bash(3601)───go(3965)─┬─clone(4074)─┬─sh(4079)
        │                                                                 │           │                                  │             ├─{clone}(4075)
        │                                                                 │           │                                  │             ├─{clone}(4076)
        │                                                                 │           │                                  │             ├─{clone}(4077)
        │                                                                 │           │                                  │             └─{clone}(4078)
        │                                                                 │           │                                  ├─{go}(3966)
```
可以看到 `go run PID/clone.go`的真实`PID`应该是`4074`,也就是说这个`4074`被映射到`PID Namespace`里后为`1`.
当然这里还不能用`ps`/`top`来查看,因为会依赖`/proc`文件系统,还需要挂载`/proc`文件系统后才行.


---------------------------------

#### `Mount Namespace`

`Mount Namespace` 用来隔离各个容器的挂载节点.
在不同的`Namespace`的容器中看到的文件系统层次是不一样的.
在`Mount Namespace`中调用`mount()`和`umount()`仅仅只会影响当前`Namespace`内的文件系统,对全局文件系统没有影响

`Mount Namespace`是`Unix & Linux`第一个实现的`Namespace`类型,所以它的系统调用参数是`NEWNS`(`New Namespace`的缩写)

基于上面的基础,添加`syscall.CLONE_NEWNS`

```Go
// MOUNT/clone.go
package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:syscall.CLONE_NEWNS | syscall.CLONE_NEWPID,
	}


	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run();err != nil{
		log.Fatal(err)
	}
}
```

1. 运行代码`go run MOUNT/clone.go` 后,查看`/proc`的文件内容
> proc 是一个(伪)文件系统,提供访问系统内核数据的操作提供接口

```Bash
root@DESKTOP-UMENNVI:# go run MOUNT/clone.go 

# ls /proc
1    307   4553  545  acpi       crypto       interrupts  kmsg         modules       self         tty
11   309   4651  547  buddyinfo  devices      iomem       kpagecgroup  mounts        softirqs     uptime
192  339   4656  597  bus        diskstats    ioports     kpagecount   mtrr          stat         version
248  3571  4657  610  cgroups    dma          irq         kpageflags   net           swaps        vmallocinfo
272  4079  537   679  cmdline    driver       kallsyms    loadavg      pagetypeinfo  sys          vmstat
273  420   538   68   config.gz  execdomains  kcore       locks        partitions    sysvipc      zoneinfo
294  4315  539   69   consoles   filesystems  keys        meminfo      sched_debug   thread-self
299  4374  540   722  cpuinfo    fs           key-users   misc         schedstat     timer_list
```

可以看到这里的`/proc`还是宿主机的,下面将`/proc` **mount**到新建的`Namespace`中来

```Bash
# mount -t proc proc /proc
# ls /proc
1          cmdline    diskstats    interrupts  keys         loadavg  mtrr          self      thread-self  vmstat
5          config.gz  dma          iomem       key-users    locks    net           softirqs  timer_list   zoneinfo
acpi       consoles   driver       ioports     kmsg         meminfo  pagetypeinfo  stat      tty
buddyinfo  cpuinfo    execdomains  irq         kpagecgroup  misc     partitions    swaps     uptime
bus        crypto     filesystems  kallsyms    kpagecount   modules  sched_debug   sys       version
cgroups    devices    fs           kcore       kpageflags   mounts   schedstat     sysvipc   vmallocinfo
```

可以看到少了很多文件,现在用`top`/`ps`来查看系统进程

```Bash
# ps -ef
UID        PID  PPID  C STIME TTY          TIME CMD
root         1     0  0 14:47 pts/5    00:00:00 sh
root         6     1  0 14:58 pts/5    00:00:00 ps -ef
```

在当前的`Namespace`中,`sh`进程的`PID`为`1`的进程,说明`Mount && PID Namespace`和宿主机是隔离的,`mount`操作没有影响到宿主机
`Docker Volume` 也是利用了这个特性(还有`USER Namespace`)


---------------------------------

#### `User Namespace`

`User Namespace` 主要用来隔离用户&用户组ID.
一个进程的`User ID`和`Group ID`在`User Namespace`内外可以是不同的,比较常见的是在宿主机上以一个非`root`用户创建一个`User Namespace`,然后在`User Namespace`里面却映射成`root`用户
从Linux Kernel 3.8开始,非`root`进程也可以创建`User Namespace`,并且此用户在创建的`Namespace`里面可以被映射成`root`并且拥有`root`权限


具体代码如下:

```Go
// USER/clone.go
package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run();err != nil{
		log.Fatal(err)
	}
}
```

1. 以`root`用户在宿主机上看一下当前的用户和用户组

```Bash
root@DESKTOP-UMENNVI:# id
uid=0(root) gid=0(root) 组=0(root)
```

可以看到我们是`root`用户,接下来运行程序

```Bash
root@DESKTOP-UMENNVI:# go run USER/clone.go 
$ id
uid=65534(nobody) gid=65534(nogroup) 组=65534(nogroup)
```

可以看到它们的`UID`是不同的说明`User Namespace`创建&&隔离完成了

---------------------------------

#### `Network Namespace`

`Network Namespace` 是用来隔离网络设备,IPV4/IPV6等网络栈的`Namespace`
`Network Namespace`可以让每个容器拥有独立的虚拟网络设备,并且容器内的应用可以绑定到容器内的端口,每个`Namespace`内的端口都不会互相冲突
在宿主机上搭建网桥后,就能很方便地实现容器之间的通信

在上面的基础上改成`syscall.CLONE_NEWNET`

```Go
package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	cmd := exec.Command("sh")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:syscall.CLONE_NEWNET,
	}


	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run();err != nil{
		log.Fatal(err)
	}
}
```

1. 首先在宿主机上查看一下自己的网络设备

```Bash
eth0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 172.25.50.16  netmask 255.255.240.0  broadcast 172.25.63.255
        inet6 fe80::215:5dff:fe50:5608  prefixlen 64  scopeid 0x20<link>
        ether 00:15:5d:50:56:08  txqueuelen 1000  (以太网)
        RX packets 766496  bytes 148694705 (148.6 MB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 688919  bytes 3273950212 (3.2 GB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

lo: flags=73<UP,LOOPBACK,RUNNING>  mtu 65536
        inet 127.0.0.1  netmask 255.0.0.0
        inet6 ::1  prefixlen 128  scopeid 0x10<host>
        loop  txqueuelen 1000  (本地环回)
        RX packets 2  bytes 100 (100.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 2  bytes 100 (100.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
```
可以宿主机上有`eth0`,`lo`等网络设备

2. 运行程序查看网络设备 `go run NET/clone.go`

```Bash
root@DESKTOP-UMENNVI:# go run NET/clone.go 
# ifconfig
# 
```

可以看到网络设备什么都没有,因为`Network Namespace`与宿主机之间的网络是处于隔离状态了


