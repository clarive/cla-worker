import * as colors from 'colors'; // eslint-disable-line
require('colors'); // otherwise we get "undefined" messages

import { Logger, LogMessage } from '@claw/types';

export default class ConsoleLogger implements Logger {
    printer: (...any) => void = console.error;

    dump(msg: LogMessage): string {
        return typeof msg === 'object' && !(msg instanceof Error)
            ? JSON.stringify(msg)
            : (msg as string);
    }

    milestone(msg: LogMessage, ...args): void {
        msg = this.dump(msg);
        this.printer(
            `✔ ${msg.white}`.green,
            ...args.map(arg => this.dump(arg))
        );
    }

    info(msg: LogMessage, ...args): void {
        msg = this.dump(msg);
        this.printer(`ℹ ${msg.white}`.blue, ...args.map(arg => this.dump(arg)));
    }

    warn(msg: LogMessage, ...args): void {
        msg = this.dump(msg);
        this.printer(
            `⚠️ ${msg}`.yellow,
            ...args.map(arg => this.dump(arg))
        );
    }

    debug(msg: LogMessage, ...args): void {
        msg = this.dump(msg);
        this.printer(`[debug] ${msg}`.grey, ...args.map(arg => this.dump(arg)));
    }

    error(msg: LogMessage, ...args): void {
        msg = this.dump(msg);
        this.printer(`✘ ${msg.bold}`.red, ...args.map(arg => this.dump(arg)));
    }

    echo(msg: LogMessage, ...args): void {
        msg = this.dump(msg);
        this.printer(msg, ...args.map(arg => this.dump(arg)));
    }

    fatal(code: number | LogMessage, msg: LogMessage, ...args): void {
        if (typeof code === 'number') {
            msg = this.dump(msg);
        } else {
            msg = this.dump(code);
            args = [msg, ...args];
            code = 1;
        }

        this.printer(
            `[fail] ${msg.white.bold}`.red.bold,
            ...args.map(arg => this.dump(arg))
        );

        process.exit(code);
    }
}
