# Clarive Worker

The Clarive Worker is one of the available communication methods used for
sending/retrieving files and running commands on remote servers.

The Worker is an executable file that needs to be installed in each
server in your network where you may want to run certain DevOps operations,
which typically are executing local shell commands, sending files (ship)
and retrieving files (fetch).

The Worker is a _pull_ agent, meaning it initiates the connection to the Clarive
pubsub server and awaits for instructions. It is ideal for cases where the server hosting
the worker runs behind a firewall and is not directly reacheable by the Clarive
server through SSH or a push agent such as ClaX.

#### Supported OS/Platforms

The Worker supports 3 main platforms:

- Linux releases 2.6 or greater, including CentOS 5.x or greater
- Windows 7.x, 8.x, 10.x and Windows Server 2003 or greater
- MacOS 10.12 or greater

#### Installing the Worker

The Clarive Worker is a single binary and has no specific prerequisites. The
only requisite is that the server where the worker is being installed
**can reach the Clarive server directly**.

Typically the process to connecting your Clarive Server to the rest of your
infrastructure for building, deploying and other DevOps-related activities is
this:

- Download the Clarive Worker binary `cla-worker` in the server
- Register the worker with the server by using a project **passkey**
- Start the worker online or as a daemon
- Run your Clarive rules and pipeline against the worker

#### Clarive server requisites

For the worker to be able to connect, the Clarive web server
must be running with the `pubsub` module active.

By default the pubsub module is active and starts with the rest of
`cla web-start` web services.

#### Worker Capabilities

A worker instance can execute the following actions
on its host server:

- receive a remote file from the Clarive server and write it locally
- send a local file to the Clarive server
- run any arbitary command locally on the server as requested by the Clarive
  server
- eval arbitary Javascript instructions on the server, using exposed or
  imported NodeJS libraries

#### Worker Security

The Worker only runs as the current OS user it has been launched with.  But
commands send by the server could escalate permissions if the user or
executable have some type `sudo` permissions, sticky bit or similar mechanism.

#### Download the Worker

The Clarive Worker is open source and can be downloaded from
the Clarive Github account:

     https://github.com/clarive/cla-worker/releases

The worker is a self-contained binary available for Linux, MacOS and Windows
and has no prerrequisites.

#### Registering the Worker

For a worker to connect to a Clarive server it needs a `passkey`. Passkeys
are available on a project basis. Each Clarive project may or may not have a passkey,
if the project owner or administrator generates one.

    cla-worker register --passkey 123428198291ad98d98c89b8

The registration process will return a ID-token pair that is unique
to this worker instance.

    ℹ Registration token:  97d317df5ad3fbb68334657ec94aefe6
    ℹ Projects registered:  ["CLARIVE"]
    ℹ Start the worker with the following command:

                cla-worker run --id RKmp3hSwb --token 97d317df5ad3fbb68334657ec94aefe6

The registration token returned is the worker "password" to access the server under
a given ID.

**important**

The ID-worker pair is analogous to a username/password for
your worker instance. Keep it in a safe place. If it's compromised
an **attacker could impersonate** the worker by making the Clarive
Server believe it's connected to the correct server when in fact it's
connected to the attacker's infrasctructure. That could end up
with sensitive information or files being sent by the Clarive Server
to the compromised worker.

#### `cla-worker.yml`

To keep your registrations safe, use the `cla-worker.yml` file.

By default the `cla-worker.yml` file is loaded from one of the
following 4 possibilities, in order of precedence:

1. from the parameter `--config filepath` or `-c /path/to/cla-worker.yml` is present
2. from the environment variable `CLA_WORKER_CONFIG=/path/to/cla-worker.yml`
3. from the current working directory, from where the binary is started
4. from the user's home directory (environment variable `HOME`)
5. from `/etc/cla-worker.yml`

To save your registration directly to the `cla-worker.yml` file,
use the `--save` or `--save /path/to/cla-worker.yml`:

     cla-worker register --save --passkey 123428198291ad98d98c89b8

#### Structure of the `cla-worker.yml` file

These are the configuration parameters allowed in the config file:

`id` - the unique identifier of the worker
`token` - the token returned by the registration process
`passkey` - the project access key for registering workers
`verbose` - to run the worker in verbose mode, set it to `verbose: true`
`tags` - a comma separated list of tags
`envs` - a comma separated list of environments

#### Assigning an ID to the Worker

Every worker, once registered, will have a corresponding random unique ID
assigned to it.

To make it easier to identify your worker, you can optionally assign it a unique
ID.

    cla-worker register --id myworker --passkey 2323928198291ad98d98c89b8

You can register a worker with the same ID against many projects, but
**never the same worker ID twice for a given project**.

#### Unregistering the worker

To unregister a given Worker from the server, making it available to
re-register with the same ID or simply to decatalog it from the available
worker list, run the following command:

    cla-worker unregister --id myworker --token [token]

You must know the `token` corresponding to the `id` to be able to
unregister a worker.

#### Running the worker

Once registered, fire-up the worker with the following command using the
ID and token used in the registration:

    cla-worker run --id myworker --token 97d317df5ad3fbb68334657ec94aefe6

Or simply, if you have a `cla-worker.yml` file in your PATH with only one
registration (ID-token pair):

    cla-worker run

Optionally, if you have multiple ID registrations in your `cla-worker.yml` file:

    cla-worker run --id myworker

#### Worker management UI

Available under the project area, `Deploy -> Workers`.

From there you can:

- Obtain the passkey for registering workers for a given project.

- Monitor worker usage.

- Unregister workers, which removes the worker from this project.

- Shutdown workers, which kills the process on the servers.

- Disable workers, which does not kill the process on the server
but prevents new jobs from using it.

**warning**

Shutting down workers will kill the server process.
After a shut down, restarting the worker requires manually
logging into the server and starting the process.

#### Limiting Workers by environment

To limit what environments are available for a given worker,
limiting it's usage to say `QA` or `PROD`, use the following
flags in the `cla-worker` command line:

```bash
$ cla-worker run --id myworkerid --envs QA,PROD

# Or by separating each entry:

$ cla-worker run --id myworkerid --env QA --env PROD
```

**note**

The environments need to exist in Clarive for the
worker to start successfully. The environment names
are case-sensitive.

#### Setting worker tags

Worker tags are useful to identify a worker capabilities.
Then, when writing rulebooks that make use of a worker,
you can ask for any available worker with a given capability.

Examples of useful tags could be `java` (can build java projects),
`gcc` (C compiler), `nodejs`, etc.

```bash
$ cla-worker run --id myworkerid --tags java,nodejs

# Or by separating each entry:

$ cla-worker run --id myworkerid --tag java --tag nodejs
```

Then invoke your worker within a rulebook with the following options:

```bash
do:
    shell:
       worker: { tags: ['java'] }
       cmd: javac MyClass.java
```

This will run your command on the first worker that supports
the tag  `java` (meaning it has a Java compiler available):

One could have several workers within a given server, with
different `id`s and `tag` sets for different capabilities.

#### Registering worker to more than one project

This is not recommended due to the fact that it creates possible breach of
project information among projects.

If you want to have 2 or more projects running at the same `user@hostname`
pair, we recommend having one `workerid` in a _per project_ basis, instead of
sharing the same worker for more than one project.

Still, if instead you prefer to share the same workerid with different
projects, you __can register__ your worker to more than one project by
re-registering an already register worker with the new project's `passkey`.

Bear in mind that workers registered to more than one project carry the
following caveats:

- users can see which projects share a worker through the Worker management UI.

- if a user in one project shuts down or disables a shared worker, the worker
  will become unavailable to all registered projects.

- unregistering a worker, however, from one project will NOT unregister it from
  all projects.

#### Starting as a daemon

To run the Clarive Worker as a daemon, ie. to start the process
in the background and get control back into the shell, run the following
command:

```bash
cla-worker start --id myworker --token 97d317df5ad3fbb68334657ec94aefe6
ℹ spawning cla-worker in the background...
ℹ logfile=/opt/cla-worker/cla-worker.log
ℹ pidfile=/opt/cla-worker/cla-worker-myworker.pid
ℹ forked child with pid 16412
```

The process id is stored in the `pidfile` while a detailed execution log is
stored in `logfile`. These can be controlled by sending the option `--logfile`
and `--pidfile` with the complete path to the files.

To check the status of the daemon, the `cla-worker status` command can come handy:

```bash
$ cla-worker status --id myworker
ℹ checking status for workerid=myworker and pidfile=/opt/cla-worker/cla-worker-myworker.pid
ℹ logfile=/opt/cla-worker/cla-worker.log
ℹ workerid=myworker is assigned pid=27532...
✔ worker is running with pid=27532
```

To stop the worker daemon, use the following:

```bash
cla-worker stop --id myworker
ℹ stopping daemon with pid=16412, from pidfile=/opt/cla-worker/cla-worker-myworker.pid
ℹ killed daemon with pid=16412
ℹ deleted '/opt/cla-worker/cla-worker-myworker.pid'
```

The `stop` command will look for a `pidfile` in the default location for a given
`id`. If the `pidfile` sits somewhere else, pass the stop command the path to
the `pidfile` with `cla-worker stop --pidfile [pidfile]`.
