# npiperelay

npiperelay is a tool that allows you to access a Windows named pipe in a way
that is more compatible with a variety of command-line tools. With it, you can
use Windows named pipes from the Windows Subsystem for Linux (WSL).

For example, you can:

* Connect to Docker for Windows from the Linux Docker client in WSL
* Connect to MySQL Server running as a Windows service
* Connect interactively to a Hyper-V Linux VM's serial console
* Use gdb to connect to debug the kernel of a Hyper-V Linux VM
* Connect to Windows SSH agent via named pipe

Let me know on Twitter ([@gigastarks](https://twitter.com/gigastarks)) if you come up with more interesting uses.

# Installation

npiperelay is a Go program and comes as a single executable file; `npiperelay.exe`.
The [releases](https://github.com/jstarks/npiperelay/releases) include pre-built
executable, ready to use. To install, download zip archive from
[latest release](https://github.com/jstarks/npiperelay/releases), and simply extract
it into a location of your choosing.

In most cases, as with the [usage examples](#usage) below, you need to do
some additional steps in WSL:
- [Make npiperelay available from PATH](#adding-to-path)
- [Install socat](#installing-socat)

## Adding to PATH

In most cases you need to make sure that the npiperelay executable is available in the WSL path.
E.g. if you have put it in `/mnt/c/Users/<username>/go/bin`, this can be achieved either by adding
`C:\Users\<username>\go\bin` to the PATH environment variable in Windows and restarting WSL,
or by just adding the path directly in WSL via the command line or in our `.bash_profile` or `.bashrc`. Alternatively, you can just symlink it into something that's already in your path:

```bash
$ sudo ln -s /mnt/c/Users/<username>/go/bin/npiperelay.exe /usr/local/bin/npiperelay.exe
```

You may be tempted to just put the real binary directly into `/usr/local/bin`, but this will not work because Windows currently cannot run binaries that exist in the Linux namespace -- they have to reside somewhere under the Windows portion of the file system.

## Installing socat

For all of the [usage examples](#usage) below, you will need the excellent `socat` tool.
Your WSL distribution should have it available; install it by running something along the lines of:

```bash
$ sudo apt install socat
```

# Building

You can also build from source. With Go, this is not too difficult:
1. [Install Go](#installing-go)
2. [Download and build](#downloading-and-building)

Then, consider the same additional steps described for installing releases above:
- [Make npiperelay available from PATH](#adding-to-path)
- [Install socat](#installing-socat)

## Installing Go

To build the binary, you will need a version of [Go](https://go.dev).

You can use a Windows build of Go or you can use a Linux build and
cross-compile the Windows binary from within WSL. To build in WSL
you can probably install Go from your distro's package manager.

## Downloading and building

With Go there are different alternative methods for building a project.
In newer versions of Go (version 1.16 and newer) you should preferrable
use one of the following two.

The first and most preferred alternative is a pure local build, where the
source will be in subfolder npiperelay of current working directory,
and resulting executable will be in root of that directory.

```cmd
git clone https://github.com/jstarks/npiperelay.git
cd npiperelay
go build
```

An optional adjustment to this is to add the go build argument -o followed by
a custom path where to put the resulting executable, e.g. to put it directly
into a location already available in [PATH](#adding-to-path).

```cmd
go build -o C:\bin\npiperelay.exe
```

The other alternative is a the Go standard method for installing third party
executables, where both the source and the resulting executable are put in
standard GOPATH. The result will be at path `%GOPATH%\bin\npiperelay.exe`.

```cmd
go install github.com/jstarks/npiperelay@latest
```

As mentioned you can also build from within WSL, using cross-compilation
to Windows executable. Set variable `GOOS=windows` and use similar commands
as above, e.g. 

```bash
git clone https://github.com/jstarks/npiperelay.git
cd npiperelay
GOOS=windows go build -o /mnt/c/bin/npiperelay.exe
```

# Usage

The examples below assume you have copied the contents of the `scripts`
directory into your PATH somewhere. These scripts are just examples and can be
modified to suit your needs.

## Connecting to Docker from WSL

This assumes you already have the Docker daemon running in Windows, e.g. because you have installed Docker for Windows. You may already have the ability to connect to this daemon from WSL via TCP, but this has security problems because any user on your machine will be able to connect. With these steps, you'll be able to limit access to privileged users.

Basic steps:

1. Start the Docker relay.
2. Use the `docker` CLI as usual.

### Starting the Docker relay

For this to work, you will need to be running in an elevated WSL session, or you will need to configure Docker to allow your Windows user access without elevating.

You also need to be running as root within WSL, or launch the command under sudo. This is necessary because the relay will create a file /var/run/docker.sock.

```bash
$ sudo docker-relay &
```

### Using the docker CLI with the relay

At this point, ordinary `docker` commands should run fine as root. Try

```bash
$ sudo docker info
```

If this succeeds, then you are connected. Now try some other Docker commands:

```bash
$ sudo docker run -it --rm microsoft/nanoserver cmd /c "Back in Windows again..."
```

#### Running without root

The `docker-relay` script configured the Docker pipe to allow access by the
`docker` group. To run as an ordinary user, add your WSL user to the docker
group. In Ubuntu:

```bash
$ sudo adduser <my_user> docker
```

Then open a new WSL window to reset your group membership.

## Connect to MySQL Server running as a Windows service

If you run MySQL Server as a Windows service, you can configure it to
communicate through TCP, named pipes or shared memory. If you use named
pipes, connecting to MySQL from WSL is very similar to connecting
to Docker.

The `mysqld-relay` script is designed to be run in a `sudo` shell.
Before creating the relay, it will try to configure your environment
(if it has not been configured yet) by:

* creating `/var/run/mysqld/`,
* creating a `mysql` group, and
* adding your user account to the `mysql` group.

You can of course pull out just the npiperelay command if you don't
need any of the above checks.

Note that if you need to enter a password for sudo, the following
command will fail because of the lack of password input:

```bash
$ sudo mysqld-relay &
```

In that case, you can run it like this:

```bash
user@machine:~$ sudo -s
[sudo] password for user:
root@machine:~# mysqld-relay &
root@machine:~# exit
user@machine:~$ _
```

Now you can use the Linux `mysql` command line client or any other
Linux process that expects to talk to MySQL Server through
`/var/run/mysqld/mysqld.sock`.

## Connecting to a Hyper-V Linux VM's serial console

If you have a Linux VM configured in Hyper-V, you may wish to use its serial
port as a serial console. With npiperelay, this can be done fairly easily from
the command line.

Basic steps:

1. Enable the serial port for your Linux VM.
2. Configure your VM to run the console on the serial port.
3. Run socat to relay between your terminal and npiperelay.

### Enabling the serial port

This is easiest to do from the command line, via the Hyper-V PowerShell cmdlets.
You'll need to add your user to the Hyper-V Administrators group or run the
command line elevated for this to work.

If you have a VM named `foo` and you want to enable the console on COM1 (/dev/ttyS0), with a named pipe name of `foo_debug_pipe`:

```bash
$ powershell.exe Set-VMComPort foo 1 '\\.\pipe\foo_debug_pipe'
```

### Configuring your VM to run the console on the serial port

Refer to your VM Linux distribution's instructions for enabling the serial console:

* [Ubuntu](https://help.ubuntu.com/community/SerialConsoleHowto)
* [Fedora](https://docs.fedoraproject.org/f26/system-administrators-guide/kernel-module-driver-configuration/Working_with_the_GRUB_2_Boot_Loader.html#sec-GRUB_2_over_a_Serial_Console])

### Connecting to the serial port

For this step, WSL must be running elevated or your Windows user must be in the
Hyper-V Administrators group.

#### Directly via socat

The easiest approach is to use socat to connect directly. The `vmserial-connect` script does this and even looks up the pipe name from the VM name and COM port for you:

```bash
$ vmserial-connect foo 1
<enter>
Ubuntu 17.04 gigastarks-vm ttyS0

gigastarks-vm login:
```

Press Ctrl-O to exit the connection and return to your shell.

#### Via screen

If you prefer to use a separate tool to connect to the device such as `screen`, then you must run a separate `socat` process to relay between the named pipe and a PTY. The `serial-relay` script does this
for you with the right parameters; simply run:

```bash
$ serial-relay //./pipe/foo_debug_pipe $HOME/foo-pty & # Starts the relay
$ screen $HOME/foo-pty                                 # Attaches to the serial terminal
```

See the `screen` documentation (`man screen`) for more details.

## Debugging the kernel of a Hyper-V Linux VM

Follow the same steps to enable the COM port for your VM, then run the serial
relay as though you were going to run `screen` to connect to the serial console.

Next, run gdb and connect to the serial port:

```bash
gdb ./vmlinux
target remote /home/<username>/foo-pty
```

## Connect to Windows SSH agent

Windows provides [OpenSSH](https://docs.microsoft.com/en-us/windows-server/administration/openssh/openssh_overview) including `ssh-agent`. If you have [configured](https://docs.microsoft.com/en-us/windows-server/administration/openssh/openssh_server_configuration) the agent to auto start and added your keys, WSL2 can connect to it using a named pipe via `SSH_AUTH_SOCK`.

Add the following to a `.bashrc` or `.zshrc` configuration to setup WSL `ssh-agent` to use Windows agent.

```bash
export SSH_AUTH_SOCK=${HOME}/.ssh/agent.sock
ss -a | grep -q $SSH_AUTH_SOCK
if [ $? -ne 0   ]; then
    rm -f ${SSH_AUTH_SOCK}
    ( setsid socat UNIX-LISTEN:${SSH_AUTH_SOCK},fork EXEC:"npiperelay.exe -ei -s //./pipe/openssh-ssh-agent",nofork & ) >/dev/null 2>&1
fi
```

## Custom usage

Take a look at the scripts for sample usage, or run `npiperelay.exe` without any parameters for parameter documentation.