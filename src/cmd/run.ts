import app from '@claw/app';
import * as yargs from 'yargs';
import { commonOptions, CmdArgs } from '@claw/commands';
import { isForked } from '@claw/common';
import { runner } from '@claw/runner';

class CmdRun implements yargs.CommandModule {
    command = 'run';
    describe = 'Run the worker online';

    builder(args: yargs.Argv) {
        commonOptions(
            args,
            'verbose',
            'token',
            'url',
            'config',
            'workerid',
            'logfile',
            'pidfile',
            'tags',
            'envs',
            'daemon'
        );
        return args;
    }

    async handler(argv: CmdArgs) {
        app.build({ argv });

        await app.startup();
        app.debug('cla-worker loaded config: ', app.config.data);

        const { id, daemon } = app.config.data;

        if (id == null) {
            app.fail(
                'workerid is not defined, please set --id [workerid] or configure your cla-worker.yml file'
            );
        }

        if (daemon && !isForked()) {
            app.info('spawning cla-worker in the background...');
            app.spawnDaemon();
        } else {
            if (isForked()) {
                app.daemonize();
            }
            runner();
        }
    }
}

module.exports = new CmdRun();
