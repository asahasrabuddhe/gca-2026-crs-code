# Building a Container Runtime from Scratch

Companion code for **Building a Container Runtime from Scratch**,
GopherCon Africa 2026. A thirty-minute slot: twenty-five minutes of talk,
five for questions.

A container is a process, in an isolated environment, with a root
filesystem. This repository builds one in Go, standard library only, one
tag at a time, the same way as the 2025 version of the talk
([`asahasrabuddhe/gcr`](https://github.com/asahasrabuddhe/gcr)). Each tag
adds one thing and the tests that pin it:

| Tag | Adds | Runs as |
|---|---|---|
| `01-pid` | a PID namespace: the container's first process is PID 1 | sudo |
| `02-mnt` | a mount namespace and `pivot_root` into the image | sudo |
| `03-user` | a user namespace: root inside, you outside | you |
| `04-uts` | a UTS namespace: the container's ID as its hostname | you |
| `05-ipc` | an IPC namespace: the host's shared memory is gone | you |
| `06-cgroups` | cgroup v2 limits on memory, CPU and processes | sudo |
| `07-runtime` | capabilities, an init as PID 1, and a flag for everything | either |

The slides live in a sibling repository,
[`gca-2026-crs-slides`](https://github.com/asahasrabuddhe/gca-2026-crs-slides).

## Setup

You need a Linux box. [`docs/setup.md`](docs/setup.md) has one for
Windows, macOS and Linux, all from the same cloud-init file. On a Mac with
OrbStack:

```bash
make vm            # a provisioned Ubuntu 24.04 VM
make vm-shell      # a shell in this directory, inside it
```

Then, inside:

```bash
make rootfs           # pulls ubuntu:26.04 into rootfs/ubuntu, as you, not root
./scripts/check.sh    # the pre-flight: OK or FAIL, one line per requirement
```

`rootfs/<image>` is what `mycontainer run <image>` runs. The 2025 version
exported the image through a Docker daemon; `img2rootfs` pulls it from the
registry with the standard library, applies the layers and their
whiteouts, and checks every digest. Pull it before the talk, while there is
network.

## The walk

Every step is the same two commands, a checkout and a build:

```bash
git checkout 01-pid && make build
```

`rootfs/` and the binaries are not tracked, so they survive every
checkout. `git checkout main` at the end.

### 01-pid

```console
$ sudo ./mycontainer run ubuntu bash -c 'echo $$; ls -d /proc/[0-9]* | wc -l'
[run] as pid 13480
[child] as pid 1
1
21
```

The parent is an ordinary process on the host. The child it starts with
`CLONE_NEWPID` is PID 1, and it execs the command, so the command is PID 1
too. But `/proc` still lists the host's 21 processes: a PID namespace
changes the numbers, and `/proc` is a filesystem the host mounted.

### 02-mnt

```console
$ sudo ./mycontainer run ubuntu bash -c 'echo $$; ls -d /proc/[0-9]* | wc -l; ls /'
[run] as pid 13494
[child] as pid 1
1
2
bin   boot  dev  etc  home  lib  media  mnt  opt  proc  root  run  sbin  srv  sys  tmp  usr  var
```

`CLONE_NEWNS`, a fresh `/proc` mounted from inside the PID namespace, and
`pivot_root` into `rootfs/ubuntu`. `/proc` lists two processes, and `/`
is the image. The order of the mounts is the lesson, and
`internal/rootfs` pins it in a test.

### 03-user

```console
$ ./mycontainer run ubuntu bash -c 'id; cat /proc/self/uid_map'
[run] as pid 13507
[child] as pid 1
uid=0(root) gid=0(root) groups=0(root),65534(nogroup)
         0        501          1
```

No sudo. `CLONE_NEWUSER` with one line of map: container root is the
image's owner on the host. Run as yourself, that is you.

### 04-uts

```console
$ hostname
crs-dev
$ ./mycontainer run ubuntu hostname
[run] as pid 13520
[child] as pid 1
67557a8fc599
```

`CLONE_NEWUTS`, and the container's ID as its hostname.

### 05-ipc

Make a shared memory segment on the host, then look for it from inside,
first at `04-uts`:

```console
$ ipcmk -M 1M
Shared memory id: 4
$ ./mycontainer run ubuntu ipcs -m
------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
0xa82740fa 4          root       644        1048576    0
```

The host's segment, and its owner reads `root` through the user
namespace's map. Then at `05-ipc`, with `CLONE_NEWIPC`:

```console
$ ./mycontainer run ubuntu ipcs -m
------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status
```

`ipcrm -m 4` afterwards.

### 06-cgroups

```console
$ sudo ./mycontainer run -pids 20 ubuntu sh -c 'cat /sys/fs/cgroup/pids.max; for i in $(seq 40); do sleep 2 & done; wait'
20
/bin/sh: 0: Cannot fork
$ sudo ./mycontainer run -memory 64m ubuntu sh -c 'a=$(head -c 100000000 /dev/zero | tr "\0" x); echo survived'; echo $?
137
```

In 2025 this was a shell script against cgroup v1 paths. Now the runtime
writes `memory.max`, `cpu.max` and `pids.max` into
`/sys/fs/cgroup/mycontainer/<id>` on cgroup v2, starts the child inside it
with `CLONE_INTO_CGROUP`, and removes it afterwards. `CLONE_NEWCGROUP`
lets the container read its own limits. The fork bomb is refused at
twenty, and the memory hog is killed: 137 is 128 plus SIGKILL. Limits need
root, or a delegated cgroup.

### 07-runtime

The full runtime, where every piece is a flag, plus the two things a real
runtime does that the tags above do not:

- **Capabilities.** Root inside keeps the fourteen capabilities Docker
  leaves a container, so it cannot mount. `-caps default,+CAP_SYS_ADMIN`
  gives one back.
- **An init as PID 1.** From `01-pid` to `06-cgroups` the command is PID 1,
  so it collects zombies and ignores SIGTERM. Now the runtime's own init
  is PID 1: it reaps orphans, forwards signals, and reports the command's
  exit code. `-init=false` turns it off.

It is rootful by default, as Docker is, and rootless when run as an
ordinary user. `./mycontainer -h` lists the flags. `scripts/demo.sh 1` to
`7` walk through them, one beat per pillar:

| Beat | Shows | Doc |
|---|---|---|
| `1` | Namespaces, and the `/proc` gotcha | [`docs/01-namespaces.md`](docs/01-namespaces.md) |
| `2` | The rootfs, and a host file that is gone | [`docs/02-rootfs.md`](docs/02-rootfs.md) |
| `3` | cgroups: limits read back, a fork bomb, a memory hog | [`docs/03-cgroups.md`](docs/03-cgroups.md) |
| `4` | Capabilities: root cannot mount, until one bit | [`docs/04-capabilities.md`](docs/04-capabilities.md) |
| `5` | Init: a zombie and a dropped SIGTERM, then neither | [`docs/05-init.md`](docs/05-init.md) |
| `6` | Rootless: root inside, you outside | [`docs/01-namespaces.md`](docs/01-namespaces.md) |
| `7` | strace: every syscall above, counted | [`docs/environment.md`](docs/environment.md) |

## Tests

```bash
make check          # gofmt, go vet (both build tags), go test, shellcheck
make test-root      # inside the VM: the end-to-end suite, as root
```

Every tag carries its own tests, and every tag passes them. The pure
packages test on macOS: the mount sequence runs against a recorder, and
cgroup v2 is files, so a temp dir stands in for `/sys/fs/cgroup`. The
end-to-end suite runs real containers, as root and as the user who ran
sudo.

## Layout, at 07-runtime

```
cmd/mycontainer/       the binary: run (the parent) and child (PID 1 inside)
cmd/img2rootfs/        pull an image into rootfs/, standard library only
internal/container/    the parent and the child (Linux only)
internal/rootfs/       mounts and pivot_root, in the order that works
internal/cgroup/       a cgroup v2 leaf and its limit files
internal/namespace/    the seven namespaces, as clone flags
internal/caps/         capability names, the default fourteen, the drop
internal/initproc/     PID 1: reap orphans, forward signals, report the exit
internal/spec/         what the parent tells the child
internal/cli/          flags to spec
internal/image/        registry client and layer unpacking
scripts/               check.sh, demo.sh, vm.sh
provision/             one cloud-init file for every VM
docs/                  setup, environment, one explainer per pillar
```

## Things worth knowing before you extend it

- **Capabilities are per thread.** The child locks itself to one OS thread
  before dropping the bounding set, or Go may fork the command from a
  thread that still holds everything.
- **`pids.max` counts the init's own threads.** They also take PIDs 2 to 7
  in the container, which is why the command is PID 8 under the init. Keep
  `-pids` above about 16.
- **A SIGKILLed runtime leaks its cgroup.** The container dies with it,
  but nothing removes the directory. `check.sh` sweeps them.
- **Pull the image as yourself.** From `03-user`, container root maps to
  the image's owner, and an image owned by root cannot be used without
  sudo.
- **There is no network.** `07-runtime` gives the container a network
  namespace with a down loopback and nothing else. Wiring it up is a
  different talk.

## Licence

MIT. See [`LICENSE`](LICENSE).
