# env86

Toolchain for embeddable virtual machines based on [v86](https://github.com/copy/v86).

## Features

* Run, author, and debug web embeddable VMs via CLI tool
* Network VMs with full virtual network stack
* Prepare HTML assets for easy deploying to static hosts


## Getting Started

You can build env86 using Go, but the static assets it bundles are set up with Docker. With both installed,
and a cloned repo, you can run:

```sh
make all
```

This will use Docker to create some static assets like [v86](https://github.com/copy/v86),
then it will use Go to compile `env86` and write the executable to the current directory. Move this into your `PATH` or you can run `./env86` and get:

```
Usage:
env86 [command]

Available Commands:
boot             boot and run a VM
prepare          prepare a VM for publishing on the web
network          run virtual network and relay
serve            serve a VM and debug console over HTTP
create           create an image from directory or using Docker

Flags:
  -v    show version

Use "env86 [command] -help" for more information about a command.
```

### Creating Images

`env86` images are directories or tarballs with an `image.json` file and some v86 specific files
for the filesystem and initial state. With the `create` subcommand you can make an image from a Linux root directory
or using Docker, either an image to pull or a Dockerfile to build. Here's a Dockerfile to make an
Alpine Linux image:

```Dockerfile
FROM i386/alpine:3.18.6
RUN apk add openrc agetty
RUN sed -i 's/getty 38400 tty1/agetty --autologin root tty1 linux/' /etc/inittab
RUN echo 'ttyS0::once:/sbin/agetty --autologin root -s ttyS0 115200 vt100' >> /etc/inittab 
RUN echo "root:root" | chpasswd
``` 

The v86 emulator is 32-bit x86, so this is a 32-bit based Alpine. Any Docker commands run are run with
`--platform=linux/386` behind the scenes, so always keep this in mind when making images. We can build this
image and write it to `./alpine-vm`:

```sh
env86 create --from-docker=./path/to/Dockerfile ./alpine-vm
```

### Booting VMs

Once we have an env86 image, we can boot it. Booting has the most options:

```
Usage:
env86 boot <image>

Flags:
  -cdp
        use headless chrome
  -cold
        cold boot without initial state
  -console-url
        show the URL to the console
  -n
        enable networking (shorthand)
  -net
        enable networking
  -no-console
        disable console window
  -no-keyboard
        disable keyboard
  -no-mouse
        disable mouse
  -p string
        forward TCP port (ex: 8080:80)
  -save
        save initial state to image on exit
  -ttyS0
        open TTY over serial0
```

We can boot our Alpine VM with `--save` so we can skip cold booting in future boots:

```sh
env86 boot --save ./alpine-vm
```

This should pop open a window showing the screen console (though please submit an issue if this isn't
working on Windows or Linux). Once it gets to the shell, at the terminal we can hit `Ctrl+D` to send EOF,
which terminates the VM, but with `--save` it will first save the current state to the initial state of the image.

Now if we boot again, it should restore back to the prompt we ended it at. If we ever don't want this,
we can pass `--cold` to cold boot without restoring initial state.

We can also boot without the console window and just interact with the VM via ttyS0 in the terminal:

```sh
env86 boot --ttyS0 --no-console ./alpine-vm
```

### Publishing VMs

Once an image is in a state you want to share and you want to make it run on the web, you can use `prepare` to 
generate all the static files needed to serve this VM over HTTP:

```sh
env86 prepare ./alpine-vm www
```

This will make a `www` directory with an example `index.html` and all the files that need to be served over
HTTP to run this VM in the browser including the `v86.wasm` file. The image files are slightly different when prepared, splitting the initial state into 10MB parts for more efficiently loading over the web.

### Networking

If you boot with `--net` a virtual network stack and switch is created and wired up to the VM virtual NIC that will forward packets to your host computer network. The guest image will need to have network drivers and then be configured *after* booting to use the Internet. Here is a Dockerfile to make an env86 image that has a `./networking.sh` script to run after
booting to use the network provided by `--net`:

```Dockerfile
FROM i386/alpine:3.18.6

ENV KERNEL=lts
ENV HOSTNAME=localhost
ENV PASSWORD='root'

RUN apk add openrc \ 
            alpine-base \
            agetty \
            alpine-conf

# Install mkinitfs from edge (todo: remove this when 3.19+ has worked properly with 9pfs)
RUN apk add mkinitfs --no-cache --allow-untrusted --repository https://dl-cdn.alpinelinux.org/alpine/edge/main/ 

RUN if [ "$KERNEL" == "lts" ]; then \
    apk add linux-lts \
            linux-firmware-none \
            linux-firmware-sb16; \
else \
    apk add linux-$KERNEL; \
fi

# Adding networking.sh script (works only on lts kernel yet)
RUN if [ "$KERNEL" == "lts" ]; then \ 
    echo -e "echo '127.0.0.1 localhost' >> /etc/hosts && rmmod ne2k-pci && modprobe ne2k-pci\nhwclock -s\nsetup-interfaces -a -r" > /root/networking.sh && \ 
    chmod +x /root/networking.sh; \ 
fi

RUN sed -i 's/getty 38400 tty1/agetty --autologin root tty1 linux/' /etc/inittab
RUN echo 'ttyS0::once:/sbin/agetty --autologin root -s ttyS0 115200 vt100' >> /etc/inittab 
RUN echo "root:$PASSWORD" | chpasswd

# https://wiki.alpinelinux.org/wiki/Alpine_Linux_in_a_chroot#Preparing_init_services
RUN for i in devfs dmesg mdev hwdrivers; do rc-update add $i sysinit; done
RUN for i in hwclock modules sysctl hostname bootmisc; do rc-update add $i boot; done
RUN rc-update add killprocs shutdown

# Generate initramfs with 9p modules
RUN mkinitfs -F "ata base ide scsi virtio ext4 9p" $(cat /usr/share/kernel/$KERNEL/kernel.release)
```

Then we can run:

```sh
env86 create --from-docker=./path/to/Dockerfile ./alpine-net
env86 boot --net --save ./alpine-net
```

At the prompt we can run `./networking.sh` and it should get an IP and be able to connect to the Internet. 

### More Features

A few more features are tucked away or are in progress. The next major focus is on a standard guest service
for Linux VMs that will open up more functionality in `env86` like Docker-style `run` and `build` commands,
mounting local directories in the VM, and more.

### Using as a Library

The `env86` command line tool is a wrapper around a Go library you can use to work with and run VMs in regular
Go programs outside the browser. 


## Thanks

This project was made possible by [my sponsors](https://github.com/sponsors/progrium) but also the amazing work of the [v86](https://github.com/copy/v86) team. Also thanks to Joël and Adam for introducing me to v86.

## License

MIT