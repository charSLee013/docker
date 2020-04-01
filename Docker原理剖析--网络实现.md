## `Docker` 网络模式

#### `Bridge`
桥接模式，`docker run` 默认模式，此模式会为容器分配`network namespace`、设置IP等,并将容器网络桥接到一个虚拟网桥`docker0`上，可以和同一宿主机上桥接模式的其他容器进行通信

#### `Host`
主机模式,容器与宿主机共用一个`network namespace`，也是说跟宿主机共用网卡,需要注意容器中服务的端口号不能与`host`上已经使用的端口号冲突

#### `Container`
该网络模式是`Docker`中一种较为特别的网络的模式。处于这个模式下的`Docker`容器会共享其他容器的`network namespace`，因此，在该`network namespace`下的容器不存网络隔离

#### `None`
该模式下不为容器创造任何的网络环境,容器内部就只能使用`loopback`网络设备，不会再有其他的网络资源.

---

接下来我们将一步步动手实现`Docker``Bridge`模式下的原理

---

## `Linux Veth` 和 `Bridge`

### 前言
`Docker`的网络以及`Flannel`的网络实现都涉及到`Veth`和`Bridge`使用.
在宿主机上创建一个`Bridge`，每到一个容器创建，就会创建一对互通`Veth` (Bridge-veth <--> Container-veth)
一端连接到主机的`Bridge`(docker0),另一端连接到容器的`Network namespace`
可以通过`sudo brctl show`查看`Bridge`连接的`veth`

### 说明

##### `VETH` (virtual Ethernet)
`Linux Kernel`支持的一种虚拟网络设备，表示一对虚拟的网络接口
`Veth`的两端可以处于不同的`Network namespace`，可以作为主机和容器之间的网络通信
发送到`Veth`一端的请求会从另一端的`Veth`发出

##### `Bridge`
`Bridge` 是 `Linux` 上用来做 `TCP/IP` 二层协议交换的设备，相当于交换机
可以将其他网络设备挂在 `Bridge` 上面
当有数据到达时，`Bridge`会根据报文中的MAC信息进行广播，转发，丢弃.


### 网络拓扑图
```bash
                           +------------------------+
                           |                        | iptables +----------+
                           |  br01 192.168.88.1/24  |          |          |
                +----------+                        <--------->+ eth0   |
                |          +------------------+-----+          |          |
                |                             |                +----------+
           +----+---------+       +-----------+-----+
           |              |       |                 |
           | br-veth01    |       |   br-veth02     |
           +--------------+       +-----------+-----+
                |                             |
+--------+------+-----------+     +-------+---+-------------+
|        |                  |     |       |                 |
|  ns01  |   veth01         |     |  ns02 |  veth01         |
|        |                  |     |       |                 |
|        |   192.168.88.11  |     |       |  192.168.88.12  |
|        |                  |     |       |                 |
|        +------------------+     |       +-----------------+
|                           |     |                         |
|                           |     |                         |
+---------------------------+     +-------------------------+

```
`br01`是创建的`Bridge`，链接着两个`Veth`，两个`Veth`的另一端分别在另外两个`namespace`里
`eth0`是宿主机对外的网卡，`namespace`对外的数据包会通过`SNAT`/`MASQUERADE`出去 


#### 部署`Bridge`和`Veth`

##### 设置`Bridge`

创建`Bridge`

```bash
sudo brctl addbr br01
```

启动`Bridge`

```bash
sudo ip link set dev br01 up
# 也可以用下面这种方式启动
sudo ifconfig br01 up 
```

给`Bridge`分配IP地址

```bash
sudo ifconfig br01 192.168.88.1/24 up
```

##### 创建`Network namespace`

创建两个`namespace`: `ns01` `ns02`

```bash
sudo ip netns add ns01
sudo ip netns add ns02

## 查看创建的ns
sudo ip netns list
ns02
ns01
```

##### 设置`Veth pair`

创建两对`veth`



```bash
# 创建 `VETH` 设备：`ip link add link [DEVICE NAME] type veth`
sudo ip link add veth01 type veth peer name br-veth01
sudo ip link add veth02 type veth peer name br-veth02
```

将其中一端的`Veth`(br-veth$)挂载到`br01`下面

```bash
# attach 设备到 Bridge：brctl addif [BRIDGE NAME] [DEVICE NAME]
sudo brctl addif br01 br-veth01
sudo brctl addif br01 br-veth02

# 查看挂载详情
sudo brctl show br01
bridge name     bridge id               STP enabled     interfaces
br01            8000.321bc3fd56fd       no              br-veth01
                                                        br-veth02
```

启动这两对`Veth`

```bash
sudo ip link set dev br-veth01 up
sudo ip link set dev br-veth02 up
```

将另一端的`veth`分配给创建好的`ns`

```bash
sudo ip link set veth01 netns ns01
sudo ip link set veth02 netns ns02
```

##### 部署`Veth`在`ns`的网络

通过`sudo ip netns [NS] [COMMAND]`命令可以在特定的网络命名空间执行命令

查看`network namespace`里的网络设备:

```bash
sudo ip netns exec ns01 ip addr
1: lo: <LOOPBACK> mtu 65536 qdisc noop state DOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: sit0@NONE: <NOARP> mtu 1480 qdisc noop state DOWN group default qlen 1000
    link/sit 0.0.0.0 brd 0.0.0.0
8: veth01@if7: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN group default qlen 1000
    link/ether d2:88:ec:62:cd:0a brd ff:ff:ff:ff:ff:ff link-netnsid 0
```

可以看到刚刚被加进来的`veth01`还没有IP地址
给两个`network namespace`的`veth`设置IP地址和默认路由
默认网关设置为`Bridge`的`IP`

```bash
sudo ip netns exec ns01 ip link set dev veth01 up
sudo ip netns exec ns01 ifconfig veth01 192.168.88.11/24 up
sudo ip netns exec ns01 ip route add default via 192.168.88.1

sudo ip netns exec ns02 ip link set dev veth02 up
sudo ip netns exec ns02 ifconfig veth02 192.168.88.12/24 up
sudo ip netns exec ns02 ip route add default via 192.168.88.1
```

查看 `ns`的`veth`是否分配了IP

```bash
sudo ip netns exec ns01 ifconfig veth01
sudo ip netns exec ns02 ifconfig veth02

veth02: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 192.168.88.12  netmask 255.255.255.0  broadcast 192.168.88.255
        inet6 fe80::fca2:57ff:fe1c:67df  prefixlen 64  scopeid 0x20<link>
        ether fe:a2:57:1c:67:df  txqueuelen 1000  (以太网)
        RX packets 15  bytes 1146 (1.1 KB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 11  bytes 866 (866.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
```


#### 验证`ns`内网络情况

从 `ns01`里`ping ns02`,同时在默认用`tcpdump`在`br01 bridge`上抓包

```bash
# 首先启动抓包
sudo tcpdump -i br01 -nn

tcpdump: verbose output suppressed, use -v or -vv for full protocol decode
listening on br01, link-type EN10MB (Ethernet), capture size 262144 bytes

# 然后从 ns01 ping ns02
sudo ip netns exec ns01 ping 192.168.88.12 -c 1

PING 192.168.88.12 (192.168.88.12) 56(84) bytes of data.
64 bytes from 192.168.88.12: icmp_seq=1 ttl=64 time=0.086 ms

--- 192.168.88.12 ping statistics ---
1 packets transmitted, 1 received, 0% packet loss, time 0ms
rtt min/avg/max/mdev = 0.086/0.086/0.086/0.000 ms

# 查看抓包信息
16:19:42.739429 ARP, Request who-has 192.168.88.12 tell 192.168.88.11, length 28
16:19:42.739471 ARP, Reply 192.168.88.12 is-at fe:a2:57:1c:67:df, length 28
16:19:42.739476 IP 192.168.88.11 > 192.168.88.12: ICMP echo request, id 984, seq 1, length 64
16:19:42.739489 IP 192.168.88.12 > 192.168.88.11: ICMP echo reply, id 984, seq 1, length 64
16:19:47.794415 ARP, Request who-has 192.168.88.11 tell 192.168.88.12, length 28
16:19:47.794451 ARP, Reply 192.168.88.11 is-at d2:88:ec:62:cd:0a, length 28
```

可以看到`ARP`能正确定位到`MAC`地址,并且`reply`包能正确返回到`ns01`中,反之在`ns02`中`ping ns01`也是通的


在`ns01`内执行`arp`

```bash
sudo ip netns exec ns01 arp

地址                     类型    硬件地址            标志  Mask            接口
192.168.88.12            ether   fe:a2:57:1c:67:df   C                     veth01
192.168.88.1             ether   32:1b:c3:fd:56:fd   C                     veth01
```

可以看到`192.168.88.1`的`MAC`地址是正确的,跟`ip link`打印出来的是一致

```bash
ip link

6: br01: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default qlen 1000
    link/ether 32:1b:c3:fd:56:fd brd ff:ff:ff:ff:ff:ff
```

#### `ns`与外网互通

从`ns02 ping`  外网地址(如下以`114.114.114.114`为例子)

```bash
sudo ip netns exec ns02 ping 114.114.114.114 -c 1

PING 114.114.114.114 (114.114.114.114) 56(84) bytes of data.

--- 114.114.114.114 ping statistics ---
1 packets transmitted, 0 received, 100% packet loss, time 0ms
```

发现是`ping`不通的,抓包查看详情

```bash
# 抓Bridge设备
tcpdump -i br01 -nn -vv host 114.114.114.114

tcpdump: listening on br01, link-type EN10MB (Ethernet), capture size 262144 bytes
17:02:59.027478 IP (tos 0x0, ttl 64, id 51092, offset 0, flags [DF], proto ICMP (1), length 84)
    192.168.88.12 > 114.114.114.114: ICMP echo request, id 1045, seq 1, length 64


# 抓出口设备
tcpdump -i eth0 -nn -vv host 114.114.114.114
```

发现只有`br01`有出口流量,而出口网卡`eth0`没有任何反应,说明没有开启`ip_forward`

```bash
# 开启 ip_forward
sudo sysctl -w net.ipv4.conf.all.forwarding=1
```

再次尝试抓包`eth0`设备

```bash
sudo tcpdump -i eth0 -nn -vv host 114.114.114.114

tcpdump: listening on eth0, link-type EN10MB (Ethernet), capture size 262144 bytes
17:11:26.517292 IP (tos 0x0, ttl 63, id 15277, offset 0, flags [DF], proto ICMP (1), length 84)
    192.168.88.12 > 114.114.114.114: ICMP echo request, id 1059, seq 1, length 64
```

发现只有发出去的包`request`没有回来`replay`的包,原因是因为源地址是私有地址,如果发回来的包是私有地址会被丢弃
解决方法是将发出去的包`sourceIP`改成`gatewayIP`,可以用`SNAT`或者`MAQUERADE`

`SNAT`: 需要搭配静态IP
`MAQUERADE`: 可以用于动态分配的IP,但每次数据包被匹配中时,都会检查使用的IP地址

```bash
sudo iptables -t nat -A POSTROUTING -s 192.168.88.0/24 -j MASQUERADE

# 查看防火墙规则
sudo iptables -t nat -nL --line-number

Chain PREROUTING (policy ACCEPT)
num  target     prot opt source               destination         

Chain INPUT (policy ACCEPT)
num  target     prot opt source               destination         

Chain OUTPUT (policy ACCEPT)
num  target     prot opt source               destination         

Chain POSTROUTING (policy ACCEPT)
num  target     prot opt source               destination         
1    MASQUERADE  all  --  192.168.88.0/24      0.0.0.0/0
```

再次尝试`ping 114.114.114.114`

```bash
sudo ip netns exec ns02 ping 114.114.114.114 -c 1
```
抓包查看

```bash
sudo tcpdump -i eth0 -nn -vv host 114.114.114.114

tcpdump: listening on eth0, link-type EN10MB (Ethernet), capture size 262144 bytes
17:43:54.744599 IP (tos 0x0, ttl 63, id 46107, offset 0, flags [DF], proto ICMP (1), length 84)
    172.22.36.202 > 114.114.114.114: ICMP echo request, id 1068, seq 1, length 64
17:43:54.783749 IP (tos 0x4, ttl 71, id 62825, offset 0, flags [none], proto ICMP (1), length 84)
    114.114.114.114 > 172.22.36.202: ICMP echo reply, id 1068, seq 1, length 64

---

sudo tcpdump -i br01 -nn -vv
tcpdump: listening on br01, link-type EN10MB (Ethernet), capture size 262144 bytes17:43:54.744560 IP (tos 0x0, ttl 64, id 46107, offset 0, flags [DF], proto ICMP (1), length 84)
    192.168.88.12 > 114.114.114.114: ICMP echo request, id 1068, seq 1, length 64
17:43:54.783805 IP (tos 0x4, ttl 70, id 62825, offset 0, flags [none], proto ICMP (1), length 84)
    114.114.114.114 > 192.168.88.12: ICMP echo reply, id 1068, seq 1, length 64
```

可以看到从`eth0`出去的数据包的`sourceIP`已经变成网卡IP了
而`br01`收到的包的`sourceIP`还是`ns02` 的 `192.168.88.12`

---

## 清理

```bash
sudo ip netns del ns01
sudo ip netns del ns02
sudo ifconfig br01 down
sudo brctl delbr br01
sudo iptables -t nat -D POSTROUTING -s 192.168.88.0/24 -j MASQUERADE
```
