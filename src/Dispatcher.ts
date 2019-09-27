import app from '@claw/app';
import { spawn } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import * as util from 'util';
import * as vm from 'vm';

import { length } from '@claw/common';
import PubSub from '@claw/pubsub';

import { Writable } from 'stream';

const fsAsync = {
    rename: util.promisify(fs.rename)
};

type EvalResult = {
    output: string;
    error?: string;
    ret?: any;
};

export default class Dispatcher {
    message: any;
    pubsub: PubSub;
    cmd: string;
    msgId: string;
    config: any;

    constructor(pubsub, config, cmd, msgId, message) {
        this.pubsub = pubsub;
        this.cmd = cmd;
        this.msgId = msgId;
        this.message = message;
        this.config = config;
    }

    async process() {
        app.info('dispatching message: %s %s', this.msgId, this.cmd);

        if (this.cmd !== 'worker.shutdown') {
            await this.pubsub.publish(`${this.cmd}.ack`, { oid: this.msgId });
        }

        try {
            switch (this.cmd) {
                case 'worker.ready':
                    break;
                case 'worker.put_file':
                    await this.cmdPutFile(this.message);
                    break;
                case 'worker.get_file':
                    await this.cmdGetFile(this.message);
                    break;
                case 'worker.shutdown':
                    app.warn(
                        `shudown event received from the server. Reason: ${
                            this.message.reason
                        }`
                    );
                    app.warn('trying to stop gracefully...');
                    await this.pubsub.close();
                    app.milestone(
                        'worker shutdown on server request completed'
                    );
                    process.exit(10);
                    break;
                case 'worker.eval':
                    await this.cmdEval(this.message);
                    break;
                case 'worker.capable':
                    this.cmdCapable(this.message);
                    break;
                case 'worker.file_exists':
                    this.cmdFileExists(this.message);
                    break;
                case 'worker.exec':
                    await this.cmdExec(this.message);
                    break;
                default:
                    this.publishError(
                        util.format(
                            'Invalid command %s in message id=%s',
                            this.cmd,
                            this.msgId
                        )
                    );
            }
        } catch (err) {
            this.publishError(err);
            this.done();
        }
    }

    publishError(err) {
        if (typeof err == 'object') {
            err = err.toString();
        }
        err = `during command ${this.cmd}: ${err}`;
        app.error(err);

        let result = {
            ret: err,
            rc: 99,
            output: err
        };

        this.result(result);
    }

    cmdCapable({ tags }) {
        const myTags: string[] = app.config.tags || [];

        if (tags.every(_ => myTags.includes(_))) {
            this.pubsub.publish('worker.capable.reply', {
                oid: this.msgId,
                workerid: this.pubsub.id,
                tags: myTags
            });
        }
    }

    cmdFileExists({ path }) {
        const myTags: string[] | string = app.config.tags || [];

        const exists = fs.existsSync(path);

        this.pubsub.publish('worker.file_exists.reply', {
            oid: this.msgId,
            workerid: this.pubsub.id,
            exists: exists ? 1 : 0
        });
    }

    async cmdEval({ code, stash }) {
        const { pubsub } = this;
        let result: EvalResult = { output: '' };

        const consoleWrapper = {
            log: (...args) => {
                args.forEach(_ => (result.output += _));
                app.info('worker.eval log:', args);
            }
        };
        const context = vm.createContext({
            console: consoleWrapper,
            fs,
            ...stash
        });

        try {
            result.ret = await vm.runInContext(code, context);
        } catch (error) {
            app.error(`worker.eval error: ${error}`);
            result.error = error.toString();
        }

        app.debug('eval result', result);

        pubsub.publish('worker.eval.done', {
            oid: this.msgId,
            ...result
        });
    }

    cmdExec({ cmd }) {
        const [cmdName, ...cmdArgs] = Array.isArray(cmd) ? cmd : [cmd];
        const cmdOpts: any = {};

        if (length(this.message.chdir)) cmdOpts.cwd = this.message.chdir;
        if (!cmdArgs.length) cmdOpts.shell = true;

        app.debug(
            'Running cmd=%s, args=%s, opts=%s',
            cmdName,
            cmdArgs,
            JSON.stringify(cmdOpts)
        );

        let output = '';

        const proc = spawn(cmdName, cmdArgs, cmdOpts).on('error', err => {
            let { message } = err;

            if (/ENOENT/.exec(message)) {
                message = util.format(
                    "Could not run command '%s' with options: %s",
                    cmdName,
                    JSON.stringify([cmdArgs, cmdOpts])
                );
            }

            this.publishError(message);
        });

        proc.stdout.on('data', data => {
            output += data;
            app.debug(`stdout: ${data}`);
        });

        proc.stderr.on('data', data => {
            output += data;
            app.debug(`stderr: ${data}`);
        });

        proc.on('close', code => {
            let result = {
                ret: '',
                rc: code,
                output: output
            };

            this.result(result);
            this.done();
        });
    }

    async cmdGetFile({ filepath, filekey }) {
        const { pubsub } = this;

        if (!filekey) {
            throw 'Missing filekey in get_file operation';
        }

        if (!filepath) {
            throw 'Missing filepath in get_file operation';
        }

        try {
            const readStream = fs.createReadStream(filepath, {
                highWaterMark: this.config.chunk_size
            });

            readStream.on('error', err => {
                const errMsg = `could not read file ${filepath}: ${err.toString()}`;

                app.error('get_file: read stream error', err);

                pubsub.publish('worker.get_file.fail', {
                    oid: this.msgId,
                    filepath,
                    filekey,
                    error: errMsg
                });
            });

            await pubsub.push(filekey, filepath, readStream);

            readStream.close();
            await pubsub.publish('worker.get_file.done', {
                oid: this.msgId,
                filekey,
                filepath
            });

            app.milestone(`get_file done ${filepath}`);
        } catch (err) {
            await pubsub.publish('worker.put_file.fail', {
                oid: this.msgId,
                filekey,
                filepath,
                error: err.toString()
            });

            app.error(err.toString());
        }
    }

    async cmdPutFile({ filepath, filekey }) {
        const { pubsub } = this;
        const onError: Array<(err: Error) => void> = [];

        if (!filekey) {
            throw 'Missing filekey in put_file operation';
        }

        if (!filepath) {
            throw 'Missing filekey in put_file operation';
        }

        let lastError = '';

        try {
            const tmpfile = [filepath, filekey, 'temp'].join('.');
            app.debug('creating temporary file %s', tmpfile);
            const tmpdir = path.dirname(tmpfile);

            if (!fs.existsSync(tmpdir)) {
                await fs.promises.mkdir(tmpdir, { recursive: true });
            }

            let stream = fs.createWriteStream(tmpfile, { flags: 'w' });

            stream.on('error', err => {
                lastError = err.toString();

                if (fs.existsSync(tmpfile)) {
                    fs.unlinkSync(tmpfile);
                }

                const errMsg = `could not write temporary file ${tmpfile}: ${err.toString()}`;

                app.error('put_file: write stream error', err);

                pubsub.publish('worker.put_file.fail', {
                    oid: this.msgId,
                    filekey,
                    filepath,
                    error: errMsg
                });
            });

            onError.push(err => {
                if (fs.existsSync(tmpfile)) fs.unlinkSync(tmpfile);
            });

            stream.on('finish', async () => {
                try {
                    await fsAsync.rename(tmpfile, filepath);
                    await pubsub.publish('worker.put_file.done', {
                        oid: this.msgId,
                        filekey,
                        filepath
                    });
                    app.milestone(`put_file: wrote file ${filepath}`);
                } catch (err) {
                    onError.forEach(ev => {
                        ev(err);
                    });
                    await pubsub.publish('worker.put_file.fail', {
                        oid: this.msgId,
                        filekey,
                        filepath,
                        error: lastError
                    });
                    app.error(
                        `could not write file ${filepath} from temp file ${tmpfile}`,
                        err
                    );
                }
            });

            await pubsub.pop(filekey, stream);
        } catch (err) {
            await pubsub.publish('worker.put_file.fail', {
                oid: this.msgId,
                filekey,
                filepath,
                error: err.toString()
            });

            onError.forEach(ev => {
                ev(err);
            });

            app.error(err.toString());
        }
    }

    async result(result) {
        await this.pubsub.publish('worker.result', {
            oid: this.msgId,
            ...result
        });
    }

    async done() {
        await this.pubsub.publish('worker.done', {
            oid: this.msgId
        });
    }
}
