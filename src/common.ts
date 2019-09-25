import app from '@claw/app';
import * as yaml from 'js-yaml';
import * as os from 'os';
import * as fs from 'fs';
import * as generate from 'shortid';

export const isForked: () => boolean = () =>
    !!process.env['CLARIVE_WORKER_FORKED'];

export const length = arg => {
    return arg !== undefined && arg !== null && arg.length > 0;
};

export const readConfig = file => {
    app.info('Parsing config file:', file);

    try {
        const configFromFile = yaml.safeLoad(fs.readFileSync(file, 'utf8'));
        return configFromFile;
    } catch (e) {
        app.error(e.toString());
        process.exit(12);
    }
};

export const workerid = () =>
    `${os.userInfo().username}@${os.hostname()}/${generate()}`;

export const origin = () =>
    `${os.userInfo().username}@${os.hostname()}/${process.pid}`;

export const sleep = (t: number): Promise<number> => {
    return new Promise(resolve => {
        setTimeout(() => {
            resolve(t);
        }, t);
    });
};
