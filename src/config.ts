import app from '@claw/app';

import * as fs from 'fs';
import * as path from 'path';
import * as YAML from 'js-yaml';

import { CmdArgs } from '@claw/commands';
import { PartialProperties } from '@claw/types';

export type Registration = {
    id: string;
    token: string;
    url?: string;
};

export class AppConfigData {
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

    constructor(data?: PartialProperties<AppConfigData>) {
        Object.assign(this, data);
    }
}

export class AppConfig {
    data: AppConfigData;
    file: string;

    constructor(argv?: Partial<CmdArgs>) {
        const argvDefaults = Object.keys(argv)
            .filter(it => !/^[_\$]/.test(it))
            .reduce((obj, key) => ((obj[key] = argv[key]), obj), {});

        const [loadedData, configPath] = this.load(
            argv.config,
            !argv.save && argv.config != null
        );

        const configData: AppConfigData = {
            ...argvDefaults, // argv defaults
            ...loadedData, // user config file
            ...argv._opts // user cmd line
        };

        configData.tags = this.makeArray(configData, 'tags', 'tag');
        configData.envs = this.makeArray(configData, 'envs', 'env');

        if (!configData.pidfile) {
            const prefix = `${process.cwd()}/cla-worker`;
            configData.pidfile =
                configData.id != null
                    ? `${prefix}-${configData.id}.pid`
                    : `${prefix}.pid`;
        }

        const { registrations } = configData;

        if (Array.isArray(registrations) && registrations.length > 0) {
            if (configData.id && !configData.token) {
                registrations.forEach(registration => {
                    if (registration.id === configData.id) {
                        configData.token = registration.token;
                        if (
                            registration.url != null &&
                            argv._opts.url === undefined
                        ) {
                            configData.url = registration.url;
                        }
                    }
                });
            } else if (!configData.id && registrations.length === 1) {
                configData.id = registrations[0].id;
                configData.token = registrations[0].token;
            }
        }

        app.debug(
            `config file set to=${configPath}, ${argv.config}, ${argv.save}`
        );

        this.data = configData;
        this.file = configPath;
    }

    candidates(argvConfig): string[] {
        const upperAppName = app.name.toUpperCase();
        const APP_HOME = process.env[`${upperAppName}_HOME`] || process.cwd();
        return [
            argvConfig,
            process.env[`${upperAppName}_CONFIG`],
            path.join(APP_HOME, `./${app.name}.yml`),
            path.join(process.env.HOME, `./${app.name}.yml`),
            path.join(`/etc/${app.name}.yml`)
        ].filter(it => it != null && typeof it !== 'boolean');
    }

    load(
        argvConfig: string | boolean,
        mustExist = false
    ): [AppConfigData, string] {
        const configCandidates: string[] = this.candidates(argvConfig);

        app.debug('config candidates', configCandidates);

        for (const configPath of configCandidates) {
            app.debug(`checking for config file at ${configPath}...`);

            if (!fs.existsSync(configPath)) {
                if (argvConfig === configPath) {
                    if (mustExist) {
                        throw `can't load config file: '${configPath}' not found`;
                    } else {
                        break;
                    }
                } else {
                    continue;
                }
            }

            app.debug(`found ${configPath}, loading...`);

            try {
                const baseFile = fs.readFileSync(configPath, 'utf8');
                return [new AppConfigData(YAML.safeLoad(baseFile)), configPath];
            } catch (err) {
                throw `failed to load config file ${configPath}: ${err}`;
            }
        }

        return [new AppConfigData(), configCandidates[0]];
    }

    save(data: PartialProperties<AppConfigData>) {
        // in case the config file has been changed since the app loaded...
        const [currentConfig] = this.load(this.file);

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

        app.debug(`saving config to file '${this.file}'...\n`, dump);

        try {
            fs.writeFileSync(this.file, dump, 'utf8');
        } catch (err) {
            throw `failed to save config file '${this.file}': ${err}`;
        }

        return [this.file, dump];
    }

    makeArray(configData: AppConfigData, keys: string, key?: string) {
        let arr: string[];

        if (configData[keys] == null && configData[key]) {
            arr =
                configData[key] === 'string'
                    ? configData[key].split(',')
                    : configData[key];
        } else {
            arr =
                configData[keys] === 'string'
                    ? configData[keys].split(',')
                    : configData[keys];
        }

        app.debug(keys, arr);
        return arr;
    }
}
