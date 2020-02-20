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


### UTS Namespace
`UTS Namespace` 主要用来隔离`nodename`和`domainname`两个系统标识,每个`Namespace`允许有自己的`hostname`

下面使用将使用`Go`来做一个` UTS Namespace`的例子.

```Go

```