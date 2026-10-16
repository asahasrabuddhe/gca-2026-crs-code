# Building a Container Runtime from Scratch

Companion code for **Building a Container Runtime from Scratch**,
GopherCon Africa 2026.

The runtime is built one tag at a time, the same way as the 2025 version
of this talk. Each tag adds one piece and its tests:

```bash
git checkout 01-pid && make build
sudo ./mycontainer run ubuntu bash
```

This commit is the scaffold: the VM, the image puller, the pre-flight. The
runtime starts at `01-pid`.

```bash
make vm && make vm-shell      # a provisioned Ubuntu 24.04 VM; docs/setup.md for others
make rootfs                   # pulls ubuntu:26.04 into rootfs/ubuntu
./scripts/check.sh            # the pre-flight
```

## Licence

MIT. See [`LICENSE`](LICENSE).
