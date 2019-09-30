import app from '@claw/app';
import * as yargs from 'yargs';
import { commonOptions } from '@claw/commands';

module.exports = new class implements yargs.CommandModule {
    command = 'status';
    describe = 'Reports the worker daemon status';

    builder(args: yargs.Argv) {
        commonOptions(args, 'verbose', 'config', 'pidfile', 'logfile', 'workerid');
        return args;
    }

    async handler(argv: yargs.Arguments) {
        app.build({ argv });

        try {
            await app.startup();

            const { id, pidfile, logfile } = app.config;

            if (id == null) {
                app.fail(
                    'workerid is not defined, please set --id [workerid] or configure your cla-worker.yml file'
                );
            }

            app.info(
                `checking status for workerid=${id} and pidfile=${pidfile}`
            );
            app.info(`logfile=${logfile}`);

            let pid: number;

            try {
                pid = app.getPid(pidfile);
                app.info(`workerid=${id} is assigned pid=${pid}...`);
            } catch (err) {
                app.error(`error getting pid from pidfile=${pidfile}: ${err}`);
            }

            try {
                process.kill(pid, 0);
                app.milestone(`worker is running with pid=${pid}`);
            } catch (err) {
                app.error(
                    `worker should be running with pid=${pid}, but was not up: ${err}`
                );
            }
        } catch (err) {
            app.debug(err);
            app.fail('command "status": %s', err);
        }
    }
}();
