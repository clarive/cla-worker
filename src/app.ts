import * as fs from 'fs';
import * as path from 'path';
import * as YAML from 'js-yaml';

import { Logger } from '@claw/types';
import ConsoleLogger from '@claw/util/logger';
import { CmdArgs } from '@claw/commands';
import { EventEmitter } from 'events';
import * as packageJson from '@claw/../package.json';

type Registration = {
    id: string;
    token: string;
};

class AppConfig {
    id: string;
    token: string;
    url: string;
    origin: string;
    daemon: boolean;
    passkey: string;
    home: string;
    logfile: string;
    pidfile: string;
    registrations: Registration[];
    tags: string[] | string;
    envs: string[] | string;
}

class App extends EventEmitter {
    argv: CmdArgs;
    config: AppConfig;
    commandName: string;
    configFile: string;
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

        this.config = this.configure(argv);
        this.commandName = argv._ ? argv._.join(' ') : '';
    }

    path(dirOrFile) {
        return path.join(this.config.home, dirOrFile);
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

    configure(argv) {
        const [configData, configFile] = this.loadConfigFile(
            argv.config,
            argv.save
        );

        this.configFile = configFile;

        const config = {
            ...this.config,
            ...configData
        };

        Object.keys(argv).map(key => (config[key] = argv[key]));

        config.tags = this.makeArray(config, 'tags', 'tag');
        config.envs = this.makeArray(config, 'envs', 'env');

        if (!config.pidfile) {
            const prefix = `${process.cwd()}/cla-worker`;
            config.pidfile =
                config.id != null
                    ? `${prefix}-${config.id}.pid`
                    : `${prefix}.pid`;
        }

        const { registrations } = config;

        if (Array.isArray(registrations) && registrations.length > 0) {
            if (config.id && !config.token) {
                registrations.forEach(registration => {
                    if (registration.id === config.id) {
                        config.token = registration.token;
                    }
                });
            } else if (!config.id && registrations.length === 1) {
                config.id = registrations[0].id;
                config.token = registrations[0].token;
            }
        }

        return config;
    }

    configCandidates(argvConfig): string[] {
        const CLA_WORKER_HOME = process.env.CLA_WORKER_HOME || process.cwd();
        return [
            argvConfig,
            process.env.CLA_WORKER_CONFIG,
            path.join(CLA_WORKER_HOME, './cla-worker.yml'),
            path.join(process.env.HOME, './cla-worker.yml'),
            path.join('/etc/cla-worker.yml')
        ].filter(it => it != null && typeof it !== 'boolean');
    }

    loadConfigFile(
        argvConfig: string | boolean,
        newFile: boolean = false
    ): [AppConfig, string] {
        const configCandidates: string[] = this.configCandidates(argvConfig);

        for (const configPath of configCandidates) {
            this.debug(`checking for config file at ${configPath}...`);

            if (!fs.existsSync(configPath)) {
                if (newFile) {
                    continue;
                } else if (configPath === argvConfig) {
                    throw `invalid config file '${configPath}'`;
                } else {
                    continue;
                }
            }

            this.debug(`found ${configPath}, loading...`);

            try {
                const baseFile = fs.readFileSync(configPath, 'utf8');
                return [YAML.safeLoad(baseFile), configPath];
            } catch (err) {
                throw `failed to load config file ${configPath}: ${err}`;
            }
        }

        return [new AppConfig(), configCandidates[0]];
    }

    saveConfigFile(data) {
        const [currentConfig, configPath] = this.loadConfigFile(
            this.configFile,
            true
        );

        const registrations = data.registrations;
        delete data.registrations;

        const newConfig = { registrations: [], ...currentConfig, ...data };

        if (registrations) {
            const regMap = {};

            newConfig.registrations.forEach(reg => (regMap[reg.id] = reg));
            registrations.forEach(reg => (regMap[reg.id] = reg));
            newConfig.registrations = Object.values(regMap);
        }

        const dump = YAML.safeDump(newConfig, {
            indent: 4,
            condenseFlow: true
        });

        this.debug(`saving config to file '${configPath}'...`);

        try {
            fs.writeFileSync(configPath, dump, 'utf8');
        } catch (err) {
            throw `failed to save config file '${configPath}': ${err}`;
        }

        return [configPath, dump];
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

    makeArray(config: AppConfig, keys: string, key?: string) {
        let arr: string[];

        if (config[keys] == null && config[key]) {
            arr =
                config[key] === 'string' ? config[key].split(',') : config[key];
        } else {
            arr =
                config[keys] === 'string'
                    ? config[keys].split(',')
                    : config[keys];
        }

        this.debug(keys, arr);
        return arr;
    }

    daemonize() {
        const { logfile, pidfile } = this.config;

        fs.writeFileSync(pidfile, `${process.pid}\n`);
        const access = fs.createWriteStream(logfile);
        process.stdout.write = process.stderr.write = access.write.bind(access);
    }

    spawnDaemon() {
        const { id, logfile, pidfile } = this.config;

        this.info(`logfile=${logfile}`);
        this.info(`pidfile=${pidfile}`);

        const [isRunning, pid] = this.isDaemonRunning();
        if (isRunning) {
            this.fail(
                `cannot start, another daemon is already running for id=${
                    this.config.id
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
            const { id } = this.config;
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
        const { pidfile } = this.config;

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
