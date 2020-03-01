// Cgroup/limitMem/demo.go
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"syscall"
)

const cgroupMemoryHierarchyMount = "/sys/fs/cgroup/memory"
const limitMemory = "100M"

func main() {
	//-----------------------------------------------------
	// 5.运行 stress 进程测试内存占用
	if os.Args[0] == "/proc/self/exe" {
		//-----------------------------------------------------
		// 6. 挂载容器内的 /proc 的文件系统
		//Mount /proc to new root's  proc directory using MNT namespace
		if err := syscall.Mount("proc", "/proc", "proc", uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV), ""); err != nil {
			fmt.Println("Proc mount failed,Error : ", err)
		}

		// 7. 异步执行一个 sh 进程进入到容器内
		go func() {
			cmd := exec.Command("/bin/sh")

			cmd.SysProcAttr = &syscall.SysProcAttr{}

			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			os.Exit(1)
		}()

		// 8. 运行 stress 进程
		log.Printf("Current pid %d \n", syscall.SYS_GETPID)
		cmd := exec.Command("sh", "-c", `stress --vm-bytes 200m --vm-keep -m 1`)
		cmd.SysProcAttr = &syscall.SysProcAttr{}

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		log.Print("Close the program, press input `exit` \n")
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		} else {
			log.Printf("Stress process pid : %d \n", cmd.Process.Pid)
		}
		os.Exit(1)

	}

	//-----------------------------------------------------
	// 1. 先创建一个外部进程
	cmd := exec.Command("/proc/self/exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	//-----------------------------------------------------
	// 2. 在挂载了memory subsysyem 下创建限制内存的cgroup
	memory_limit_path := path.Join(cgroupMemoryHierarchyMount, "memorylimit")
	if f, err := os.Stat(memory_limit_path); err == nil {
		if !f.IsDir() {
			if err = os.Mkdir(memory_limit_path, 0755); err != nil {
				log.Fatal(err)
			} else {
				log.Printf("Mkdir memory cgroup %s \n", path.Join(cgroupMemoryHierarchyMount, "memorylimit"))
			}
		}
	} else {
		if err = os.Mkdir(memory_limit_path, 0755); err != nil {
			log.Fatal(err)
		} else {
			log.Printf("Mkdir memory cgroup %s \n", path.Join(cgroupMemoryHierarchyMount, "memorylimit"))
		}
	}

	//-----------------------------------------------------
	// 3. 限制 cgroup 内进程最大物理内存<limitMemory>
	if err := ioutil.WriteFile(path.Join(memory_limit_path, "memory.limit_in_bytes"), []byte(limitMemory), 0644); err != nil {
		log.Fatal("Litmit memory error,", err)
	} else {
		log.Printf("Litmit memory %v sucessed\n", limitMemory)
	}

	log.Printf("Self process pid : %d \n", cmd.Process.Pid)

	//-----------------------------------------------------
	// 4. 将进程加入到 cgroup 中
	if err := ioutil.WriteFile(path.Join(memory_limit_path, "tasks"), []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		log.Fatal("Move process to task error,", err)
	} else {
		log.Printf("Move process %d to task sucessed \n", cmd.Process.Pid)
	}

	cmd.Process.Wait()
}
