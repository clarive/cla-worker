import * as yargs from 'yargs';
import { origin } from '@claw/common';

export interface CmdArgs extends yargs.Arguments {
    id?: string;
    token?: string;
    passkey?: string;
    config?: string | boolean;
    tags?: string;
    save?: boolean;
    verbose?: boolean | number;
    _opts?: CmdArgs;
}

const commons = {
    workerid: {
        alias: 'id',
        describe: 'sets the worker id'
    },
    passkey: {
        required: true,
        describe: 'the server registration passkey'
    },
    token: {
        describe: 'set the registered worker connection token'
    },
    verbose: {
        alias: 'v',
        count: true
    },
    config: {
        alias: 'c',
        describe: 'path to config file',
        type: 'string'
    },
    save: {
        type: 'boolean',
        default: false,
        describe: 'save registration to config file'
    },
    daemon: {
        alias: 'fork',
        type: 'boolean',
        default: false,
        describe: 'run worker in the background'
    },
    logfile: {
        default: `${process.cwd()}/cla-worker.log`,
        describe: 'path to log file'
    },
    pidfile: {
        describe: 'path to daemon pid file'
    },
    tags: {
        alias: 'tag',
        type: 'array',
        describe: 'set the worker tags'
    },
    envs: {
        alias: 'env',
        type: 'array',
        describe: 'set the worker environments'
    },
    origin: {
        default: origin(),
        describe: 'current user@hostname'
    },
    url: {
        default: 'http://localhost:8080',
        describe: 'set the base server url'
    }
};

export function commonOptions(args: yargs.Argv, ...keys) {
    const userArgv = {};
    const userDefaults = {};

    for (const key of keys) {
        const opt = commons[key];

        if (!opt) {
            throw `invalid common option '${key}'`;
        }

        userDefaults[key] = opt.default;
        delete opt.default;
        const userCoerce = opt.coerce;

        // save cmd line params separatedly
        opt.coerce = val => {
            userArgv[key] = val;
            return userCoerce ? userCoerce(val) : val;
        };

        args.option(key, opt);
    }

    args.middleware(argv => {
        argv['_opts'] = userArgv;

        keys.forEach(key => {
            if (userDefaults[key] !== undefined && argv[key] === undefined) {
                const def = userDefaults[key];
                argv[key] = typeof def === 'function' ? def(argv, key) : def;
            }
        });
    });

    return args;
}
