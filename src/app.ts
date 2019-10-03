import * as fs from 'fs';
import * as path from 'path';

import { Logger } from '@claw/types';
import ConsoleLogger from '@claw/util/logger';
import { CmdArgs } from '@claw/commands';
import { EventEmitter } from 'events';
import * as packageJson from '@claw/../package.json';
import { AppConfig } from '@claw/config';

class App extends EventEmitter {
    argv: CmdArgs;
    config: AppConfig;
    commandName: string;
    logger: Logger = new ConsoleLogger();
    env: string; // TODO this concept does not fit well here
    DEBUG = 0;
    version: string = packageJson.version;

    build({ argv, logger }: { argv: CmdArgs; logger?: Logger }) {
        this.argv = argv;
        this.DEBUG =
            argv.verbose === false
                ? 0
                : argv.verbose === true
                    ? 1
                    : argv.verbose;

        if (logger) {
            this.logger = logger;
        }

        this.config = new AppConfig(argv);
        this.commandName = argv._ ? argv._.join(' ') : '';
    }

    path(dirOrFile) {
        return path.join(this.config.data.home, dirOrFile);
    }

    info = this.logger.info.bind(this.logger);
    warn = this.logger.warn.bind(this.logger);
    error = this.logger.error.bind(this.logger);
    echo = this.logger.echo.bind(this.logger);
    milestone = this.logger.milestone.bind(this.logger);
    log = this.logger.info.bind(this.logger);

    debug(msg, ...args) {
        if (!this.DEBUG) return;
        this.logger.debug(msg, ...args);
    }

    fail(msg = 'system failure (no reason)', ...args) {
        this.logger.fatal(1, msg, ...args);
    }

    exitHandler = async signal => {
        this.echo('\n');
        this.warn(`cla-worker exiting on request signal=${signal}`);
        for (const listener of this.listeners('exit')) {
            await listener();
        }
        process.exit(2);
    };

    async startup() {
        //do something when app is closing
        process.on('SIGTERM', this.exitHandler);
        // process.on('exit', this.exitHandler);

        //catches ctrl+c event
        process.on('SIGINT', this.exitHandler);

        // catches "kill pid"
        process.on('SIGUSR1', this.exitHandler);
        process.on('SIGUSR2', this.exitHandler);

        //catches uncaught exceptions
        process.on('uncaughtException', this.exitHandler);

        return;
    }

    registry() {
        // TODO load registry here, from multiple special registry files (js or yaml?)
        //  located in server/registry/* and from plugins
    }

    loadPlugins() {
        /// TODO load all plugin code
    }

    daemonize() {
        const { logfile, pidfile } = this.config.data;

        fs.writeFileSync(pidfile, `${process.pid}\n`);
        const access = fs.createWriteStream(logfile);
        process.stdout.write = process.stderr.write = access.write.bind(access);
    }

    spawnDaemon() {
        const { id, logfile, pidfile } = this.config.data;

        this.info(`logfile=${logfile}`);
        this.info(`pidfile=${pidfile}`);

        const [isRunning, pid] = this.isDaemonRunning();
        if (isRunning) {
            this.fail(
                `cannot start, another daemon is already running for id=${
                    this.config.data.id
                } and pid=${pid}`
            );
        }

        const { spawn } = require('child_process');

        const isNode = process.argv[0] === 'node' ? true : false;

        const cmd = process.argv[isNode ? 1 : 0],
            args = process.argv.slice(isNode ? 2 : 1);

        this.debug(`cmd=${cmd}`);
        this.debug(`args=${args}`);

        const subprocess = spawn(cmd, args, {
            env: Object.assign({}, process.env, {
                CLARIVE_WORKER_FORKED: 1
            }),
            detached: true,
            stdio: 'ignore'
        });

        this.info(`forked child with pid ${subprocess.pid}`);
        this.info(`waiting for daemon to start...`);

        setTimeout(() => {
            try {
                process.kill(subprocess.pid, 0);
                this.milestone(
                    `workerid ${id} and pid=${subprocess.pid} started.`
                );
            } catch (err) {
                this.error(
                    `worker with pid=${
                        subprocess.pid
                    } did not start successfully`
                );
            }
            subprocess.unref();
        }, 5000);
    }

    getPid(pidfile): number {
        if (!pidfile) {
            this.fail('missing pidfile');
        }

        if (!fs.existsSync(pidfile)) {
            this.fail(`could not find daemon, no pidfile exists at ${pidfile}`);
        }

        const pidBuf = fs.readFileSync(pidfile);
        return parseInt(pidBuf.toString(), 10);
    }

    deletePidfile(pidfile) {
        try {
            fs.unlinkSync(pidfile);
            this.info(`deleted '${pidfile}'`);
        } catch (err) {
            this.warn(`could not delete pidfile ${pidfile}`, err);
        }
    }

    killDaemon(pidfile: string): Promise<boolean> {
        return new Promise((resolve, reject) => {
            const { id } = this.config.data;
            const pid = this.getPid(pidfile);

            this.info(
                `stopping daemon with pid=${pid}, from pidfile=${pidfile}`
            );

            try {
                process.kill(pid, 15);
                this.info(`killed daemon with pid=${pid}`);
            } catch (err) {
                this.deletePidfile(pidfile);
                reject(
                    `process pid=${pid} is not running or cannot be killed (SIG 15)`
                );
            }

            this.info(`waiting for daemon to stop...`);

            let maxWait = 10;

            const timeout = setInterval(() => {
                try {
                    process.kill(pid, 0);

                    if (--maxWait <= 0) {
                        clearInterval(timeout);
                        reject(
                            `worker with pid=${pid} did not stop successfully`
                        );
                    }
                } catch (err) {
                    clearInterval(timeout);
                    this.milestone(`workerid ${id} pid=${pid} stopped.`);
                    this.deletePidfile(pidfile);
                    resolve(true);
                }
            }, 1000);
        });
    }

    isDaemonRunning(): [boolean, number] {
        const { pidfile } = this.config.data;

        if (!fs.existsSync(pidfile)) {
            return [false, null];
        }

        const pid = this.getPid(pidfile);
        try {
            process.kill(pid, 0);
            return [true, pid];
        } catch (err) {
            return [false, null];
        }
    }
}

export default new App();
